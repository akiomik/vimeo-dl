// Copyright 2020 Akiomi Kamakura
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vimeo

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/akiomik/vimeo-dl/config"
)

const (
	maxManifestBytes   = 64 << 20
	maxSegmentAttempts = 3
)

// RequestError keeps the original error available to callers without putting
// a signed media URL in the message shown by the CLI.
type RequestError struct {
	Err error
}

func (e *RequestError) Error() string {
	if errors.Is(e.Err, context.Canceled) {
		return "HTTP request canceled"
	}
	if errors.Is(e.Err, context.DeadlineExceeded) {
		return "HTTP request timed out"
	}
	var netErr net.Error
	if errors.As(e.Err, &netErr) && netErr.Timeout() {
		return "HTTP request timed out"
	}
	return "HTTP request failed (URL omitted because it may contain access tokens)"
}

func (e *RequestError) Unwrap() error { return e.Err }

type Client struct {
	Client    *http.Client
	UserAgent string
}

func NewClient() *Client {
	client := Client{}
	client.Client = http.DefaultClient
	client.UserAgent = "vimeo-dl/" + config.Version

	return &client
}

func (c *Client) get(resourceURL *url.URL) (*http.Response, error) {
	if resourceURL == nil || (resourceURL.Scheme != "http" && resourceURL.Scheme != "https") || resourceURL.Host == "" {
		return nil, errors.New("input must be an absolute HTTP or HTTPS URL")
	}

	req, err := http.NewRequest("GET", resourceURL.String(), nil)
	if err != nil {
		return nil, errors.New("invalid HTTP URL")
	}
	req.Header.Set("User-Agent", c.UserAgent)

	res, err := c.Client.Do(req)
	if err != nil {
		return nil, &RequestError{Err: err}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		res.Body.Close()
		return nil, fmt.Errorf("HTTP %d %s", res.StatusCode, http.StatusText(res.StatusCode))
	}

	return res, nil
}

func (c *Client) GetMasterJson(resourceURL *url.URL) (*MasterJson, error) {
	res, err := c.get(resourceURL)
	if err != nil {
		return nil, fmt.Errorf("manifest request failed: %w", err)
	}
	defer res.Body.Close()

	if res.ContentLength > maxManifestBytes {
		return nil, errors.New("manifest exceeds size limit")
	}
	jsonBlob, err := io.ReadAll(io.LimitReader(res.Body, maxManifestBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading manifest failed: %w", err)
	}
	if len(jsonBlob) > maxManifestBytes {
		return nil, errors.New("manifest exceeds size limit")
	}
	if isHTML(res.Header.Get("Content-Type")) || looksLikeHTML(jsonBlob) {
		return nil, errors.New("manifest response is HTML, not JSON; check the input URL and access permissions")
	}

	masterJson := new(MasterJson)
	err = json.Unmarshal(jsonBlob, masterJson)
	if err != nil {
		return nil, fmt.Errorf("manifest response is not valid JSON: %w", err)
	}
	if masterJson.ClipId == "" || len(masterJson.Video) == 0 {
		return nil, errors.New("manifest is missing clip_id or video tracks; input must be a Vimeo JSON manifest URL")
	}
	if res.Request != nil && res.Request.URL != nil {
		masterJson.sourceURL = res.Request.URL
	}

	return masterJson, nil
}

func (c *Client) Download(resourceURL *url.URL, output io.Writer) error {
	return c.DownloadSized(resourceURL, output, 0)
}

func (c *Client) DownloadSized(resourceURL *url.URL, output io.Writer, expectedSize int64) error {
	if expectedSize < 0 || expectedSize == int64(1<<63-1) {
		return errors.New("segment size is invalid or too large")
	}
	if expectedSize == 0 {
		_, err := c.downloadSegmentOnce(resourceURL, output, 0)
		return err
	}

	// Keep each attempt out of the output until its length has been verified.
	// This allows retrying a truncated response without duplicating its prefix.
	var staged io.Writer
	var reset func() error
	var publish func() error
	if expectedSize <= 8<<20 {
		var segmentBuffer bytes.Buffer
		segmentBuffer.Grow(int(expectedSize))
		staged = &segmentBuffer
		reset = func() error {
			segmentBuffer.Reset()
			return nil
		}
		publish = func() error {
			_, err := io.Copy(output, &segmentBuffer)
			return err
		}
	} else {
		temporaryDirectory := ""
		if file, ok := output.(*os.File); ok {
			temporaryDirectory = filepath.Dir(file.Name())
		}
		segmentFile, err := os.CreateTemp(temporaryDirectory, ".vimeo-dl-segment-*")
		if err != nil && temporaryDirectory != "" {
			// The destination may be an already-open file in a directory where
			// creating siblings is forbidden (for example, os.Stdout in /dev).
			segmentFile, err = os.CreateTemp("", ".vimeo-dl-segment-*")
		}
		if err != nil {
			return fmt.Errorf("creating segment staging file failed: %w", err)
		}
		defer os.Remove(segmentFile.Name())
		defer segmentFile.Close()
		staged = segmentFile
		reset = func() error {
			if err := segmentFile.Truncate(0); err != nil {
				return err
			}
			_, err := segmentFile.Seek(0, io.SeekStart)
			return err
		}
		publish = func() error {
			if _, err := segmentFile.Seek(0, io.SeekStart); err != nil {
				return err
			}
			_, err := io.Copy(output, segmentFile)
			return err
		}
	}

	for attempt := 1; attempt <= maxSegmentAttempts; attempt++ {
		if err := reset(); err != nil {
			return fmt.Errorf("resetting segment staging file failed: %w", err)
		}
		retryable, err := c.downloadSegmentOnce(resourceURL, staged, expectedSize)
		if err == nil {
			if err := publish(); err != nil {
				return fmt.Errorf("writing completed segment failed: %w", err)
			}
			return nil
		}
		if !retryable || attempt == maxSegmentAttempts {
			if attempt > 1 {
				return fmt.Errorf("segment failed after %d attempts: %w", attempt, err)
			}
			return err
		}
		time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
	}
	return errors.New("segment retry limit reached")
}

type writeErrorRecorder struct {
	output io.Writer
	err    error
}

func (w *writeErrorRecorder) Write(data []byte) (int, error) {
	n, err := w.output.Write(data)
	if err != nil {
		w.err = err
	} else if n < len(data) {
		w.err = io.ErrShortWrite
	}
	return n, err
}

// downloadSegmentOnce reports whether a failed attempt can be retried safely.
func (c *Client) downloadSegmentOnce(resourceURL *url.URL, output io.Writer, expectedSize int64) (bool, error) {
	res, err := c.get(resourceURL)
	if err != nil {
		var requestErr *RequestError
		return errors.As(err, &requestErr) && !errors.Is(err, context.Canceled), fmt.Errorf("segment request failed: %w", err)
	}
	defer res.Body.Close()
	if isHTML(res.Header.Get("Content-Type")) {
		return false, errors.New("segment response is HTML, not media")
	}
	reader := bufio.NewReader(res.Body)
	firstBytes, _ := reader.Peek(64)
	if looksLikeHTML(firstBytes) {
		return false, errors.New("segment response is HTML, not media")
	}
	var body io.Reader = reader
	if expectedSize > 0 {
		body = io.LimitReader(reader, expectedSize+1)
	}

	recorder := &writeErrorRecorder{output: output}
	written, err := io.Copy(recorder, body)
	if err != nil {
		return recorder.err == nil && !errors.Is(err, context.Canceled), fmt.Errorf("segment download failed: %w", err)
	}
	if expectedSize > 0 && written != expectedSize {
		return written < expectedSize, fmt.Errorf("segment size mismatch: expected %d bytes, received %d", expectedSize, written)
	}

	return false, nil
}

func isHTML(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && (mediaType == "text/html" || mediaType == "application/xhtml+xml")
}

func looksLikeHTML(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	return bytes.HasPrefix(bytes.ToLower(trimmed), []byte("<!doctype html")) ||
		strings.HasPrefix(strings.ToLower(string(trimmed)), "<html")
}

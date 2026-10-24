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
	"strings"

	"github.com/akiomik/vimeo-dl/config"
)

const maxManifestBytes = 64 << 20

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
	res, err := c.get(resourceURL)
	if err != nil {
		return fmt.Errorf("segment request failed: %w", err)
	}
	defer res.Body.Close()
	if isHTML(res.Header.Get("Content-Type")) {
		return errors.New("segment response is HTML, not media")
	}
	reader := bufio.NewReader(res.Body)
	firstBytes, _ := reader.Peek(64)
	if looksLikeHTML(firstBytes) {
		return errors.New("segment response is HTML, not media")
	}
	var body io.Reader = reader
	if expectedSize > 0 {
		body = io.LimitReader(reader, expectedSize+1)
	}

	written, err := io.Copy(output, body)
	if err != nil {
		return fmt.Errorf("segment download failed: %w", err)
	}
	if expectedSize > 0 && written != expectedSize {
		return fmt.Errorf("segment size mismatch: expected %d bytes, received %d", expectedSize, written)
	}

	return nil
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

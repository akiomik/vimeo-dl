package vimeo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDownloadSizedRetriesTruncatedSegmentWithoutDuplicateBytes(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "4")
		if requests.Add(1) == 1 {
			w.Write([]byte("AB"))
			return
		}
		w.Write([]byte("ABCD"))
	}))
	defer server.Close()

	resourceURL, _ := url.Parse(server.URL + "/segment.m4s")
	output := bytes.NewBufferString("prefix:")
	err := NewClient().DownloadSized(resourceURL, output, 4)
	if err != nil {
		t.Fatalf("download failed after transient truncation: %v", err)
	}
	if got := output.String(); got != "prefix:ABCD" {
		t.Fatalf("output = %q, want one complete segment", got)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d, want one retry", got)
	}
}

type failOnceTransport struct {
	next     http.RoundTripper
	requests atomic.Int32
}

func (transport *failOnceTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if transport.requests.Add(1) == 1 {
		return nil, errors.New("stream error: PROTOCOL_ERROR")
	}
	return transport.next.RoundTrip(request)
}

func TestDownloadSizedRetriesRequestStreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ABCD"))
	}))
	defer server.Close()
	transport := &failOnceTransport{next: http.DefaultTransport}
	client := NewClient()
	client.Client = &http.Client{Transport: transport}
	resourceURL, _ := url.Parse(server.URL + "/segment.m4s")
	var output bytes.Buffer
	if err := client.DownloadSized(resourceURL, &output, 4); err != nil {
		t.Fatal(err)
	}
	if output.String() != "ABCD" || transport.requests.Load() != 2 {
		t.Fatalf("output = %q, requests = %d; want one complete segment after two requests", output.String(), transport.requests.Load())
	}
}

func TestDownloadSizedExhaustsRetriesWithoutPublishingPartialSegment(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Length", "4")
		w.Write([]byte("AB"))
	}))
	defer server.Close()
	resourceURL, _ := url.Parse(server.URL + "/segment.m4s")
	output := bytes.NewBufferString("prefix:")
	err := NewClient().DownloadSized(resourceURL, output, 4)
	if err == nil || !strings.Contains(err.Error(), "after 3 attempts") {
		t.Fatalf("error = %v, want bounded retry failure", err)
	}
	if got := requests.Load(); got != 3 {
		t.Fatalf("requests = %d, want 3 attempts", got)
	}
	if got := output.String(); got != "prefix:" {
		t.Fatalf("output = %q, want no partial segment", got)
	}
}

func TestDownloadSizedStagesLargeSegmentInOutputDirectory(t *testing.T) {
	const size = (8 << 20) + 1
	data := bytes.Repeat([]byte("X"), size)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	}))
	defer server.Close()
	directory := t.TempDir()
	output, err := os.Create(filepath.Join(directory, "output.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	resourceURL, _ := url.Parse(server.URL + "/segment.m4s")
	if err := NewClient().DownloadSized(resourceURL, output, size); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 || entries[0].Name() != "output.mp4" {
		t.Fatalf("staging cleanup failed: entries = %v, error = %v", entries, err)
	}
	if info, err := output.Stat(); err != nil || info.Size() != size {
		t.Fatalf("output size = %v, error = %v; want %d", info, err, size)
	}
}

func TestDownloadSizedStagesLargeSegmentWhenOutputDirectoryIsNotWritable(t *testing.T) {
	output, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Skipf("cannot open null output: %v", err)
	}
	defer output.Close()
	directory := filepath.Dir(output.Name())
	probe, err := os.CreateTemp(directory, ".vimeo-dl-probe-*")
	if err == nil {
		probe.Close()
		os.Remove(probe.Name())
		t.Skip("null output directory permits creating sibling files")
	}

	const size = (8 << 20) + 1
	data := bytes.Repeat([]byte("X"), size)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	}))
	defer server.Close()
	resourceURL, _ := url.Parse(server.URL + "/segment.m4s")
	if err := NewClient().DownloadSized(resourceURL, output, size); err != nil {
		t.Fatalf("large segment to existing writable file failed: %v", err)
	}
}

type canceledSegmentBody struct{ sent bool }

func (body *canceledSegmentBody) Read(data []byte) (int, error) {
	if !body.sent {
		body.sent = true
		return copy(data, "AB"), nil
	}
	return 0, context.Canceled
}

type canceledBodyTransport struct{ requests atomic.Int32 }

func (transport *canceledBodyTransport) RoundTrip(*http.Request) (*http.Response, error) {
	transport.requests.Add(1)
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(&canceledSegmentBody{})}, nil
}

func TestDownloadSizedDoesNotRetryCanceledBody(t *testing.T) {
	transport := &canceledBodyTransport{}
	client := NewClient()
	client.Client = &http.Client{Transport: transport}
	resourceURL, _ := url.Parse("https://media.example.invalid/segment.m4s")
	var output bytes.Buffer
	err := client.DownloadSized(resourceURL, &output, 4)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
	if got := transport.requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want no retry after cancellation", got)
	}
	if output.Len() != 0 {
		t.Fatalf("canceled response published %d bytes", output.Len())
	}
}

func TestDownloadSizedDoesNotRetryAccessDenial(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	resourceURL, _ := url.Parse(server.URL + "/segment.m4s")
	var output bytes.Buffer
	err := NewClient().DownloadSized(resourceURL, &output, 4)
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("error = %v, want HTTP 403", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want no retry", got)
	}
	if output.Len() != 0 {
		t.Fatalf("access denial wrote %d bytes", output.Len())
	}
}

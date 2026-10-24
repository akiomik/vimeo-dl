package vimeo

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyntheticManifestPreflight(t *testing.T) {
	cases := []struct {
		filename string
		want     string
	}{
		{"master-ranged.json", ""},
		{"playlist-segments.json", ""},
		{"repeated-parcel.json", "same URL"},
		{"master-unencoded-init.json", "base64_init=1"},
	}
	manifestURL, _ := url.Parse("https://media.example.invalid/path/master.json")
	for _, tc := range cases {
		t.Run(tc.filename, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", tc.filename))
			if err != nil {
				t.Fatal(err)
			}
			var manifest MasterJson
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
			err = manifest.ValidateForDownload(manifestURL, "synthetic-video", "synthetic-audio", true)
			if tc.want == "" && err != nil {
				t.Fatalf("unexpected preflight error: %v", err)
			}
			if tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("preflight error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestManifestHTTPResponses(t *testing.T) {
	htmlResponse, err := os.ReadFile(filepath.Join("testdata", "html-response.html"))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/html":
			w.Header().Set("Content-Type", "text/html")
			w.Write(htmlResponse)
		case "/denied":
			w.WriteHeader(http.StatusForbidden)
		case "/null":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, "null")
		case "/redirect":
			http.Redirect(w, r, "/cdn/master.json", http.StatusFound)
		case "/cdn/master.json":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"clip_id":"synthetic","base_url":"video/","video":[{"id":"v","init_segment":"AAECAw==","segments":[{"url":"part.m4s","size":4}]}]}`)
		}
	}))
	defer server.Close()
	client := NewClient()
	for _, tc := range []struct{ path, want string }{
		{"/html", "HTML"},
		{"/denied", "HTTP 403"},
		{"/null", "missing clip_id"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			u, _ := url.Parse(server.URL + tc.path)
			_, err := client.GetMasterJson(u)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	entry, _ := url.Parse(server.URL + "/redirect")
	manifest, err := client.GetMasterJson(entry)
	if err != nil {
		t.Fatal(err)
	}
	urls, err := manifest.VideoSegmentUrls(entry, "v")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := urls[0].String(), server.URL+"/cdn/video/part.m4s"; got != want {
		t.Fatalf("resolved URL = %q, want %q", got, want)
	}
}

func TestSegmentHTTPResponsesAndSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			io.WriteString(w, "ABCD")
		case "/html":
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, "<html>synthetic denial</html>")
		case "/wrong":
			io.WriteString(w, "ABCDEFGHIJ")
		case "/denied":
			w.WriteHeader(http.StatusForbidden)
		}
	}))
	defer server.Close()
	client := NewClient()
	for _, tc := range []struct {
		path, want string
	}{
		{"/ok", ""},
		{"/html", "HTML"},
		{"/wrong", "size mismatch"},
		{"/denied", "HTTP 403"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			u, _ := url.Parse(server.URL + tc.path)
			var output bytes.Buffer
			err := client.DownloadSized(u, &output, 4)
			if tc.want == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			if output.Len() > 5 {
				t.Fatalf("wrote %d bytes for a four-byte segment", output.Len())
			}
		})
	}
}

type failingRoundTripper struct{}

func (failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, &url.Error{Op: "Get", URL: "https://media.example.invalid/secret-path-token", Err: errors.New("secret-path-token")}
}

func TestRequestErrorDoesNotPrintSignedURL(t *testing.T) {
	client := NewClient()
	client.Client = &http.Client{Transport: failingRoundTripper{}}
	u, _ := url.Parse("https://media.example.invalid/secret-path-token")
	_, err := client.GetMasterJson(u)
	if err == nil || strings.Contains(err.Error(), "secret-path-token") {
		t.Fatalf("error exposed signed URL: %v", err)
	}
}

func TestManifestURLParseErrorDoesNotPrintSignedToken(t *testing.T) {
	manifest := MasterJson{
		BaseUrl: "%secret-token",
		Video:   []Video{{Id: "synthetic-video", InitSegment: "AAECAw==", Segments: []Segment{{Url: "part.m4s"}}}},
	}
	manifestURL, _ := url.Parse("https://media.example.invalid/master.json")
	err := manifest.ValidateForDownload(manifestURL, "synthetic-video", "", false)
	if err == nil || strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("error exposed a manifest value: %v", err)
	}
}

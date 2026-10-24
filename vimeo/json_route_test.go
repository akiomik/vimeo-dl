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

func TestSegmentURLResolutionBoundsExpansion(t *testing.T) {
	base, err := url.Parse("https://media.example.invalid/" + strings.Repeat("a", 48) + "/")
	if err != nil {
		t.Fatal(err)
	}
	segments := []Segment{
		{Url: "part.mp4?range=0-1"},
		{Url: "part.mp4?range=2-3"},
	}
	limits := segmentURLLimits{maxSegments: 2, maxURLBytes: 256, maxResolvedBytes: 128}
	if _, err := resolveSegmentURLs(base, segments, "video", limits); err == nil || !strings.Contains(err.Error(), "length limit") {
		t.Fatalf("expanded URLs were accepted: %v", err)
	}
	limits.maxResolvedBytes = 256
	urls, err := resolveSegmentURLs(base, segments, "video", limits)
	if err != nil || len(urls) != 2 || urls[0].String() == urls[1].String() {
		t.Fatalf("distinct range URLs were rejected: urls=%v, error=%v", urls, err)
	}
	limits.maxURLBytes = 8
	if _, err := resolveSegmentURLs(base, segments, "video", limits); err == nil || !strings.Contains(err.Error(), "length limit") {
		t.Fatalf("oversized raw segment URL was accepted: %v", err)
	}
}

func TestSegmentURLResolutionRejectsExcessiveCountBeforeAllocation(t *testing.T) {
	manifestURL, _ := url.Parse("https://media.example.invalid/master.json")
	segments := make([]Segment, maxSegmentsPerTrack+1)
	manifest := MasterJson{
		Video: []Video{{Id: "v", Segments: segments}},
		Audio: []Audio{{Id: "a", Segments: segments}},
	}
	if _, err := manifest.VideoSegmentUrls(manifestURL, "v"); err == nil || !strings.Contains(err.Error(), "too many segments") {
		t.Fatalf("oversized video track was accepted: %v", err)
	}
	if _, err := manifest.AudioSegmentUrls(manifestURL, "a"); err == nil || !strings.Contains(err.Error(), "too many segments") {
		t.Fatalf("oversized audio track was accepted: %v", err)
	}
}

func TestSegmentDownloadSizeAcceptsBothRangeSizeConventions(t *testing.T) {
	resourceURL, _ := url.Parse("https://media.example.invalid/segment.mp4?range=10-19")
	for _, size := range []int64{9, 10} {
		actual, err := segmentDownloadSize(Segment{Size: size}, resourceURL)
		if err != nil || actual != 10 {
			t.Errorf("size %d: download size = %d, error = %v; want 10", size, actual, err)
		}
	}
}

func TestManifestPreflightRejectsUnboundedOrInconsistentSegments(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "master-ranged.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifestURL, _ := url.Parse("https://media.example.invalid/manifest/master.json")
	for _, tc := range []struct {
		name   string
		change func(*MasterJson)
		want   string
	}{
		{"missing size", func(m *MasterJson) { m.Video[0].Segments[0].Size = 0 }, "size is missing"},
		{"range mismatch", func(m *MasterJson) { m.Audio[0].Segments[0].Size = 1 }, "size does not match byte range"},
		{"malformed range", func(m *MasterJson) { m.Audio[0].Segments[0].Url = "sample.mp4?range=0-" }, "byte range must be start-end"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var manifest MasterJson
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
			tc.change(&manifest)
			err := manifest.ValidateForDownload(manifestURL, "synthetic-video", "synthetic-audio", true)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
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

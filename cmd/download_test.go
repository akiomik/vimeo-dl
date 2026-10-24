package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "vimeo", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertNoOutputOrTemp(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("failed download left output files: %v", entries)
	}
}

func TestRunDownloadRecoversAfterAudioFailure(t *testing.T) {
	manifest := fixture(t, "master-ranged.json")
	var failAudio atomic.Bool
	failAudio.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest/master.json":
			w.Header().Set("Content-Type", "application/json")
			w.Write(manifest)
		case "/parcel/video/segment-1.m4s":
			w.Write([]byte("V001"))
		case "/parcel/video/segment-2.m4s":
			w.Write([]byte("V002"))
		case "/parcel/audio/sample.mp4":
			if failAudio.Load() {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			if r.URL.Query().Get("range") == "0-3" {
				w.Write([]byte("A001"))
			} else {
				w.Write([]byte("A002"))
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	directory := t.TempDir()
	base := filepath.Join(directory, "result")
	options := downloadOptions{Input: server.URL + "/manifest/master.json", OutputFilename: base}
	if err := runDownload(options); err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("first download error = %v, want HTTP 403", err)
	}
	assertNoOutputOrTemp(t, directory)

	failAudio.Store(false)
	if err := runDownload(options); err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	video, err := os.ReadFile(base + "-video.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if want := append([]byte{0, 1, 2, 3}, []byte("V001V002")...); !bytes.Equal(video, want) {
		t.Fatalf("video bytes = %v, want %v", video, want)
	}
	audio, err := os.ReadFile(base + "-audio.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if want := append([]byte{4, 5, 6, 7}, []byte("A001A002")...); !bytes.Equal(audio, want) {
		t.Fatalf("audio bytes = %v, want %v", audio, want)
	}
}

func TestRunDownloadRejectsRepeatedParcelBeforeMediaRequests(t *testing.T) {
	manifest := fixture(t, "repeated-parcel.json")
	var mediaRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/manifest/master.json" {
			w.Write(manifest)
			return
		}
		mediaRequests.Add(1)
	}))
	defer server.Close()
	directory := t.TempDir()
	err := runDownload(downloadOptions{Input: server.URL + "/manifest/master.json", OutputFilename: filepath.Join(directory, "result")})
	if err == nil || !strings.Contains(err.Error(), "same URL") {
		t.Fatalf("error = %v, want duplicate URL diagnosis", err)
	}
	if got := mediaRequests.Load(); got != 0 {
		t.Fatalf("made %d media requests before preflight completed", got)
	}
	assertNoOutputOrTemp(t, directory)
}

func TestRunDownloadPreservesExistingOutput(t *testing.T) {
	manifest := fixture(t, "master-ranged.json")
	var mediaRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/manifest/master.json" {
			w.Write(manifest)
			return
		}
		mediaRequests.Add(1)
	}))
	defer server.Close()
	base := filepath.Join(t.TempDir(), "result")
	if err := os.WriteFile(base+"-video.mp4", []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	err := runDownload(downloadOptions{Input: server.URL + "/manifest/master.json", OutputFilename: base})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %v, want existing output diagnosis", err)
	}
	got, err := os.ReadFile(base + "-video.mp4")
	if err != nil || string(got) != "existing" || mediaRequests.Load() != 0 {
		t.Fatalf("existing file changed or media was requested: bytes=%q, err=%v, requests=%d", got, err, mediaRequests.Load())
	}
}

func TestRunDownloadRejectsVideoPageAndEscapedAmpersand(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"https://vimeo.com/synthetic-page", "video page"},
		{`https://media.example.invalid/master.json?base64_init=1\u0026query_string_ranges=1`, `literal \u0026`},
	} {
		err := runDownload(downloadOptions{Input: tc.input})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("input error = %v, want %q", err, tc.want)
		}
	}
}

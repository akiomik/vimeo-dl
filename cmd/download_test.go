package cmd

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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

func TestRunDownloadContextCancelsAndCleansStaging(t *testing.T) {
	manifest := fixture(t, "master-ranged.json")
	segmentStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest/master.json":
			w.Write(manifest)
		case "/parcel/video/segment-1.m4s":
			w.Write([]byte("V0"))
			w.(http.Flusher).Flush()
			close(segmentStarted)
			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	directory := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runDownloadContext(ctx, downloadOptions{
			Input: server.URL + "/manifest/master.json", OutputFilename: filepath.Join(directory, "result"),
		})
	}()
	select {
	case <-segmentStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("segment download did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("download error = %v, want cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("download did not stop after cancellation")
	}
	assertNoOutputOrTemp(t, directory)
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

func TestRunDownloadRetriesOnlyInterruptedAudioSegment(t *testing.T) {
	manifest := fixture(t, "master-ranged.json")
	var videoRequests atomic.Int32
	var firstAudioRequests atomic.Int32
	var secondAudioRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest/master.json":
			w.Write(manifest)
		case "/parcel/video/segment-1.m4s":
			videoRequests.Add(1)
			w.Write([]byte("V001"))
		case "/parcel/video/segment-2.m4s":
			videoRequests.Add(1)
			w.Write([]byte("V002"))
		case "/parcel/audio/sample.mp4":
			w.Header().Set("Content-Length", "4")
			switch r.URL.Query().Get("range") {
			case "0-3":
				if firstAudioRequests.Add(1) == 1 {
					w.Write([]byte("A0"))
					return
				}
				w.Write([]byte("A001"))
			case "4-7":
				secondAudioRequests.Add(1)
				w.Write([]byte("A002"))
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	directory := t.TempDir()
	base := filepath.Join(directory, "result")
	if err := runDownload(downloadOptions{Input: server.URL + "/manifest/master.json", OutputFilename: base}); err != nil {
		t.Fatal(err)
	}
	video, err := os.ReadFile(base + "-video.mp4")
	if err != nil {
		t.Fatal(err)
	}
	audio, err := os.ReadFile(base + "-audio.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if want := append([]byte{0, 1, 2, 3}, []byte("V001V002")...); !bytes.Equal(video, want) {
		t.Fatalf("video bytes = %v, want %v", video, want)
	}
	if want := append([]byte{4, 5, 6, 7}, []byte("A001A002")...); !bytes.Equal(audio, want) {
		t.Fatalf("audio bytes = %v, want %v", audio, want)
	}
	if videoRequests.Load() != 2 || firstAudioRequests.Load() != 2 || secondAudioRequests.Load() != 1 {
		t.Fatalf("requests: video=%d, first audio=%d, second audio=%d", videoRequests.Load(), firstAudioRequests.Load(), secondAudioRequests.Load())
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 2 {
		t.Fatalf("unexpected staged files: entries=%v, error=%v", entries, err)
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

func unsupportedLink(string, string) error {
	return errors.New("hard links are unavailable")
}

func TestPublishFilesWithoutHardLinks(t *testing.T) {
	directory := t.TempDir()
	temporary := filepath.Join(directory, "staged.mp4")
	final := filepath.Join(directory, "published.mp4")
	if err := os.WriteFile(temporary, []byte("synthetic media"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := publishFilesWithLink(unsupportedLink, stagedFile{temporary, final}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(final)
	if err != nil || string(got) != "synthetic media" {
		t.Fatalf("published bytes = %q, err = %v", got, err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(final)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0077 != 0 {
			t.Fatalf("published file is accessible to other users: %v", info.Mode().Perm())
		}
	}
}

func TestPublishFilesRollbackPreservesExistingFile(t *testing.T) {
	for _, tc := range []struct {
		name string
		link func(string, string) error
	}{
		{"hard-link", os.Link},
		{"copy-fallback", unsupportedLink},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			firstTemp := filepath.Join(directory, "first-staged.mp4")
			secondTemp := filepath.Join(directory, "second-staged.mp4")
			firstFinal := filepath.Join(directory, "first.mp4")
			secondFinal := filepath.Join(directory, "second.mp4")
			for _, path := range []string{firstTemp, secondTemp} {
				if err := os.WriteFile(path, []byte("synthetic media"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(secondFinal, []byte("existing"), 0600); err != nil {
				t.Fatal(err)
			}
			err := publishFilesWithLink(tc.link,
				stagedFile{firstTemp, firstFinal}, stagedFile{secondTemp, secondFinal})
			if err == nil {
				t.Fatal("expected a no-overwrite error")
			}
			if _, err := os.Stat(firstFinal); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("first output remains after rollback: %v", err)
			}
			got, err := os.ReadFile(secondFinal)
			if err != nil || string(got) != "existing" {
				t.Fatalf("existing output changed: bytes=%q, err=%v", got, err)
			}
		})
	}
}

func TestPublishFilesRollsBackOnCancellation(t *testing.T) {
	directory := t.TempDir()
	firstTemp := filepath.Join(directory, "first-staged.mp4")
	secondTemp := filepath.Join(directory, "second-staged.mp4")
	firstFinal := filepath.Join(directory, "first.mp4")
	secondFinal := filepath.Join(directory, "second.mp4")
	for _, path := range []string{firstTemp, secondTemp} {
		if err := os.WriteFile(path, []byte("synthetic media"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	link := func(source, destination string) error {
		err := os.Link(source, destination)
		cancel()
		return err
	}
	err := publishFilesWithLinkContext(ctx, link,
		stagedFile{firstTemp, firstFinal}, stagedFile{secondTemp, secondFinal})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("publish error = %v, want cancellation", err)
	}
	for _, path := range []string{firstFinal, secondFinal} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("canceled publish left %s: %v", path, err)
		}
	}
}

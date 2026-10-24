//go:build darwin || linux

package cmd

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestFindFFmpegRequiresExplicitCurrentDirectoryPath(t *testing.T) {
	directory := t.TempDir()
	ffmpegPath := filepath.Join(directory, "ffmpeg")
	if err := os.WriteFile(ffmpegPath, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(directory)
	t.Setenv("PATH", ".")
	if _, err := findFFmpeg(""); err == nil || !strings.Contains(err.Error(), "--ffmpeg-path") {
		t.Fatalf("implicit current-directory lookup error = %v", err)
	}
	got, err := findFFmpeg("./ffmpeg")
	if err != nil {
		t.Fatalf("explicit current-directory executable failed: %v", err)
	}
	if got != ffmpegPath {
		t.Fatalf("ffmpeg path = %q, want %q", got, ffmpegPath)
	}
}

func TestPublishFilesCopyFailureCleansUp(t *testing.T) {
	directory := t.TempDir()
	// Reading a directory fails after the fallback has exclusively created
	// the destination, exercising its partial-file cleanup path.
	stagedDirectory := filepath.Join(directory, "staged")
	if err := os.Mkdir(stagedDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	final := filepath.Join(directory, "published.mp4")
	if err := publishFilesWithLink(unsupportedLink, stagedFile{stagedDirectory, final}); err == nil {
		t.Fatal("expected the fallback copy to fail")
	}
	if _, err := os.Stat(final); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial output remains: %v", err)
	}
}

func TestRunDownloadKeepsPrivatePermissionsWithRestrictiveUmask(t *testing.T) {
	manifest := fixture(t, "master-ranged.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest/master.json":
			w.Header().Set("Content-Type", "application/json")
			w.Write(manifest)
		case "/parcel/video/segment-1.m4s", "/parcel/video/segment-2.m4s", "/parcel/audio/sample.mp4":
			w.Write([]byte("DATA"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	directory := t.TempDir()
	ffmpegDirectory := t.TempDir()
	ffmpegPath := filepath.Join(ffmpegDirectory, "ffmpeg")
	ffmpegScript := "#!/bin/sh\nif [ \"$1\" = \"-version\" ]; then exit 0; fi\nfor argument do output=\"$argument\"; done\nprintf 'synthetic combined' > \"$output\"\n"
	if err := os.WriteFile(ffmpegPath, []byte(ffmpegScript), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", ffmpegDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))
	previousUmask := syscall.Umask(0077)
	defer syscall.Umask(previousUmask)

	for _, tc := range []struct {
		name       string
		combine    bool
		ffmpegPath string
		fileEnds   []string
	}{
		{"separate", false, "", []string{"-video.mp4", "-audio.mp4"}},
		{"combined", true, "", []string{".mp4"}},
		{"combined-explicit", true, ffmpegPath, []string{".mp4"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := filepath.Join(directory, tc.name)
			if err := runDownload(downloadOptions{
				Input: server.URL + "/manifest/master.json", OutputFilename: base,
				Combine: tc.combine, FFmpegPath: tc.ffmpegPath,
			}); err != nil {
				t.Fatal(err)
			}
			for _, suffix := range tc.fileEnds {
				info, err := os.Stat(base + suffix)
				if err != nil {
					t.Fatal(err)
				}
				if got := info.Mode().Perm(); got != 0600 {
					t.Fatalf("output permission = %04o, want 0600", got)
				}
			}
		})
	}
}

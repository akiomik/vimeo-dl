//go:build darwin || linux

package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCombineVideoAndAudioBoundsFFmpegErrorOutput(t *testing.T) {
	directory := t.TempDir()
	ffmpegPath := filepath.Join(directory, "ffmpeg")
	script := "#!/bin/sh\nprintf 'FIRST' >&2\nprintf '%05000d' 0 >&2\nprintf 'LAST' >&2\nexit 7\n"
	if err := os.WriteFile(ffmpegPath, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	err := combineVideoAndAudio(ffmpegPath, "video.mp4", "audio.mp4", filepath.Join(directory, "combined.mp4"))
	if err == nil || !strings.Contains(err.Error(), "LAST") || strings.Contains(err.Error(), "FIRST") {
		t.Fatalf("ffmpeg error did not retain only the output tail: %v", err)
	}
	if len(err.Error()) > maxFFmpegErrorBytes+100 {
		t.Fatalf("ffmpeg error contains %d bytes; want bounded output", len(err.Error()))
	}
}

func TestCombineVideoAndAudioStopsOnCancellation(t *testing.T) {
	directory := t.TempDir()
	ffmpegPath := filepath.Join(directory, "ffmpeg")
	marker := filepath.Join(directory, "started")
	if err := os.WriteFile(ffmpegPath, []byte("#!/bin/sh\nprintf started > \"$FFMPEG_MARKER\"\nexec sleep 30\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FFMPEG_MARKER", marker)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- combineVideoAndAudioContext(ctx, ffmpegPath, "video.mp4", "audio.mp4", filepath.Join(directory, "combined.mp4"))
	}()
	waitForFFmpegMarker(t, marker)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ffmpeg error = %v, want cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ffmpeg did not stop after cancellation")
	}
}

func TestFindFFmpegStopsVersionProbeOnCancellation(t *testing.T) {
	directory := t.TempDir()
	ffmpegPath := filepath.Join(directory, "ffmpeg")
	marker := filepath.Join(directory, "started")
	if err := os.WriteFile(ffmpegPath, []byte("#!/bin/sh\nprintf started > \"$FFMPEG_MARKER\"\nexec sleep 30\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FFMPEG_MARKER", marker)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := findFFmpegContext(ctx, ffmpegPath)
		done <- err
	}()
	waitForFFmpegMarker(t, marker)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("version probe error = %v, want cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("version probe did not stop after cancellation")
	}
}

func waitForFFmpegMarker(t *testing.T, marker string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(marker); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("synthetic FFmpeg did not start")
}

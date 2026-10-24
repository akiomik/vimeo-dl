//go:build darwin || linux

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

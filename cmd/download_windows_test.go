//go:build windows

package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindFFmpegWindowsCurrentDirectory(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "ffmpeg.go")
	if err := os.WriteFile(source, []byte("package main\nfunc main() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("go", "build", "-o", filepath.Join(directory, "ffmpeg.exe"), source).CombinedOutput(); err != nil {
		t.Fatalf("building synthetic executable failed: %v: %s", err, output)
	}
	t.Chdir(directory)
	if _, err := findFFmpeg(""); err == nil || !strings.Contains(err.Error(), "--ffmpeg-path") {
		t.Fatalf("implicit current-directory lookup error = %v", err)
	}
	got, err := findFFmpeg(`.\ffmpeg.exe`)
	if err != nil {
		t.Fatalf("explicit current-directory executable failed: %v", err)
	}
	if want := filepath.Join(directory, "ffmpeg.exe"); got != want {
		t.Fatalf("ffmpeg path = %q, want %q", got, want)
	}
}

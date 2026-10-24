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

package cmd

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/akiomik/vimeo-dl/vimeo"
)

type downloadOptions struct {
	Input          string
	UserAgent      string
	VideoID        string
	AudioID        string
	OutputFilename string
	Combine        bool
}

type stagedFile struct {
	temporary string
	final     string
}

type publishedFile struct {
	path string
	info os.FileInfo
}

func runDownload(options downloadOptions) error {
	if strings.Contains(options.Input, `\u0026`) {
		return errors.New("input URL contains a literal \\u0026; copy the decoded URL and quote it in the shell")
	}
	manifestURL, err := url.Parse(options.Input)
	if err != nil || manifestURL == nil || (manifestURL.Scheme != "http" && manifestURL.Scheme != "https") || manifestURL.Host == "" {
		return errors.New("input must be an absolute HTTP or HTTPS manifest URL")
	}
	if (manifestURL.Hostname() == "vimeo.com" || manifestURL.Hostname() == "www.vimeo.com") && !strings.HasSuffix(manifestURL.Path, ".json") {
		return errors.New("input is a Vimeo video page; use its master.json or playlist.json manifest URL")
	}

	client := vimeo.NewClient()
	if options.UserAgent != "" {
		client.UserAgent = options.UserAgent
	}
	manifest, err := client.GetMasterJson(manifestURL)
	if err != nil {
		return err
	}

	name := options.OutputFilename
	if name == "" {
		if manifest.ClipId == "." || manifest.ClipId == ".." || strings.ContainsAny(manifest.ClipId, `/\:`) {
			return errors.New("manifest clip_id cannot be used as an output name; specify --output-file-name")
		}
		name = manifest.ClipId
	}
	selectedVideoID := options.VideoID
	if selectedVideoID == "" {
		selectedVideoID = manifest.FindMaximumBitrateVideo().Id
	}
	includeAudio := len(manifest.Audio) > 0
	if options.Combine && !includeAudio {
		return errors.New("--combine was requested but the manifest has no audio track")
	}
	selectedAudioID := options.AudioID
	if includeAudio && selectedAudioID == "" {
		selectedAudioID = manifest.FindMaximumBitrateAudio().Id
	}
	if err := manifest.ValidateForDownload(manifestURL, selectedVideoID, selectedAudioID, includeAudio); err != nil {
		return err
	}

	videoFinal := name + "-video.mp4"
	audioFinal := name + "-audio.mp4"
	combinedFinal := name + ".mp4"
	if options.Combine {
		if err := ensureAbsent(combinedFinal); err != nil {
			return err
		}
	} else {
		if err := ensureAbsent(videoFinal); err != nil {
			return err
		}
		if includeAudio {
			if err := ensureAbsent(audioFinal); err != nil {
				return err
			}
		}
	}

	var ffmpegPath string
	if options.Combine {
		ffmpegPath, err = findFFmpeg()
		if err != nil {
			return err
		}
	}
	stagingDirectory, err := os.MkdirTemp(filepath.Dir(videoFinal), ".vimeo-dl-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stagingDirectory)

	fmt.Println("Downloading video")
	videoTemp, err := downloadToTemp(stagingDirectory, func(output io.Writer) error {
		return manifest.CreateVideoFile(output, manifestURL, selectedVideoID, client)
	})
	if err != nil {
		return err
	}
	if !includeAudio {
		return publishFiles(stagedFile{videoTemp, videoFinal})
	}
	fmt.Println("Downloading audio")
	audioTemp, err := downloadToTemp(stagingDirectory, func(output io.Writer) error {
		return manifest.CreateAudioFile(output, manifestURL, selectedAudioID, client)
	})
	if err != nil {
		return err
	}
	if !options.Combine {
		return publishFiles(stagedFile{videoTemp, videoFinal}, stagedFile{audioTemp, audioFinal})
	}
	combinedTemp, err := reserveTemp(stagingDirectory)
	if err != nil {
		return err
	}
	if err := combineVideoAndAudio(ffmpegPath, videoTemp, audioTemp, combinedTemp); err != nil {
		return err
	}
	return publishFiles(stagedFile{combinedTemp, combinedFinal})
}

func ensureAbsent(filename string) error {
	_, err := os.Lstat(filename)
	if err == nil {
		return fmt.Errorf("output already exists: %s; choose another --output-file-name", filename)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func downloadToTemp(directory string, write func(io.Writer) error) (string, error) {
	file, err := os.CreateTemp(directory, "*.mp4")
	if err != nil {
		return "", err
	}
	temporary := file.Name()
	success := false
	defer func() {
		if !success {
			os.Remove(temporary)
		}
	}()
	writeErr := write(file)
	closeErr := file.Close()
	if writeErr != nil {
		return "", writeErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	success = true
	return temporary, nil
}

func reserveTemp(directory string) (string, error) {
	file, err := os.CreateTemp(directory, "*.mp4")
	if err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

func publishFiles(files ...stagedFile) error {
	return publishFilesWithLink(os.Link, files...)
}

func publishFilesWithLink(link func(string, string) error, files ...stagedFile) error {
	created := make([]publishedFile, 0, len(files))
	for _, file := range files {
		published, err := publishFile(link, file)
		if err != nil {
			for _, previous := range created {
				removeIfUnchanged(previous)
			}
			return fmt.Errorf("publishing %s failed: %w", file.final, err)
		}
		created = append(created, published)
	}
	return nil
}

func publishFile(link func(string, string) error, file stagedFile) (publishedFile, error) {
	temporaryInfo, err := os.Stat(file.temporary)
	if err != nil {
		return publishedFile{}, err
	}
	if err := link(file.temporary, file.final); err == nil {
		return publishedFile{file.final, temporaryInfo}, nil
	}

	// Some filesystems do not support hard links. Copying to an exclusively
	// created destination keeps the no-overwrite guarantee on those volumes.
	source, err := os.Open(file.temporary)
	if err != nil {
		return publishedFile{}, err
	}
	defer source.Close()
	destination, err := os.OpenFile(file.final, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return publishedFile{}, err
	}
	info, statErr := destination.Stat()
	if statErr != nil {
		destination.Close()
		return publishedFile{}, statErr
	}
	published := publishedFile{file.final, info}
	_, copyErr := io.Copy(destination, source)
	closeErr := destination.Close()
	if copyErr != nil {
		removeIfUnchanged(published)
		return publishedFile{}, copyErr
	}
	if closeErr != nil {
		removeIfUnchanged(published)
		return publishedFile{}, closeErr
	}
	return published, nil
}

func removeIfUnchanged(file publishedFile) {
	current, err := os.Stat(file.path)
	if err == nil && os.SameFile(file.info, current) {
		os.Remove(file.path)
	}
}

func findFFmpeg() (string, error) {
	path, err := exec.LookPath("ffmpeg")
	if errors.Is(err, exec.ErrDot) {
		return "", errors.New("ffmpeg was found only through the current directory; install it in an absolute PATH directory")
	}
	if err != nil {
		return "", fmt.Errorf("ffmpeg is required for --combine: %w", err)
	}
	if err := exec.Command(path, "-version").Run(); err != nil {
		return "", fmt.Errorf("ffmpeg cannot run: %w", err)
	}
	return path, nil
}

func combineVideoAndAudio(ffmpegPath, videoFilename, audioFilename, outputFilename string) error {
	command := exec.Command(ffmpegPath, "-nostdin", "-y", "-loglevel", "error",
		"-i", videoFilename, "-i", audioFilename,
		"-map", "0:v:0", "-map", "1:a:0", "-c", "copy", outputFilename)
	output, err := command.CombinedOutput()
	if err != nil {
		if len(output) > 4096 {
			output = output[len(output)-4096:]
		}
		return fmt.Errorf("ffmpeg failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	info, err := os.Stat(outputFilename)
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return errors.New("ffmpeg produced an empty output")
	}
	if err := os.Chmod(outputFilename, 0600); err != nil {
		return err
	}
	return nil
}

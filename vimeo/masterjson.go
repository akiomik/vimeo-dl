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

package vimeo

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
)

type Segment struct {
	Url  string `json:"url"`
	Size int64  `json:"size"`
}

type Video struct {
	Id          string    `json:"id"`
	BaseUrl     string    `json:"base_url"`
	Bitrate     int       `json:"bitrate"`
	InitSegment string    `json:"init_segment"`
	Segments    []Segment `json:"segments"`
}

type Audio struct {
	Id          string    `json:"id"`
	BaseUrl     string    `json:"base_url"`
	Bitrate     int       `json:"bitrate"`
	InitSegment string    `json:"init_segment"`
	Segments    []Segment `json:"segments"`
}

type MasterJson struct {
	ClipId    string  `json:"clip_id"`
	BaseUrl   string  `json:"base_url"`
	Video     []Video `json:"video"`
	Audio     []Audio `json:"audio"`
	sourceURL *url.URL
}

func (v *Video) DecodedInitSegment() ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(v.InitSegment)
	if err != nil {
		return nil, err
	}

	return decoded, err
}

func (a *Audio) DecodedInitSegment() ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(a.InitSegment)
	if err != nil {
		return nil, err
	}

	return decoded, err
}

func (mj *MasterJson) FindVideo(id string) (*Video, error) {
	video := new(Video)
	for _, v := range mj.Video {
		if v.Id == id {
			video = &v
			break
		}
	}

	if len(video.Id) == 0 {
		return nil, errors.New("A video which has id '" + id + "' is not found in MasterJson")
	}

	return video, nil
}

func (mj *MasterJson) FindAudio(id string) (*Audio, error) {
	audio := new(Audio)
	for _, a := range mj.Audio {
		if a.Id == id {
			audio = &a
			break
		}
	}

	if len(audio.Id) == 0 {
		return nil, errors.New("A audio which has id '" + id + "' is not found in MasterJson")
	}

	return audio, nil
}

func (mj *MasterJson) FindMaximumBitrateVideo() *Video {
	if len(mj.Video) == 0 {
		return &Video{}
	}
	video := mj.Video[0]
	for _, v := range mj.Video[1:] {
		if v.Bitrate > video.Bitrate {
			video = v
		}
	}

	return &video
}

func (mj *MasterJson) FindMaximumBitrateAudio() *Audio {
	if len(mj.Audio) == 0 {
		return &Audio{}
	}
	audio := mj.Audio[0]
	for _, a := range mj.Audio[1:] {
		if a.Bitrate > audio.Bitrate {
			audio = a
		}
	}

	return &audio
}

func (mj *MasterJson) VideoSegmentUrls(masterJsonUrl *url.URL, id string) ([]*url.URL, error) {
	if mj.sourceURL != nil {
		masterJsonUrl = mj.sourceURL
	}
	if masterJsonUrl == nil {
		return nil, errors.New("manifest URL is missing")
	}
	baseUrl, err := url.Parse(mj.BaseUrl)
	if err != nil {
		return nil, errors.New("manifest base_url is invalid")
	}

	video, err := mj.FindVideo(id)
	if err != nil {
		return nil, err
	}

	videoBaseUrl, err := url.Parse(video.BaseUrl)
	if err != nil {
		return nil, errors.New("video base_url is invalid")
	}

	urls := make([]*url.URL, len(video.Segments))
	for i, s := range video.Segments {
		segmentUrl, err := url.Parse(s.Url)
		if err != nil {
			return nil, fmt.Errorf("video segment %d URL is invalid", i+1)
		}

		urls[i] = masterJsonUrl.ResolveReference(baseUrl).ResolveReference(videoBaseUrl).ResolveReference(segmentUrl)
	}

	return urls, validateDistinctSegmentURLs(urls)
}

func (mj *MasterJson) AudioSegmentUrls(masterJsonUrl *url.URL, id string) ([]*url.URL, error) {
	if mj.sourceURL != nil {
		masterJsonUrl = mj.sourceURL
	}
	if masterJsonUrl == nil {
		return nil, errors.New("manifest URL is missing")
	}
	baseUrl, err := url.Parse(mj.BaseUrl)
	if err != nil {
		return nil, errors.New("manifest base_url is invalid")
	}

	audio, err := mj.FindAudio(id)
	if err != nil {
		return nil, err
	}

	audioBaseUrl, err := url.Parse(audio.BaseUrl)
	if err != nil {
		return nil, errors.New("audio base_url is invalid")
	}

	urls := make([]*url.URL, len(audio.Segments))
	for i, s := range audio.Segments {
		segmentUrl, err := url.Parse(s.Url)
		if err != nil {
			return nil, fmt.Errorf("audio segment %d URL is invalid", i+1)
		}

		urls[i] = masterJsonUrl.ResolveReference(baseUrl).ResolveReference(audioBaseUrl).ResolveReference(segmentUrl)
	}

	return urls, validateDistinctSegmentURLs(urls)
}

func validateDistinctSegmentURLs(urls []*url.URL) error {
	seen := make(map[string]int, len(urls))
	for i, segmentURL := range urls {
		if (segmentURL.Scheme != "http" && segmentURL.Scheme != "https") || segmentURL.Host == "" {
			return fmt.Errorf("segment %d does not resolve to an HTTP or HTTPS URL", i+1)
		}
		key := segmentURL.String()
		if previous, ok := seen[key]; ok {
			return fmt.Errorf("segments %d and %d resolve to the same URL; obtain a manifest with query-string ranges enabled", previous+1, i+1)
		}
		seen[key] = i
	}
	return nil
}

// segmentDownloadSize accounts for Vimeo manifests that report the difference
// between range endpoints while the CDN serves both endpoints inclusively.
func segmentDownloadSize(segment Segment, resourceURL *url.URL) (int64, error) {
	if segment.Size < 0 {
		return 0, errors.New("negative size")
	}
	const maxInt64 = int64(1<<63 - 1)
	ranges, hasRange := resourceURL.Query()["range"]
	if !hasRange {
		if segment.Size == 0 {
			return 0, errors.New("size is missing and URL has no byte range")
		}
		if segment.Size == maxInt64 {
			return 0, errors.New("size is too large")
		}
		return segment.Size, nil
	}
	if len(ranges) != 1 {
		return 0, errors.New("byte range must have exactly one value")
	}
	startText, endText, ok := strings.Cut(ranges[0], "-")
	if !ok || startText == "" || endText == "" {
		return 0, errors.New("byte range must be start-end")
	}
	start, startErr := strconv.ParseInt(startText, 10, 64)
	end, endErr := strconv.ParseInt(endText, 10, 64)
	if startErr != nil || endErr != nil || start < 0 || end < start || end-start >= maxInt64-1 {
		return 0, errors.New("byte range is invalid or too large")
	}
	inclusiveSize := end - start + 1
	if segment.Size != inclusiveSize && segment.Size != inclusiveSize-1 {
		return 0, errors.New("size does not match byte range")
	}
	return inclusiveSize, nil
}

func segmentDownloadSizes(segments []Segment, urls []*url.URL, kind string) ([]int64, error) {
	sizes := make([]int64, len(segments))
	for i, segment := range segments {
		size, err := segmentDownloadSize(segment, urls[i])
		if err != nil {
			return nil, fmt.Errorf("%s segment %d: %w", kind, i+1, err)
		}
		sizes[i] = size
	}
	return sizes, nil
}

// ValidateForDownload checks the selected tracks before any output file is made.
func (mj *MasterJson) ValidateForDownload(masterJsonUrl *url.URL, videoID, audioID string, includeAudio bool) error {
	video, err := mj.FindVideo(videoID)
	if err != nil {
		return err
	}
	if len(video.Segments) == 0 {
		return errors.New("selected video has no segments")
	}
	videoInit, err := video.DecodedInitSegment()
	if err != nil {
		return fmt.Errorf("video init_segment is not base64; obtain a manifest with base64_init=1: %w", err)
	}
	if len(videoInit) == 0 {
		return errors.New("selected video has no init_segment")
	}
	videoURLs, err := mj.VideoSegmentUrls(masterJsonUrl, videoID)
	if err != nil {
		return fmt.Errorf("video segments are not downloadable: %w", err)
	}
	if _, err := segmentDownloadSizes(video.Segments, videoURLs, "video"); err != nil {
		return err
	}
	if !includeAudio {
		return nil
	}
	audio, err := mj.FindAudio(audioID)
	if err != nil {
		return err
	}
	if len(audio.Segments) == 0 {
		return errors.New("selected audio has no segments")
	}
	audioInit, err := audio.DecodedInitSegment()
	if err != nil {
		return fmt.Errorf("audio init_segment is not base64; obtain a manifest with base64_init=1: %w", err)
	}
	if len(audioInit) == 0 {
		return errors.New("selected audio has no init_segment")
	}
	audioURLs, err := mj.AudioSegmentUrls(masterJsonUrl, audioID)
	if err != nil {
		return fmt.Errorf("audio segments are not downloadable: %w", err)
	}
	if _, err := segmentDownloadSizes(audio.Segments, audioURLs, "audio"); err != nil {
		return err
	}
	return nil
}

func (mj *MasterJson) CreateVideoFile(output io.Writer, masterJsonUrl *url.URL, id string, client *Client) error {
	video, err := mj.FindVideo(id)
	if err != nil {
		return err
	}

	initSegment, err := video.DecodedInitSegment()
	if err != nil {
		return err
	}
	videoSegmentUrls, err := mj.VideoSegmentUrls(masterJsonUrl, id)
	if err != nil {
		return err
	}
	videoSizes, err := segmentDownloadSizes(video.Segments, videoSegmentUrls, "video")
	if err != nil {
		return err
	}

	if written, err := output.Write(initSegment); err != nil {
		return err
	} else if written != len(initSegment) {
		return io.ErrShortWrite
	}

	for i, videoSegmentUrl := range videoSegmentUrls {
		err = client.DownloadSized(videoSegmentUrl, output, videoSizes[i])
		if err != nil {
			return err
		}
	}

	return nil
}

func (mj *MasterJson) CreateAudioFile(output io.Writer, masterJsonUrl *url.URL, id string, client *Client) error {
	audio, err := mj.FindAudio(id)
	if err != nil {
		return err
	}

	initSegment, err := audio.DecodedInitSegment()
	if err != nil {
		return err
	}
	audioSegmentUrls, err := mj.AudioSegmentUrls(masterJsonUrl, id)
	if err != nil {
		return err
	}
	audioSizes, err := segmentDownloadSizes(audio.Segments, audioSegmentUrls, "audio")
	if err != nil {
		return err
	}

	if written, err := output.Write(initSegment); err != nil {
		return err
	} else if written != len(initSegment) {
		return io.ErrShortWrite
	}

	for i, audioSegmentUrl := range audioSegmentUrls {
		err = client.DownloadSized(audioSegmentUrl, output, audioSizes[i])
		if err != nil {
			return err
		}
	}

	return nil
}

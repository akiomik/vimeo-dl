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
	"context"
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

// A manifest can reuse one long base_url for every segment. These limits bound
// the memory used by resolved URLs separately from the manifest response size.
const (
	maxSegmentsPerTrack        = 100_000
	maxSegmentURLBytes         = 32 << 10
	maxResolvedSegmentURLBytes = 64 << 20
)

type segmentURLLimits struct {
	maxSegments      int
	maxURLBytes      int
	maxResolvedBytes int
}

var defaultSegmentURLLimits = segmentURLLimits{
	maxSegments:      maxSegmentsPerTrack,
	maxURLBytes:      maxSegmentURLBytes,
	maxResolvedBytes: maxResolvedSegmentURLBytes,
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
	baseUrl, err := mj.resolveBaseURL(masterJsonUrl)
	if err != nil {
		return nil, err
	}

	video, err := mj.FindVideo(id)
	if err != nil {
		return nil, err
	}

	videoBaseUrl, err := parseTrackBaseURL(video.BaseUrl, "video")
	if err != nil {
		return nil, err
	}
	return resolveSegmentURLs(baseUrl.ResolveReference(videoBaseUrl), video.Segments, "video", defaultSegmentURLLimits)
}

func (mj *MasterJson) AudioSegmentUrls(masterJsonUrl *url.URL, id string) ([]*url.URL, error) {
	baseUrl, err := mj.resolveBaseURL(masterJsonUrl)
	if err != nil {
		return nil, err
	}

	audio, err := mj.FindAudio(id)
	if err != nil {
		return nil, err
	}

	audioBaseUrl, err := parseTrackBaseURL(audio.BaseUrl, "audio")
	if err != nil {
		return nil, err
	}
	return resolveSegmentURLs(baseUrl.ResolveReference(audioBaseUrl), audio.Segments, "audio", defaultSegmentURLLimits)
}

func (mj *MasterJson) resolveBaseURL(manifestURL *url.URL) (*url.URL, error) {
	if mj.sourceURL != nil {
		manifestURL = mj.sourceURL
	}
	if manifestURL == nil {
		return nil, errors.New("manifest URL is missing")
	}
	if len(mj.BaseUrl) > maxSegmentURLBytes {
		return nil, errors.New("manifest base_url exceeds length limit")
	}
	baseURL, err := url.Parse(mj.BaseUrl)
	if err != nil {
		return nil, errors.New("manifest base_url is invalid")
	}
	return manifestURL.ResolveReference(baseURL), nil
}

func parseTrackBaseURL(raw, kind string) (*url.URL, error) {
	if len(raw) > maxSegmentURLBytes {
		return nil, fmt.Errorf("%s base_url exceeds length limit", kind)
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%s base_url is invalid", kind)
	}
	return parsed, nil
}

func resolveSegmentURLs(baseURL *url.URL, segments []Segment, kind string, limits segmentURLLimits) ([]*url.URL, error) {
	if len(segments) > limits.maxSegments {
		return nil, fmt.Errorf("%s has too many segments (limit %d)", kind, limits.maxSegments)
	}
	urls := make([]*url.URL, len(segments))
	seen := make(map[string]int, len(segments))
	resolvedBytes := 0
	for i, segment := range segments {
		if len(segment.Url) > limits.maxURLBytes {
			return nil, fmt.Errorf("%s segment %d URL exceeds length limit", kind, i+1)
		}
		segmentURL, err := url.Parse(segment.Url)
		if err != nil {
			return nil, fmt.Errorf("%s segment %d URL is invalid", kind, i+1)
		}
		resolved := baseURL.ResolveReference(segmentURL)
		if (resolved.Scheme != "http" && resolved.Scheme != "https") || resolved.Host == "" {
			return nil, fmt.Errorf("segment %d does not resolve to an HTTP or HTTPS URL", i+1)
		}
		key := resolved.String()
		if len(key) > limits.maxURLBytes || len(key) > limits.maxResolvedBytes-resolvedBytes {
			return nil, fmt.Errorf("%s segment URLs exceed length limit", kind)
		}
		if previous, ok := seen[key]; ok {
			return nil, fmt.Errorf("segments %d and %d resolve to the same URL; obtain a manifest with query-string ranges enabled", previous+1, i+1)
		}
		seen[key] = i
		resolvedBytes += len(key)
		urls[i] = resolved
	}
	return urls, nil
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
	return mj.CreateVideoFileContext(context.Background(), output, masterJsonUrl, id, client)
}

func (mj *MasterJson) CreateVideoFileContext(ctx context.Context, output io.Writer, masterJsonUrl *url.URL, id string, client *Client) error {
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
		err = client.DownloadSizedContext(ctx, videoSegmentUrl, output, videoSizes[i])
		if err != nil {
			return err
		}
	}

	return ctx.Err()
}

func (mj *MasterJson) CreateAudioFile(output io.Writer, masterJsonUrl *url.URL, id string, client *Client) error {
	return mj.CreateAudioFileContext(context.Background(), output, masterJsonUrl, id, client)
}

func (mj *MasterJson) CreateAudioFileContext(ctx context.Context, output io.Writer, masterJsonUrl *url.URL, id string, client *Client) error {
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
		err = client.DownloadSizedContext(ctx, audioSegmentUrl, output, audioSizes[i])
		if err != nil {
			return err
		}
	}

	return ctx.Err()
}

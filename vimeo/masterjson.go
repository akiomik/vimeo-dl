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
	"os"

	"golang.org/x/sync/errgroup"
)

type Segment struct {
	Url string `json:"url"`
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
	ClipId  string  `json:"clip_id"`
	BaseUrl string  `json:"base_url"`
	Video   []Video `json:"video"`
	Audio   []Audio `json:"audio"`
}

func ConsolidateTempFiles(output io.Writer, results []*os.File) error {
	for _, result := range results {
		file, err := os.Open(result.Name())

		if err != nil {
			return err
		}

		io.Copy(output, file)

		if err := file.Close(); err != nil {
			return err
		}

		if err := Cleanup(file); err != nil {
			return err
		}
	}

	return nil
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
	var video Video
	for _, v := range mj.Video {
		if v.Bitrate > video.Bitrate {
			video = v
		}
	}

	return &video
}

func (mj *MasterJson) FindMaximumBitrateAudio() *Audio {
	var audio Audio
	for _, a := range mj.Audio {
		if a.Bitrate > audio.Bitrate {
			audio = a
		}
	}

	return &audio
}

func (mj *MasterJson) VideoSegmentUrls(masterJsonUrl *url.URL, id string) ([]*url.URL, error) {
	baseUrl, err := url.Parse(mj.BaseUrl)
	if err != nil {
		return nil, err
	}

	video, err := mj.FindVideo(id)
	if err != nil {
		return nil, err
	}

	videoBaseUrl, err := url.Parse(video.BaseUrl)
	if err != nil {
		return nil, err
	}

	urls := make([]*url.URL, len(video.Segments))
	for i, s := range video.Segments {
		segmentUrl, err := url.Parse(s.Url)
		if err != nil {
			return nil, err
		}

		urls[i] = masterJsonUrl.ResolveReference(baseUrl).ResolveReference(videoBaseUrl).ResolveReference(segmentUrl)
	}

	return urls, nil
}

func (mj *MasterJson) AudioSegmentUrls(masterJsonUrl *url.URL, id string) ([]*url.URL, error) {
	baseUrl, err := url.Parse(mj.BaseUrl)
	if err != nil {
		return nil, err
	}

	audio, err := mj.FindAudio(id)
	if err != nil {
		return nil, err
	}

	audioBaseUrl, err := url.Parse(audio.BaseUrl)
	if err != nil {
		return nil, err
	}

	urls := make([]*url.URL, len(audio.Segments))
	for i, s := range audio.Segments {
		segmentUrl, err := url.Parse(s.Url)
		if err != nil {
			return nil, err
		}

		urls[i] = masterJsonUrl.ResolveReference(baseUrl).ResolveReference(audioBaseUrl).ResolveReference(segmentUrl)
	}

	return urls, nil
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
	output.Write(initSegment)

	videoSegmentUrls, err := mj.VideoSegmentUrls(masterJsonUrl, id)
	if err != nil {
		return err
	}

	concurrency := 50
	var g errgroup.Group
	g.SetLimit(concurrency)
	results := make([]*os.File, len(videoSegmentUrls))

	for i, videoSegmentUrl := range videoSegmentUrls {
		i, videoSegmentUrl := i, videoSegmentUrl // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			fmt.Println("Downloading " + videoSegmentUrl.String())
			file, err := client.Download(videoSegmentUrl)
			if err == nil {
				results[i] = file
			}

			return err
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	ConsolidateTempFiles(output, results)

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
	output.Write(initSegment)

	audioSegmentUrls, err := mj.AudioSegmentUrls(masterJsonUrl, id)
	if err != nil {
		return err
	}

	concurrency := 50
	var g errgroup.Group
	g.SetLimit(concurrency)
	results := make([]*os.File, len(audioSegmentUrls))

	// map
	for i, audioSegmentUrl := range audioSegmentUrls {
		i, audioSegmentUrl := i, audioSegmentUrl // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			fmt.Println("Downloading " + audioSegmentUrl.String())
			file, err := client.Download(audioSegmentUrl)
			if err == nil {
				results[i] = file
			}

			return err
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	ConsolidateTempFiles(output, results)

	return nil
}

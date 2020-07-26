package vimeo

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
)

type Segment struct {
	Url string `json:"url"`
}

type Video struct {
	Id          string    `json:"id"`
	InitSegment string    `json:"init_segment"`
	Segments    []Segment `json:"segments"`
}

type Audio struct {
	// TODO
}

type MasterJson struct {
	ClipId  string  `json:"clip_id"`
	BaseUrl string  `json:"base_url"`
	Video   []Video `json:"video"`
	Audio   []Audio `json:"audio"`
}

func (v *Video) DecodedInitSegment() ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(v.InitSegment)
	if err != nil {
		return nil, err
	}

	return decoded, err
}

func (mj *MasterJson) FindVideo(scale string) (*Video, error) {
	video := new(Video)
	for _, v := range mj.Video {
		if v.Id == scale {
			video = &v
			break
		}
	}

	if len(video.Id) == 0 {
		return nil, errors.New("A video which has scale '" + scale + "' is not found in MasterJson")
	}

	return video, nil
}

func (mj *MasterJson) VideoSegmentUrls(masterJsonUrl *url.URL, scale string) ([]*url.URL, error) {
	baseUrl, err := url.Parse(mj.BaseUrl)
	if err != nil {
		return nil, err
	}

	video, err := mj.FindVideo(scale)
	if err != nil {
		return nil, err
	}

	urls := make([]*url.URL, len(video.Segments))
	for i, s := range video.Segments {
		segmentUrl, err := url.Parse(s.Url)
		if err != nil {
			return nil, err
		}

		urls[i] = masterJsonUrl.ResolveReference(baseUrl).ResolveReference(segmentUrl)
	}

	return urls, nil
}

func (mj *MasterJson) CreateVideoFile(output io.Writer, masterJsonUrl *url.URL, scale string, client *Client) error {
	video, err := mj.FindVideo(scale)
	if err != nil {
		return err
	}

	initSegment, err := video.DecodedInitSegment()
	if err != nil {
		return err
	}
	output.Write(initSegment)

	videoSegmentUrls, err := mj.VideoSegmentUrls(masterJsonUrl, scale)
	if err != nil {
		return err
	}

	for _, videoSegmentUrl := range videoSegmentUrls {
		fmt.Println("Downloading " + videoSegmentUrl.String())
		err = client.Download(videoSegmentUrl, output)
		if err != nil {
			return err
		}
	}

	return nil
}

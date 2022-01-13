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
	"bytes"
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

func TestVideoDecodedInitSegment(t *testing.T) {
	masterJson := MasterJson{
		Video: []Video{
			Video{
				Id:          "1080p",
				InitSegment: "Zm9vYmFyYmF6cXV4Cg==",
			},
		},
	}
	expected := []byte("foobarbazqux\x0a") // LF for RFC2045

	video, _ := masterJson.FindVideo("1080p")
	actual, err := video.DecodedInitSegment()
	if err != nil {
		t.Errorf("DecodedInitSegment failed to decode: %v", err)
		return
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("DecodedInitSegment decoded init segment does not match.\nexpected: %v\nactual:   %v", expected, actual)
		return
	}
}

func TestAudioDecodedInitSegment(t *testing.T) {
	masterJson := MasterJson{
		Audio: []Audio{
			Audio{
				Id:          "1080p",
				InitSegment: "Zm9vYmFyYmF6cXV4Cg==",
			},
		},
	}
	expected := []byte("foobarbazqux\x0a") // LF for RFC2045

	audio, _ := masterJson.FindAudio("1080p")
	actual, err := audio.DecodedInitSegment()
	if err != nil {
		t.Errorf("DecodedInitSegment failed to decode: %v", err)
		return
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("DecodedInitSegment decoded init segment does not match.\nexpected: %v\nactual:   %v", expected, actual)
		return
	}
}

func TestFindVideo(t *testing.T) {
	masterJson := MasterJson{
		Video: []Video{
			Video{
				Id:       "240p",
				Segments: make([]Segment, 0),
			},
			Video{
				Id:       "360p",
				Segments: make([]Segment, 0),
			},
			Video{
				Id:       "720p",
				Segments: make([]Segment, 0),
			},
			Video{
				Id:       "1080p",
				Segments: make([]Segment, 0),
			},
		},
	}

	for _, id := range []string{"240p", "360p", "720p", "1080p"} {
		v, _ := masterJson.FindVideo(id)
		if v.Id != id {
			t.Errorf("FindVideo video id does not match.\nexpected: %v\nactual:   %v", id, v.Id)
			return
		}
	}

	_, err := masterJson.FindVideo("notfound")
	if err == nil {
		t.Errorf("FindVideo err must be nil when video is not found.")
		return
	}
}

func TestFindMaximumBitrateVideo(t *testing.T) {
	masterJson := MasterJson{
		Video: []Video{
			Video{
				Id:       "240p",
				Bitrate:  430000,
				Segments: make([]Segment, 0),
			},
			Video{
				Id:       "360p",
				Bitrate:  750000,
				Segments: make([]Segment, 0),
			},
			Video{
				Id:       "540p",
				Bitrate:  2098000,
				Segments: make([]Segment, 0),
			},
			Video{
				Id:       "720p",
				Bitrate:  3325000,
				Segments: make([]Segment, 0),
			},
			Video{
				Id:       "1080p",
				Bitrate:  6385000,
				Segments: make([]Segment, 0),
			},
		},
	}
	expected := &masterJson.Video[4]

	actual := masterJson.FindMaximumBitrateVideo()
	if !reflect.DeepEqual(*actual, *expected) {
		t.Errorf("FindMaximumBitrateVideo id does not match.\nexpected: %v\nactual:   %v", *expected, *actual)
		return
	}
}

func TestFindMaximumBitrateAudio(t *testing.T) {
	masterJson := MasterJson{
		Audio: []Audio{
			Audio{
				Id:       "foo",
				Bitrate:  255000,
				Segments: make([]Segment, 0),
			},
			Audio{
				Id:       "bar",
				Bitrate:  128000,
				Segments: make([]Segment, 0),
			},
			Audio{
				Id:       "buz",
				Bitrate:  64000,
				Segments: make([]Segment, 0),
			},
		},
	}
	expected := &masterJson.Audio[0]

	actual := masterJson.FindMaximumBitrateAudio()
	if !reflect.DeepEqual(*actual, *expected) {
		t.Errorf("FindMaximumBitrateVideo id does not match.\nexpected: %v\nactual:   %v", *expected, *actual)
		return
	}
}

func TestVideoSegmentUrls(t *testing.T) {
	masterJson := MasterJson{
		BaseUrl: "../../../parcel/archive/",
		Video: []Video{
			Video{
				Id: "1080p",
				Segments: []Segment{
					Segment{Url: "1080p.mp4?range=0-9"},
					Segment{Url: "1080p.mp4?range=10-19"},
					Segment{Url: "1080p.mp4?range=20-29"},
				},
			},
		},
	}

	url0, _ := url.Parse("https://example.com/foo/parcel/archive/1080p.mp4?range=0-9")
	url1, _ := url.Parse("https://example.com/foo/parcel/archive/1080p.mp4?range=10-19")
	url2, _ := url.Parse("https://example.com/foo/parcel/archive/1080p.mp4?range=20-29")
	expected := []*url.URL{url0, url1, url2}

	masterJsonUrl, _ := url.Parse("https://example.com/foo/bar/baz/qux/master.json")
	actual, err := masterJson.VideoSegmentUrls(masterJsonUrl, "1080p")
	if err != nil {
		t.Errorf("VideoSegmentUrls failed to parse urls: %v", err)
		return
	}

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("VideoSegmentUrls urls does not match.\nexpected: %v\nactual:   %v", expected, actual)
		return
	}
}

func TestVideoSegmentUrls2(t *testing.T) {
	masterJson := MasterJson{
		BaseUrl: "../",
		Video: []Video{
			Video{
				Id:      "qux",
				BaseUrl: "qux/chop/",
				Segments: []Segment{
					Segment{Url: "segment-1.m4s"},
					Segment{Url: "segment-2.m4s"},
					Segment{Url: "segment-3.m4s"},
				},
			},
		},
	}

	url0, _ := url.Parse("https://example.com/foo/bar/video/qux/chop/segment-1.m4s")
	url1, _ := url.Parse("https://example.com/foo/bar/video/qux/chop/segment-2.m4s")
	url2, _ := url.Parse("https://example.com/foo/bar/video/qux/chop/segment-3.m4s")
	expected := []*url.URL{url0, url1, url2}

	masterJsonUrl, _ := url.Parse("https://example.com/foo/bar/video/baz/master.json")
	actual, err := masterJson.VideoSegmentUrls(masterJsonUrl, "qux")
	if err != nil {
		t.Errorf("VideoSegmentUrls failed to parse urls: %v", err)
		return
	}

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("VideoSegmentUrls urls does not match.\nexpected: %v\nactual:   %v", expected, actual)
		return
	}
}

func TestAudioSegmentUrls(t *testing.T) {
	masterJson := MasterJson{
		BaseUrl: "../",
		Audio: []Audio{
			Audio{
				Id:      "qux",
				BaseUrl: "../audio/qux/chop/",
				Segments: []Segment{
					Segment{Url: "segment-1.m4s"},
					Segment{Url: "segment-2.m4s"},
					Segment{Url: "segment-3.m4s"},
				},
			},
		},
	}

	url0, _ := url.Parse("https://example.com/foo/bar/audio/qux/chop/segment-1.m4s")
	url1, _ := url.Parse("https://example.com/foo/bar/audio/qux/chop/segment-2.m4s")
	url2, _ := url.Parse("https://example.com/foo/bar/audio/qux/chop/segment-3.m4s")
	expected := []*url.URL{url0, url1, url2}

	masterJsonUrl, _ := url.Parse("https://example.com/foo/bar/video/baz/master.json")
	actual, err := masterJson.AudioSegmentUrls(masterJsonUrl, "qux")
	if err != nil {
		t.Errorf("AudioSegmentUrls failed to parse urls: %v", err)
		return
	}

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("AudioSegmentUrls urls does not match.\nexpected: %v\nactual:   %v", expected, actual)
		return
	}
}

func TestCreateVideoFile(t *testing.T) {
	masterJson := MasterJson{
		BaseUrl: "../../../parcel/archive/",
		Video: []Video{
			Video{
				Id:          "1080p",
				InitSegment: "Zm9vYmFyYmF6cXV4Cg==",
				Segments: []Segment{
					Segment{Url: "1080p.mp4?range=0-9"},
					Segment{Url: "1080p.mp4?range=10-19"},
					Segment{Url: "1080p.mp4?range=20-29"},
				},
			},
		},
	}
	url0, _ := url.Parse("https://example.com/foo/parcel/archive/1080p.mp4?range=0-9")
	url1, _ := url.Parse("https://example.com/foo/parcel/archive/1080p.mp4?range=10-19")
	url2, _ := url.Parse("https://example.com/foo/parcel/archive/1080p.mp4?range=20-29")
	body0 := []byte("0123456789")
	body1 := []byte("abcdefghij")
	body2 := []byte("ABCDEFGHIJ")
	expected := []byte("foobarbazqux\x0a0123456789abcdefghijABCDEFGHIJ")

	masterJsonUrl, _ := url.Parse("https://example.com/foo/bar/baz/qux/master.json")
	id := "1080p"
	output := new(bytes.Buffer)
	client := NewClient()
	client.Client = NewMockClient(func(req *http.Request) *http.Response {
		switch *req.URL {
		case *url0:
			return NewMockReponseFromBytes(body0)
		case *url1:
			return NewMockReponseFromBytes(body1)
		case *url2:
			return NewMockReponseFromBytes(body2)
		}

		t.Errorf("MockClient got unexpected request url: %v", req.URL.String())
		return nil
	})

	err := masterJson.CreateVideoFile(output, masterJsonUrl, id, client)
	if err != nil {
		t.Errorf("VideoSegmentUrls failed to create video: %v", err)
		return
	}

	actual := output.Bytes()
	if bytes.Compare(expected, actual) != 0 {
		t.Errorf("CreateVideoFile output does not match.\nexpected: %v\nactual:   %v", expected, actual)
		return
	}
}

func TestCreateAudioFile(t *testing.T) {
	masterJson := MasterJson{
		BaseUrl: "../",
		Audio: []Audio{
			Audio{
				Id:          "qux",
				BaseUrl:     "../audio/qux/chop/",
				InitSegment: "Zm9vYmFyYmF6cXV4Cg==",
				Segments: []Segment{
					Segment{Url: "segment-1.m4s"},
					Segment{Url: "segment-2.m4s"},
					Segment{Url: "segment-3.m4s"},
				},
			},
		},
	}
	url0, _ := url.Parse("https://example.com/foo/bar/audio/qux/chop/segment-1.m4s")
	url1, _ := url.Parse("https://example.com/foo/bar/audio/qux/chop/segment-2.m4s")
	url2, _ := url.Parse("https://example.com/foo/bar/audio/qux/chop/segment-3.m4s")
	body0 := []byte("0123456789")
	body1 := []byte("abcdefghij")
	body2 := []byte("ABCDEFGHIJ")
	expected := []byte("foobarbazqux\x0a0123456789abcdefghijABCDEFGHIJ")

	masterJsonUrl, _ := url.Parse("https://example.com/foo/bar/video/baz/master.json")
	id := "qux"
	output := new(bytes.Buffer)
	client := NewClient()
	client.Client = NewMockClient(func(req *http.Request) *http.Response {
		switch *req.URL {
		case *url0:
			return NewMockReponseFromBytes(body0)
		case *url1:
			return NewMockReponseFromBytes(body1)
		case *url2:
			return NewMockReponseFromBytes(body2)
		}

		t.Errorf("MockClient got unexpected request url: %v", req.URL.String())
		return nil
	})

	err := masterJson.CreateAudioFile(output, masterJsonUrl, id, client)
	if err != nil {
		t.Errorf("VideoSegmentUrls failed to create video: %v", err)
		return
	}

	actual := output.Bytes()
	if bytes.Compare(expected, actual) != 0 {
		t.Errorf("CreateAudioFile output does not match.\nexpected: %v\nactual:   %v", expected, actual)
		return
	}
}

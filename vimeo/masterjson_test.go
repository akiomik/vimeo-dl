package vimeo

import (
  "bytes"
  "net/http"
  "net/url"
  "reflect"
  "testing"
)

func TestDecodedInitSegment(t *testing.T) {
  masterJson := MasterJson{
    Video: []Video{
      Video{
        Id: "1080p",
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

func TestFindVideo(t *testing.T) {
  masterJson := MasterJson{
    Video: []Video{
      Video{
        Id: "240p",
        Segments: make([]Segment, 0),
      },
      Video{
        Id: "360p",
        Segments: make([]Segment, 0),
      },
      Video{
        Id: "720p",
        Segments: make([]Segment, 0),
      },
      Video{
        Id: "1080p",
        Segments: make([]Segment, 0),
      },
    },
  }

  for _, scale := range []string{"240p", "360p", "720p", "1080p"} {
    v, _ := masterJson.FindVideo(scale)
    if v.Id != scale {
      t.Errorf("FindVideo video id does not match.\nexpected: %v\nactual:   %v", scale, v.Id)
      return
    }
  }

  _, err := masterJson.FindVideo("notfound")
  if err == nil {
    t.Errorf("FindVideo err must be nil when video is not found.")
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
          Segment{ Url: "1080p.mp4?range=0-9" },
          Segment{ Url: "1080p.mp4?range=10-19" },
          Segment{ Url: "1080p.mp4?range=20-29" },
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

func TestCreateVideoFile(t *testing.T) {
  masterJson := MasterJson{
    BaseUrl: "../../../parcel/archive/",
    Video: []Video{
      Video{
        Id: "1080p",
        InitSegment: "Zm9vYmFyYmF6cXV4Cg==",
        Segments: []Segment{
          Segment{ Url: "1080p.mp4?range=0-9" },
          Segment{ Url: "1080p.mp4?range=10-19" },
          Segment{ Url: "1080p.mp4?range=20-29" },
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
  scale := "1080p"
  output := new(bytes.Buffer)
  client := NewClient()
  client.Client = NewMockClient(func (req *http.Request) *http.Response {
    switch (*req.URL) {
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

  err := masterJson.CreateVideoFile(output, masterJsonUrl, scale, client)
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

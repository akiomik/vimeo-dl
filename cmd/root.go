package cmd

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"os/exec"

	"github.com/akiomik/vimeo-dl/config"
	"github.com/akiomik/vimeo-dl/vimeo"
	"github.com/spf13/cobra"
)

var (
	input     string
	userAgent string
	videoId   string
	audioId   string
	combine   bool
)

var rootCmd = &cobra.Command{
	Use:     "vimeo-dl",
	Short:   "vimeo-dl " + config.Version,
	Version: config.Version,
	Run: func(cmd *cobra.Command, args []string) {
		client := vimeo.NewClient()
		if len(userAgent) > 0 {
			client.UserAgent = userAgent
		}

		masterJsonUrl, err := url.Parse(input)
		if err != nil {
			fmt.Println("Error:", err.Error())
			os.Exit(1)
		}

		masterJson, err := client.GetMasterJson(masterJsonUrl)
		if err != nil {
			fmt.Println("Error:", err.Error())
			os.Exit(1)
		}

		videoOutputFilename := masterJson.ClipId + "-video.mp4"
		err = createVideo(client, masterJson, masterJsonUrl, videoOutputFilename)
		if err != nil {
			fmt.Println("Error:", err.Error())

			if _, ok := err.(base64.CorruptInputError); ok {
				query := masterJsonUrl.Query()
				query.Add("base64_init", "1")
				query.Del("query_string_ranges")
				masterJsonUrl.RawQuery = query.Encode()
				fmt.Println("Try this url:", masterJsonUrl.String())
			}

			os.Exit(1)
		}

		if len(masterJson.Audio) > 0 {
			audioOutputFilename := masterJson.ClipId + "-audio.mp4"
			err = createAudio(client, masterJson, masterJsonUrl, audioOutputFilename)
			if err != nil {
				fmt.Println("Error:", err.Error())
				os.Exit(1)
			}

			if combine {
				outputFilename := masterJson.ClipId + ".mp4"
				err = combineVideoAndAudio(videoOutputFilename, audioOutputFilename, outputFilename)
				if err != nil {
					fmt.Println("Error:", err.Error())
					os.Exit(1)
				}
			}
		}

		fmt.Println("Done!")
	},
}

func init() {
	rootCmd.Flags().StringVarP(&input, "input", "i", "", "url for master.json (required)")
	rootCmd.Flags().StringVarP(&userAgent, "user-agent", "", "", "user-agent for request")
	rootCmd.Flags().StringVarP(&videoId, "video-id", "", "", "video id")
	rootCmd.Flags().StringVarP(&audioId, "audio-id", "", "", "audio id")
	rootCmd.Flags().BoolVarP(&combine, "combine", "", false, "combine video and audio into a single mp4 (ffmpeg is required)")
	rootCmd.MarkFlagRequired("input")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println("Error:", err.Error())
		os.Exit(1)
	}
}

func createVideo(client *vimeo.Client, masterJson *vimeo.MasterJson, masterJsonUrl *url.URL, outputFilename string) error {
	videoFile, err := os.OpenFile(outputFilename, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer videoFile.Close()
	fmt.Println("Downloading to " + outputFilename)

	if len(videoId) == 0 {
		videoId = masterJson.FindMaximumBitrateVideo().Id
	}

	err = masterJson.CreateVideoFile(videoFile, masterJsonUrl, videoId, client)
	if err != nil {
		return err
	}

	return nil
}

func createAudio(client *vimeo.Client, masterJson *vimeo.MasterJson, masterJsonUrl *url.URL, outputFilename string) error {
	audioFile, err := os.OpenFile(outputFilename, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer audioFile.Close()
	fmt.Println("Downloading to " + outputFilename)

	if len(audioId) == 0 {
		audioId = masterJson.FindMaximumBitrateAudio().Id
	}

	err = masterJson.CreateAudioFile(audioFile, masterJsonUrl, audioId, client)
	if err != nil {
		return err
	}

	return nil
}

func combineVideoAndAudio(videoFilename string, audioFilename string, outputFilename string) error {
	err := exec.Command("ffmpeg", "-version").Run()
	if err != nil {
		return err
	}

	err = exec.Command("ffmpeg", "-i", videoFilename, "-i", audioFilename, "-c", "copy", outputFilename).Run()
	if err != nil {
		return err
	}

	err = os.Remove(videoFilename)
	if err != nil {
		return err
	}

	err = os.Remove(audioFilename)
	if err != nil {
		return err
	}

	return nil
}

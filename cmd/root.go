package cmd

import (
	"fmt"
	"net/url"
	"os"

	"github.com/akiomik/vimeo-dl/config"
	"github.com/akiomik/vimeo-dl/vimeo"
	"github.com/spf13/cobra"
)

var (
	input     string
	userAgent string
	videoId   string
	audioId   string
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
			fmt.Println(err.Error())
			os.Exit(1)
		}

		masterJson, err := client.GetMasterJson(masterJsonUrl)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		err = createVideo(client, masterJson, masterJsonUrl)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		if len(masterJson.Audio) > 0 {
			err = createAudio(client, masterJson, masterJsonUrl)
			if err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
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
	rootCmd.MarkFlagRequired("input")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func createVideo(client *vimeo.Client, masterJson *vimeo.MasterJson, masterJsonUrl *url.URL) error {
	videoOutput := masterJson.ClipId + "-video.mp4"
	videoFile, err := os.OpenFile(videoOutput, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer videoFile.Close()
	fmt.Println("Downloading to " + videoOutput)

	if len(videoId) == 0 {
		videoId = masterJson.FindMaximumBitrateVideo().Id
	}

	err = masterJson.CreateVideoFile(videoFile, masterJsonUrl, videoId, client)
	if err != nil {
		return err
	}

	return nil
}

func createAudio(client *vimeo.Client, masterJson *vimeo.MasterJson, masterJsonUrl *url.URL) error {
	audioOutput := masterJson.ClipId + "-audio.mp4"
	audioFile, err := os.OpenFile(audioOutput, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer audioFile.Close()
	fmt.Println("Downloading to " + audioOutput)

	if len(audioId) == 0 {
		audioId = masterJson.FindMaximumBitrateAudio().Id
	}

	err = masterJson.CreateAudioFile(audioFile, masterJsonUrl, audioId, client)
	if err != nil {
		return err
	}

	return nil
}

package cmd

import (
	"fmt"
	"net/url"
	"os"

	"github.com/akiomik/vimeo-dl/vimeo"
	"github.com/spf13/cobra"
)

var (
	input     string
	output    string
	userAgent string
	scale     string
)

var rootCmd = &cobra.Command{
	Use:   "vimeo-dl",
	Short: "vimeo downloader",
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

		if len(output) == 0 {
			output = masterJson.ClipId + ".mp4"
		}
		f, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer f.Close()
		fmt.Println("Downloading to " + output)

		if len(scale) == 0 {
			scale = masterJson.Video[len(masterJson.Video)-1].Id
		}

		err = masterJson.CreateVideoFile(f, masterJsonUrl, scale, client)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.Flags().StringVarP(&input, "input", "i", "", "url for master.json (required)")
	rootCmd.Flags().StringVarP(&output, "output", "o", "", "output file name")
	rootCmd.Flags().StringVarP(&userAgent, "user-agent", "", "", "user-agent for request")
	rootCmd.Flags().StringVarP(&scale, "scale", "s", "", "scale")
	rootCmd.MarkFlagRequired("input")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

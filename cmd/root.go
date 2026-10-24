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
	"fmt"
	"os"

	"github.com/akiomik/vimeo-dl/config"
	"github.com/spf13/cobra"
)

var (
	input          string
	userAgent      string
	videoId        string
	audioId        string
	outputFilename string
	combine        bool
)

var rootCmd = &cobra.Command{
	Use:     "vimeo-dl",
	Short:   "vimeo-dl " + config.Version,
	Version: config.Version,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDownload(downloadOptions{
			Input: input, UserAgent: userAgent, VideoID: videoId,
			AudioID: audioId, OutputFilename: outputFilename, Combine: combine,
		}); err != nil {
			fmt.Println("Error:", err.Error())
			os.Exit(1)
		}
		fmt.Println("Done!")
	},
}

func init() {
	rootCmd.Flags().StringVarP(&input, "input", "i", "", "url for master.json or playlist.json (required)")
	rootCmd.Flags().StringVarP(&userAgent, "user-agent", "", "", "user-agent for request")
	rootCmd.Flags().StringVarP(&videoId, "video-id", "", "", "video id")
	rootCmd.Flags().StringVarP(&audioId, "audio-id", "", "", "audio id")
	rootCmd.Flags().StringVarP(&outputFilename, "output-file-name", "o", "", "output file name")
	rootCmd.Flags().BoolVarP(&combine, "combine", "", false, "combine video and audio into a single mp4 (ffmpeg is required)")
	rootCmd.MarkFlagRequired("input")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println("Error:", err.Error())
		os.Exit(1)
	}
}

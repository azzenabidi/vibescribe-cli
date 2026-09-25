package main

import (
	"github.com/spf13/cobra"
)

var version = "dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "vibescribe",
		Short: "Local, offline video-to-text transcription with whisper.cpp",
		Long: `VibeScribe transcribes video and audio files into text on your own
machine using whisper.cpp. Audio is extracted with system ffmpeg and
transcribed with the whisper.cpp CLI. No media ever leaves your device.`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		newTranscribeCmd(),
		newModelCmd(),
	)
	return root
}

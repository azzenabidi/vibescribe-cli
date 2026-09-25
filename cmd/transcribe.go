package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/azzenabidi/vibescribe-cli/pkg/audio"
	"github.com/azzenabidi/vibescribe-cli/pkg/exporter"
	"github.com/azzenabidi/vibescribe-cli/pkg/model"
	"github.com/azzenabidi/vibescribe-cli/pkg/transcriber"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

func newTranscribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transcribe",
		Short: "Transcribe a video or audio file to text",
		Long: `Extract the audio track from a video or audio file with system ffmpeg and
transcribe it locally with whisper.cpp.

The whisper.cpp engine (whisper-cli) is used if available on PATH, or can be
pointed to explicitly with the VIBESCRIBE_WHISPER_CLI environment variable.`,
		Args: cobra.NoArgs,
		RunE: runTranscribe,
	}

	fl := cmd.Flags()
	fl.StringP("input", "i", "", "path to the input video or audio file (required)")
	fl.StringP("model", "m", "base", "model size (tiny, base, small, medium, large-v3) or a path to a .bin GGML model")
	fl.StringP("format", "f", "txt,srt", "comma-separated export formats: txt, srt, vtt, json, html")
	fl.StringP("output-dir", "o", ".", "directory to write transcripts into")
	fl.StringP("language", "l", "auto", "spoken language (e.g. en, fr, es) or 'auto' to auto-detect")
	fl.IntP("threads", "t", 0, "number of CPU threads for inference (default: all logical CPUs)")
	fl.Bool("yes", false, "auto-confirm model download prompts")
	fl.Bool("keep-wav", false, "keep the extracted temporary WAV file")

	cobra.CheckErr(cmd.MarkFlagRequired("input"))
	return cmd
}

func runTranscribe(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	input, _ := cmd.Flags().GetString("input")
	modelRef, _ := cmd.Flags().GetString("model")
	formatRaw, _ := cmd.Flags().GetString("format")
	outputDir, _ := cmd.Flags().GetString("output-dir")
	language, _ := cmd.Flags().GetString("language")
	threads, _ := cmd.Flags().GetInt("threads")
	yes, _ := cmd.Flags().GetBool("yes")
	keepWav, _ := cmd.Flags().GetBool("keep-wav")

	formats, err := exporter.ParseFormats(formatRaw)
	if err != nil {
		return fmt.Errorf("invalid -f: %w", err)
	}

	if _, err := os.Stat(input); err != nil {
		return fmt.Errorf("input file: %w", err)
	}

	modelPath, err := model.Resolve(modelRef)
	if err != nil {
		return err
	}

	if !model.Exists(modelPath) {
		if isModelName(modelRef) {
			if ok, err := confirmDownloadOrAbort(ctx, modelRef, modelPath, yes); err != nil {
				return err
			} else if !ok {
				return nil
			}
		} else {
			return fmt.Errorf("model file not found: %s", modelPath)
		}
	}

	if err := audio.RequireFFmpeg(); err != nil {
		return err
	}

	tmp, err := os.MkdirTemp("", "vibescribe-")
	if err != nil {
		return err
	}
	if !keepWav {
		defer os.RemoveAll(tmp)
	}
	wavPath := filepath.Join(tmp, "audio.wav")

	if err := extractAudio(ctx, input, wavPath); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr)
	bar := progressbar.NewOptions64(
		100,
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionSetDescription("Transcribing"),
	)
	last := 0
	result, err := transcriber.Transcribe(ctx, transcriber.Options{
		ModelPath: modelPath,
		WavPath:   wavPath,
		Language:  language,
		Threads:   threads,
		Progress: func(pct int) {
			if pct > last {
				_ = bar.Add(pct - last)
				last = pct
			}
		},
	})
	if err != nil {
		return err
	}
	bar.Finish()

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("output-dir: %w", err)
	}

	base := filepath.Join(outputDir, strings.TrimSuffix(filepath.Base(input), filepath.Ext(input)))
	written := make([]string, 0, len(formats))
	for _, format := range formats {
		path := base + exporter.Format(format)
		if err := exporter.Write(path, format, result.Segments); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
		written = append(written, path)
	}

	fmt.Fprintln(os.Stderr)
	fmt.Println("Transcription complete")
	lang := result.DetectedLanguage
	if lang == "" {
		lang = "auto"
	}
	fmt.Printf("  Language:   %s\n", lang)
	fmt.Printf("  Segments:   %d\n", len(result.Segments))
	if len(written) > 0 {
		fmt.Println("  Output:")
		for _, p := range written {
			fmt.Printf("    %s\n", p)
		}
	}
	return nil
}

func extractAudio(ctx context.Context, input, wavPath string) error {
	theme := 14
	bar := progressbar.NewOptions64(
		-1,
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionSpinnerType(theme),
		progressbar.OptionSetDescription("Extracting audio track"),
	)
	err := audio.Extract(ctx, input, wavPath, func(fraction float64) {
		if fraction >= 0 {
			bar.Describe(fmt.Sprintf("Extracting audio track %3.0f%%", fraction*100))
		}
	})
	if err != nil {
		bar.Clear()
		return fmt.Errorf("audio extraction: %w", err)
	}
	bar.Finish()
	return nil
}

func isModelName(ref string) bool {
	return model.ValidNames(ref)
}

func confirmDownloadOrAbort(ctx context.Context, name, dest string, yes bool) (bool, error) {
	sizeMB := ""
	if size, err := model.RemoteSize(name); err == nil && size > 0 {
		sizeMB = fmt.Sprintf(" (~%.1f MB)", float64(size)/(1024*1024))
	}
	fmt.Fprintf(os.Stderr, "Model %q is not in the cache.%s\n", name, sizeMB)

	if yes {
		_ = model.EnsureCacheDir()
		if err := downloadModel(ctx, name, dest); err != nil {
			return false, err
		}
		return true, nil
	}

	ok, err := confirm("Download it now?", true)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, errors.New("aborted: model not downloaded")
	}
	if err := downloadModel(ctx, name, dest); err != nil {
		return false, err
	}
	return true, nil
}

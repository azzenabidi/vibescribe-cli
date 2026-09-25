// Package audio wraps system ffmpeg to extract a 16 kHz mono 16-bit PCM WAV
// track from a video or audio file, which is the format whisper.cpp expects.
package audio

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var errFFmpegMissing = errors.New("ffmpeg is required but was not found on PATH; install it (e.g. sudo pacman -S ffmpeg)")

var timeRe = regexp.MustCompile(`time=(\d+):(\d+):(\d+(?:\.\d+)?)`)

// RequireFFmpeg checks that both ffmpeg and ffprobe are available on PATH and
// returns a descriptive error otherwise.
func RequireFFmpeg() error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return errFFmpegMissing
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return errors.New("ffprobe (ships with ffmpeg) is required for progress reporting")
	}
	return nil
}

// Duration returns the media duration in seconds using ffprobe. A non-nil
// error indicates the duration could not be determined.
func Duration(ctx context.Context, input string) (float64, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		input,
	)
	raw, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe: %w", err)
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("ffprobe: invalid duration %q", strings.TrimSpace(string(raw)))
	}
	return duration, nil
}

// Extract runs ffmpeg to convert input into a 16 kHz mono 16-bit PCM WAV at
// wavPath. When onProgress is non-nil it is called with the extraction
// fraction (0..1) as ffmpeg reports time progress; if the duration cannot be
// determined it is called with -1.
func Extract(ctx context.Context, input, wavPath string, onProgress func(fraction float64)) error {
	duration, _ := Duration(ctx, input)

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner", "-y",
		"-i", input,
		"-vn",
		"-ar", "16000",
		"-ac", "1",
		"-c:a", "pcm_s16le",
		wavPath,
	)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}

	if onProgress != nil {
		onProgress(0)
		go func() {
			scanner := bufio.NewScanner(stderr)
			scanner.Buffer(make([]byte, 64<<10), 64<<10)
			for scanner.Scan() {
				line := scanner.Text()
				m := timeRe.FindStringSubmatch(line)
				if m == nil {
					continue
				}
				hh, _ := strconv.ParseFloat(m[1], 64)
				mm, _ := strconv.ParseFloat(m[2], 64)
				ss, _ := strconv.ParseFloat(m[3], 64)
				elapsed := hh*3600 + mm*60 + ss
				if duration <= 0 {
					onProgress(-1)
				} else {
					f := elapsed / duration
					if f < 0 {
						f = 0
					}
					if f > 1 {
						f = 1
					}
					onProgress(f)
				}
			}
		}()
	} else {
		go drain(stderr)
	}

	// doscan is consumed by the progress goroutine above; wait for ffmpeg.
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}

	info, err := os.Stat(wavPath)
	if err != nil || info.Size() < 44 {
		return errors.New("ffmpeg produced no usable audio (wrong file type or no audio stream?)")
	}
	if onProgress != nil {
		onProgress(1)
	}
	return nil
}

func drain(r interface{ Read([]byte) (int, error) }) {
	buf := make([]byte, 4096)
	for {
		if _, err := r.Read(buf); err != nil {
			return
		}
	}
}

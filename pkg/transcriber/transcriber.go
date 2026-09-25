// Package transcriber drives the whisper.cpp command-line engine to turn a
// 16 kHz mono WAV file into timestamped text segments.
package transcriber

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// Segment is one transcribed utterance with millisecond offsets.
type Segment struct {
	ID      int    `json:"id"`
	StartMs int64  `json:"start_ms"`
	EndMs   int64  `json:"end_ms"`
	Text    string `json:"text"`
}

// Result is the outcome of a transcription run.
type Result struct {
	DetectedLanguage string
	Segments         []Segment
}

// Options configures a transcription run. Progress, when non-nil, is invoked
// with the inference percentage (0..100) as whisper.cpp reports it.
type Options struct {
	ModelPath string
	WavPath   string
	Language  string
	Threads   int
	Progress  func(percent int)
}

// ErrEngineNotFound is returned when no whisper-cli executable can be located.
var ErrEngineNotFound = errors.New(
	"whisper-cli was not found on PATH; install whisper.cpp (see README) or set VIBESCRIBE_WHISPER_CLI")

// EnginePath locates the whisper.cpp CLI binary. The VIBESCRIBE_WHISPER_CLI
// environment variable takes precedence over a PATH lookup.
func EnginePath() (string, error) {
	if p := strings.TrimSpace(os.Getenv("VIBESCRIBE_WHISPER_CLI")); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("VIBESCRIBE_WHISPER_CLI points at %q: %w", p, err)
		}
		return p, nil
	}
	if p, err := exec.LookPath("whisper-cli"); err == nil {
		return p, nil
	}
	return "", ErrEngineNotFound
}

// NumCPU is the number of logical CPUs used when -t/--threads is not given.
var NumCPU = runtime.NumCPU

var progressRe = regexp.MustCompile(`progress\s*=\s*(\d+)%`)

// whisperOut mirrors the JSON file whisper-cli writes with -oj. Model metadata
// is intentionally ignored; only the detected language and segments matter.
type whisperOut struct {
	Result struct {
		Language string `json:"language"`
	} `json:"result"`
	Transcription []struct {
		Offsets struct {
			From int64 `json:"from"`
			To   int64 `json:"to"`
		} `json:"offsets"`
		Text string `json:"text"`
	} `json:"transcription"`
}

// Transcribe runs whisper-cli against the given WAV and returns the parsed
// segments. stderr is drained concurrently (so a full pipe cannot block the
// engine), progress lines are surfaced through opts.Progress, and the scratch
// directory is removed before returning.
func Transcribe(ctx context.Context, opts Options) (*Result, error) {
	if opts.Threads <= 0 {
		opts.Threads = NumCPU()
	}
	if opts.Language == "" {
		opts.Language = "auto"
	}
	if opts.ModelPath == "" {
		return nil, errors.New("transcriber: empty model path")
	}
	if _, err := os.Stat(opts.WavPath); err != nil {
		return nil, fmt.Errorf("transcriber: audio file: %w", err)
	}

	engine, err := EnginePath()
	if err != nil {
		return nil, err
	}

	tmp, err := os.MkdirTemp("", "vibescribe-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	outBase := filepath.Join(tmp, "transcription")

	args := []string{
		"-m", opts.ModelPath,
		"-f", opts.WavPath,
		"-l", opts.Language,
		"-t", strconv.Itoa(opts.Threads),
		"-oj", "-of", outBase,
		"-pp",
	}

	cmd := exec.CommandContext(ctx, engine, args...)
	cmd.Stdout = io.Discard

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	var progressBuf strings.Builder
	go func() {
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 64<<10), 64<<10)
		for scanner.Scan() {
			line := scanner.Text()
			if line != "" && progressBuf.Len() < 8<<10 {
				progressBuf.WriteString(line)
				progressBuf.WriteByte('\n')
			}
			if m := progressRe.FindStringSubmatch(line); m != nil {
				if pct, err := strconv.Atoi(m[1]); err == nil && pct >= 0 && pct <= 100 && opts.Progress != nil {
					opts.Progress(pct)
				}
			}
		}
	}()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("launching whisper-cli: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("transcription cancelled: %w", ctxErr)
		}
		return nil, fmt.Errorf("whisper-cli failed: %w%s", err, tail(&progressBuf))
	}

	return parseOutput(outBase + ".json")
}

// parseOutput reads the JSON emitted by whisper-cli -oj and maps it to a
// Result. Offsets in that file are already in milliseconds.
func parseOutput(jsonPath string) (*Result, error) {
	raw, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("whisper-cli produced no JSON output at %s: %w", jsonPath, err)
	}
	var out whisperOut
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parsing whisper-cli output %s: %w", jsonPath, err)
	}

	res := &Result{DetectedLanguage: out.Result.Language}
	for i, seg := range out.Transcription {
		text := strings.TrimSpace(seg.Text)
		if text == "" {
			continue
		}
		res.Segments = append(res.Segments, Segment{
			ID:      i,
			StartMs: seg.Offsets.From,
			EndMs:   seg.Offsets.To,
			Text:    text,
		})
	}
	return res, nil
}

func tail(buf *strings.Builder) string {
	if buf.Len() == 0 {
		return ""
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) <= 4 {
		return "\n  stderr: " + strings.Join(lines, "\n           ")
	}
	return "\n  stderr (last lines):\n  " + strings.Join(lines[len(lines)-4:], "\n  ")
}

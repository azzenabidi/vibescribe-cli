// Package exporter writes transcription segments to the supported output
// formats: plain text, SubRip subtitles, WebVTT, JSON, and HTML.
package exporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"strings"
	"time"

	"github.com/azzenabidi/vibescribe-cli/pkg/transcriber"
)

// Format returns the file extension (including the dot) for a canonical
// format name.
func Format(name string) string {
	switch name {
	case "txt":
		return ".txt"
	case "srt":
		return ".srt"
	case "vtt":
		return ".vtt"
	case "json":
		return ".json"
	case "html":
		return ".html"
	default:
		return ""
	}
}

// Supports reports whether name is a known export format.
func Supports(name string) bool {
	return Format(name) != ""
}

// ParseFormats expands a comma-separated format list, trims whitespace,
// lowercases, and validates each entry.
func ParseFormats(raw string) ([]string, error) {
	var formats []string
	for _, part := range strings.Split(raw, ",") {
		name := strings.ToLower(strings.TrimSpace(part))
		if name == "" {
			continue
		}
		if !Supports(name) {
			return nil, fmt.Errorf("unknown export format %q (supported: txt, srt, vtt, json, html)", name)
		}
		formats = append(formats, name)
	}
	if len(formats) == 0 {
		return nil, fmt.Errorf("no export formats specified (supported: txt, srt, vtt, json, html)")
	}
	return formats, nil
}

// Write renders segments to the named format at path and returns the canonical
// format it produced.
func Write(path, format string, segs []transcriber.Segment) error {
	switch format {
	case "txt":
		return ToTXT(path, segs)
	case "srt":
		return ToSRT(path, segs)
	case "vtt":
		return ToVTT(path, segs)
	case "json":
		return ToJSON(path, segs)
	case "html":
		return ToHTML(path, segs)
	default:
		return fmt.Errorf("unknown export format %q", format)
	}
}

// ToTXT writes one segment per line.
func ToTXT(path string, segs []transcriber.Segment) error {
	var b bytes.Buffer
	for _, s := range segs {
		b.WriteString(s.Text)
		b.WriteByte('\n')
	}
	return writeFile(path, b.Bytes())
}

// ToSRT writes SubRip subtitles with millisecond timestamps.
func ToSRT(path string, segs []transcriber.Segment) error {
	var b bytes.Buffer
	for i, s := range segs {
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", i+1, srtStamp(s.StartMs), srtStamp(s.EndMs), s.Text)
	}
	return writeFile(path, b.Bytes())
}

// ToVTT writes WebVTT subtitles with millisecond timestamps.
func ToVTT(path string, segs []transcriber.Segment) error {
	var b bytes.Buffer
	b.WriteString("WEBVTT\n\n")
	for _, s := range segs {
		fmt.Fprintf(&b, "%s --> %s\n%s\n\n", vttStamp(s.StartMs), vttStamp(s.EndMs), s.Text)
	}
	return writeFile(path, b.Bytes())
}

// ToJSON writes a compact array of { id, start_ms, end_ms, text } objects.
func ToJSON(path string, segs []transcriber.Segment) error {
	type segmentJSON struct {
		ID      int    `json:"id"`
		StartMs int64  `json:"start_ms"`
		EndMs   int64  `json:"end_ms"`
		Text    string `json:"text"`
	}
	out := make([]segmentJSON, 0, len(segs))
	for i, s := range segs {
		out = append(out, segmentJSON{ID: i, StartMs: s.StartMs, EndMs: s.EndMs, Text: s.Text})
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, append(raw, '\n'))
}

// ToHTML emits a minimal self-contained transcript page with CSS styling.
func ToHTML(path string, segs []transcriber.Segment) error {
	var b bytes.Buffer
	b.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString("<title>VibeScribe Transcript</title>\n")
	b.WriteString("<style>")
	b.WriteString("body{font-family:system-ui,-apple-system,'Segoe UI',Roboto,sans-serif;max-width:60em;margin:2em auto;padding:0 1em;line-height:1.65;color:#1f2328}")
	b.WriteString("h1{font-size:1.5rem}.seg{display:block;margin:0.7em 0}time{color:#57606a;font-variant-numeric:tabular-nums;font-size:.85em}")
	b.WriteString("</style>\n</head>\n<body>\n<h1>Transcription</h1>\n")
	for _, s := range segs {
		fmt.Fprintf(&b, "<p class=\"seg\"><time>[%s – %s]</time><br>%s</p>\n",
			stampSeconds(s.StartMs), stampSeconds(s.EndMs), html.EscapeString(s.Text))
	}
	b.WriteString("</body>\n</html>\n")
	return writeFile(path, b.Bytes())
}

func srtStamp(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	return fmt.Sprintf("%02d:%02d:%02d,%03d",
		int(d/time.Hour), int(d/time.Minute)%60, int(d/time.Second)%60, int(d/time.Millisecond)%1000)
}

func vttStamp(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	return fmt.Sprintf("%02d:%02d:%02d.%03d",
		int(d/time.Hour), int(d/time.Minute)%60, int(d/time.Second)%60, int(d/time.Millisecond)%1000)
}

func stampSeconds(ms int64) string {
	return fmt.Sprintf("%.1fs", float64(ms)/1000.0)
}

func writeFile(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	return nil
}

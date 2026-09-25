package exporter

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/azzenabidi/vibescribe-cli/pkg/transcriber"
)

func TestParseFormats(t *testing.T) {
	formats, err := ParseFormats(" TXT, srt ,vtt,json,html ")
	if err != nil {
		t.Fatalf("ParseFormats returned an error: %v", err)
	}

	want := []string{"txt", "srt", "vtt", "json", "html"}
	if !reflect.DeepEqual(formats, want) {
		t.Fatalf("ParseFormats returned %v, want %v", formats, want)
	}
}

func TestParseFormatsRejectsInvalidInput(t *testing.T) {
	for _, raw := range []string{"", ",,", "txt,pdf"} {
		t.Run(raw, func(t *testing.T) {
			if _, err := ParseFormats(raw); err == nil {
				t.Fatalf("ParseFormats(%q) returned no error", raw)
			}
		})
	}
}

func TestWrite(t *testing.T) {
	segments := []transcriber.Segment{
		{ID: 8, StartMs: 1500, EndMs: 2750, Text: "Hello <world>"},
		{ID: 42, StartMs: 3600000, EndMs: 3661250, Text: "Second line"},
	}

	tests := map[string]string{
		"txt":  "Hello <world>\nSecond line\n",
		"srt":  "1\n00:00:01,500 --> 00:00:02,750\nHello <world>\n\n2\n01:00:00,000 --> 01:01:01,250\nSecond line\n\n",
		"vtt":  "WEBVTT\n\n00:00:01.500 --> 00:00:02.750\nHello <world>\n\n01:00:00.000 --> 01:01:01.250\nSecond line\n\n",
		"json": "[\n  {\n    \"id\": 0,\n    \"start_ms\": 1500,\n    \"end_ms\": 2750,\n    \"text\": \"Hello \\u003cworld\\u003e\"\n  },\n  {\n    \"id\": 1,\n    \"start_ms\": 3600000,\n    \"end_ms\": 3661250,\n    \"text\": \"Second line\"\n  }\n]\n",
		"html": "<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n<title>VibeScribe Transcript</title>\n<style>body{font-family:system-ui,-apple-system,'Segoe UI',Roboto,sans-serif;max-width:60em;margin:2em auto;padding:0 1em;line-height:1.65;color:#1f2328}h1{font-size:1.5rem}.seg{display:block;margin:0.7em 0}time{color:#57606a;font-variant-numeric:tabular-nums;font-size:.85em}</style>\n</head>\n<body>\n<h1>Transcription</h1>\n<p class=\"seg\"><time>[1.5s – 2.8s]</time><br>Hello &lt;world&gt;</p>\n<p class=\"seg\"><time>[3600.0s – 3661.2s]</time><br>Second line</p>\n</body>\n</html>\n",
	}

	for format, want := range tests {
		t.Run(format, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "transcript"+Format(format))
			if err := Write(path, format, segments); err != nil {
				t.Fatalf("Write returned an error: %v", err)
			}

			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile returned an error: %v", err)
			}
			if string(got) != want {
				t.Fatalf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
		})
	}
}

func TestWriteRejectsUnknownFormat(t *testing.T) {
	err := Write(filepath.Join(t.TempDir(), "transcript.txt"), "pdf", nil)
	if err == nil || !strings.Contains(err.Error(), "unknown export format") {
		t.Fatalf("Write returned %v, want unknown format error", err)
	}
}

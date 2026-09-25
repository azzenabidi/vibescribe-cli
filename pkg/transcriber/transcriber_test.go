package transcriber

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transcription.json")
	raw := `{
		"result": {"language": "en"},
		"transcription": [
			{"offsets": {"from": 0, "to": 1200}, "text": " Hello "},
			{"offsets": {"from": 1200, "to": 0}, "text": "   "},
			{"offsets": {"from": 1250, "to": 3100}, "text": "world"}
		]
	}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := parseOutput(path)
	if err != nil {
		t.Fatalf("parseOutput returned an error: %v", err)
	}
	want := &Result{
		DetectedLanguage: "en",
		Segments: []Segment{
			{ID: 0, StartMs: 0, EndMs: 1200, Text: "Hello"},
			{ID: 2, StartMs: 1250, EndMs: 3100, Text: "world"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseOutput returned %+v, want %+v", got, want)
	}
}

func TestParseOutputRejectsInvalidFiles(t *testing.T) {
	dir := t.TempDir()

	if _, err := parseOutput(filepath.Join(dir, "missing.json")); err == nil {
		t.Fatal("parseOutput accepted a missing file")
	}

	malformed := filepath.Join(dir, "malformed.json")
	if err := os.WriteFile(malformed, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := parseOutput(malformed); err == nil {
		t.Fatal("parseOutput accepted malformed JSON")
	}
}

func TestEnginePathPrefersEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom-whisper")
	if err := os.WriteFile(path, []byte("engine"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIBESCRIBE_WHISPER_CLI", path)

	got, err := EnginePath()
	if err != nil {
		t.Fatalf("EnginePath returned an error: %v", err)
	}
	if got != path {
		t.Fatalf("EnginePath returned %q, want %q", got, path)
	}
}

func TestEnginePathRejectsInvalidOverride(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-whisper")
	t.Setenv("VIBESCRIBE_WHISPER_CLI", missing)

	_, err := EnginePath()
	if err == nil || !strings.Contains(err.Error(), "VIBESCRIBE_WHISPER_CLI") {
		t.Fatalf("EnginePath returned %v, want invalid override error", err)
	}
}

func TestTail(t *testing.T) {
	var empty strings.Builder
	if got := tail(&empty); got != "" {
		t.Fatalf("tail returned %q for an empty builder", got)
	}

	var short strings.Builder
	short.WriteString("first\nsecond\n")
	if got, want := tail(&short), "\n  stderr: first\n           second"; got != want {
		t.Fatalf("tail returned %q, want %q", got, want)
	}

	var long strings.Builder
	long.WriteString("one\ntwo\nthree\nfour\nfive\n")
	if got, want := tail(&long), "\n  stderr (last lines):\n  two\n  three\n  four\n  five"; got != want {
		t.Fatalf("tail returned %q, want %q", got, want)
	}
}

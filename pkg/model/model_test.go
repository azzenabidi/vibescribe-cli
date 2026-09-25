package model

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNames(t *testing.T) {
	got := Names()
	want := []string{"base", "large-v3", "medium", "small", "tiny"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Names returned %v, want %v", got, want)
	}
}

func TestValidNamesAndFile(t *testing.T) {
	for _, name := range Names() {
		if !ValidNames(" " + strings.ToUpper(name) + " ") {
			t.Errorf("ValidNames rejected %q", name)
		}
		if File(" "+strings.ToUpper(name)+" ") != "ggml-"+name+".bin" {
			t.Errorf("File returned an unexpected name for %q", name)
		}
	}

	if ValidNames("unknown") {
		t.Fatal("ValidNames accepted an unknown model")
	}
}

func TestCacheDir(t *testing.T) {
	cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	got := CacheDir()
	want := filepath.Join(cacheHome, "vibescribe", "models")
	if got != want {
		t.Fatalf("CacheDir returned %q, want %q", got, want)
	}

	if err := EnsureCacheDir(); err != nil {
		t.Fatalf("EnsureCacheDir returned an error: %v", err)
	}
	info, err := os.Stat(want)
	if err != nil {
		t.Fatalf("cache directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("cache path %q is not a directory", want)
	}
}

func TestResolve(t *testing.T) {
	cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	got, err := Resolve(" Base ")
	if err != nil {
		t.Fatalf("Resolve returned an error: %v", err)
	}
	want := filepath.Join(cacheHome, "vibescribe", "models", "ggml-base.bin")
	if got != want {
		t.Fatalf("Resolve returned %q, want %q", got, want)
	}

	custom := filepath.Join(t.TempDir(), "custom.bin")
	got, err = Resolve(custom)
	if err != nil {
		t.Fatalf("Resolve returned an error for a model path: %v", err)
	}
	if got != custom {
		t.Fatalf("Resolve returned %q, want %q", got, custom)
	}

	if _, err := Resolve("unknown"); err == nil {
		t.Fatal("Resolve accepted an unknown model")
	}
}

func TestExists(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.bin")
	full := filepath.Join(dir, "full.bin")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("model"), 0o644); err != nil {
		t.Fatal(err)
	}

	if Exists(empty) {
		t.Fatal("Exists accepted an empty model")
	}
	if !Exists(full) {
		t.Fatal("Exists rejected a non-empty model")
	}
	if Exists(dir) {
		t.Fatal("Exists accepted a directory")
	}
	if Exists(filepath.Join(dir, "missing.bin")) {
		t.Fatal("Exists accepted a missing model")
	}
}

func TestScan(t *testing.T) {
	cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	if err := EnsureCacheDir(); err != nil {
		t.Fatal(err)
	}

	files := map[string][]byte{
		"ggml-tiny.bin":  []byte("tiny"),
		"ggml-base.bin":  []byte("base model"),
		"ggml-empty.bin": nil,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(CacheDir(), name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(CacheDir(), "nested.bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := Scan()
	if err != nil {
		t.Fatalf("Scan returned an error: %v", err)
	}
	want := CacheInfo{
		Files: []CacheFile{
			{Name: "ggml-base.bin", Size: 10},
			{Name: "ggml-tiny.bin", Size: 4},
		},
		TotalBytes: 14,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan returned %+v, want %+v", got, want)
	}
}

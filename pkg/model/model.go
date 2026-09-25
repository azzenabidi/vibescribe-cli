// Package model handles Whisper GGML model discovery, cache management, and
// downloading from Hugging Face.
package model

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// baseURL points at the default whisper.cpp model repository on Hugging Face.
// Models are addressed as <repo>/resolve/main/ggml-<name>.bin.
const baseURL = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main"

// knownModels maps a canonical model name to its remote filename.
var knownModels = map[string]string{
	"tiny":     "ggml-tiny.bin",
	"base":     "ggml-base.bin",
	"small":    "ggml-small.bin",
	"medium":   "ggml-medium.bin",
	"large-v3": "ggml-large-v3.bin",
}

// CacheDir returns the directory VibeScribe stores downloaded models in:
// $XDG_CACHE_HOME/vibescribe/models or ~/.cache/vibescribe/models.
func CacheDir() string {
	if d := os.Getenv("XDG_CACHE_HOME"); d != "" {
		return filepath.Join(d, "vibescribe", "models")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".cache/vibescribe/models"
	}
	return filepath.Join(home, ".cache", "vibescribe", "models")
}

// Names returns the canonical model names in a stable order.
func Names() []string {
	names := make([]string, 0, len(knownModels))
	for name := range knownModels {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ValidNames reports whether name is one of the known model sizes.
func ValidNames(name string) bool {
	_, ok := knownModels[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

// File returns the on-disk filename for a canonical model name.
func File(name string) string {
	return knownModels[strings.ToLower(strings.TrimSpace(name))]
}

// Resolve maps the -m/--model value to a local file path. If the value looks
// like a filesystem path (contains a separator, ends in .bin, or already
// exists), the value is returned as-is. Otherwise it is treated as a model
// size name resolved against the cache directory.
func Resolve(nameOrPath string) (string, error) {
	value := strings.TrimSpace(nameOrPath)
	if fileExists(value) || strings.ContainsRune(value, os.PathSeparator) || strings.HasSuffix(strings.ToLower(value), ".bin") {
		if value == "" {
			return "", fmt.Errorf("empty model name or path")
		}
		return value, nil
	}

	name := strings.ToLower(value)
	file, ok := knownModels[name]
	if !ok {
		return "", fmt.Errorf("unknown model %q (known sizes: %s)", nameOrPath, strings.Join(Names(), ", "))
	}
	return filepath.Join(CacheDir(), file), nil
}

// Exists reports whether a local model file is present and non-empty.
// A 0-byte file is treated as invalid because such a download is unusable.
func Exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}

// EnsureCacheDir creates the model cache directory if it does not exist.
func EnsureCacheDir() error {
	return os.MkdirAll(CacheDir(), 0o755)
}

// CacheFile describes a single model file present in the cache.
type CacheFile struct {
	Name string
	Size int64
}

// CacheInfo summarizes the contents and size of the model cache.
type CacheInfo struct {
	Files      []CacheFile
	TotalBytes int64
}

// Scan reads the model cache directory for usable model files.
func Scan() (CacheInfo, error) {
	var info CacheInfo
	entries, err := os.ReadDir(CacheDir())
	if err != nil {
		return info, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		full, err := os.Stat(filepath.Join(CacheDir(), e.Name()))
		if err != nil || full.Size() == 0 {
			continue
		}
		info.Files = append(info.Files, CacheFile{Name: e.Name(), Size: full.Size()})
		info.TotalBytes += full.Size()
	}
	sort.Slice(info.Files, func(i, j int) bool {
		return info.Files[i].Name < info.Files[j].Name
	})
	return info, nil
}

// RemoteSize returns the expected payload size in bytes for a known model by
// issuing a HEAD request to Hugging Face. A non-nil error (or -1) means the
// size could not be determined.
func RemoteSize(name string) (int64, error) {
	file, ok := knownModels[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return -1, fmt.Errorf("unknown model %q", name)
	}
	url := baseURL + "/" + file
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return -1, err
	}
	req.Header.Set("User-Agent", "vibescribe-cli (offline transcription)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()
	return resp.ContentLength, nil
}

// Download fetches a known model from Hugging Face into dest, reporting
// progress via the optional callback (done, total). It writes to a .part
// sibling and atomically renames the file into place on success.
func Download(ctx context.Context, name, dest string, progress func(done, total int64)) error {
	file, ok := knownModels[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return fmt.Errorf("unknown model %q", name)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	url := baseURL + "/" + file
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "vibescribe-cli (offline transcription)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: server returned %s", url, resp.Status)
	}

	part := dest + ".part"
	out, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}

	var written int64
	total := resp.ContentLength
	buf := make([]byte, 256<<10)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				out.Close()
				os.Remove(part)
				return err
			}
			written += int64(n)
			if progress != nil {
				progress(written, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			out.Close()
			os.Remove(part)
			return fmt.Errorf("download %s: %w", url, readErr)
		}
	}

	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(part)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(part)
		return err
	}

	if written == 0 {
		os.Remove(part)
		return fmt.Errorf("download %s: empty response", url)
	}

	if err := os.Rename(part, dest); err != nil {
		os.Remove(part)
		return err
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

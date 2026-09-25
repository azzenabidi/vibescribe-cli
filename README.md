# VibeScribe CLI

Local, offline video-to-text transcription powered by [whisper.cpp](https://github.com/ggml-org/whisper.cpp).

`vibescribe` extracts the audio track from a video or audio file with system
`ffmpeg`, runs it through a local Whisper GGML model, and writes transcripts in
TXT, SRT, VTT, JSON, or HTML formats. Nothing leaves your machine — the media
never leaves your device.

## Features

- **Fully local & offline** — no API keys, no network required for inference
- **5 output formats** — `txt`, `srt`, `vtt`, `json`, `html` (comma-separable)
- **Automatic model management** — downloads GGML models from Hugging Face into a cache, verifies downloads, and reuses them across runs
- **Auto language detection** — or force a language (`en`, `fr`, `es`, …)
- **Progress bars** — for audio extraction, model download, and transcription
- **Video/audio inputs** — MP4, MKV, MP3, FLAC, WAV, and anything else ffmpeg reads

## Requirements

- Go 1.22+ (to build)
- [ffmpeg](https://ffmpeg.org/) (+ `ffprobe`) for audio extraction
- The whisper.cpp `whisper-cli` for inference (see [Install the engine](#install-the-engine))

## Install

```bash
make install          # builds and copies vibescribe to /usr/local/bin
# or
go install github.com/azzenabidi/vibescribe-cli/cmd@latest
```

### Install the engine

The default engine is the whisper.cpp command-line tool:

```bash
make whisper-cli      # clones whisper.cpp v1.9.4, builds it, installs bin/whisper-cli
```

`vibescribe` locates `whisper-cli` on `PATH`, or you can point at a specific
binary (any whisper.cpp build, e.g. one compiled for the WhisperQtTranscriber
app) with the `VIBESCRIBE_WHISPER_CLI` environment variable:

```bash
export VIBESCRIBE_WHISPER_CLI=/path/to/whisper-cli
```

## Usage

```bash
vibescribe transcribe -i input.mp4 -m base -f txt,srt,json -o ./transcripts
```

| Flag | Default | Description |
|------|---------|-------------|
| `-i, --input` | *required* | Input video or audio file |
| `-m, --model` | `base` | Model size (`tiny`, `base`, `small`, `medium`, `large-v3`) or a path to a `.bin` GGML model |
| `-f, --format` | `txt,srt` | Comma-separated formats: `txt`, `srt`, `vtt`, `json`, `html` |
| `-o, --output-dir` | `.` | Directory for transcript files |
| `-l, --language` | `auto` | Spoken language (`en`, `fr`, `es`, …) or `auto` |
| `-t, --threads` | all CPUs | Inference threads |
| `--yes` | | Skip download confirmation prompts |
| `--keep-wav` | | Keep the extracted temporary WAV |

Output files are named after the input: `input.mp4` + `-f txt` → `input.txt`.

### Managing models

```bash
vibescribe model download small       # fetch the ~466 MB Small model
vibescribe model download             # default: base
vibescribe model list                 # cached + known models
```

Models are verified to be non-empty on download and cached in
`~/.cache/vibescribe/models/` (honors `$XDG_CACHE_HOME`), so transcribing never
asks twice. A cancelled or interrupted download leaves no partial model behind.

### Example

```console
$ vibescribe transcribe -i 2026-05-30-talk.mp4 -m base -f srt,json -o out/
Extracting audio track 100%
Transcribing 100% |████████████████████████████████████████|
Transcription complete
  Language:   en
  Segments:   12
  Output:
    out/2026-05-30-talk.srt
    out/2026-05-30-talk.json
```

## How it works

```
input (mp4/mkv/mp3/…) ──ffmpeg──▶ 16 kHz mono PCM WAV ──whisper-cli──▶ segments ──▶ txt/srt/vtt/json/html
```

1. `ffmpeg` demuxes the audio track into a 16 kHz mono 16-bit WAV.
2. `whisper-cli` (with `-oj -pp`) peforms local inference using the GGML model.
3. Segments (with millisecond offsets) are parsed from whisper's JSON output
   and rendered into every requested format.

## Project layout

```
cmd/            CLI entrypoint (cobra): root, transcribe, model
pkg/audio/      ffmpeg/ffprobe wrappers + progress parsing
pkg/model/      model cache, download + verification, HEAD sizing
pkg/transcriber whisper-cli runner + JSON/segment parsing
pkg/exporter/   txt/srt/vtt/json/html renderers
```

## License

[MIT](LICENSE)
# VibeScribe CLI

[![Tests](https://github.com/azzenabidi/vibescribe-cli/actions/workflows/ci.yml/badge.svg?label=tests)](https://github.com/azzenabidi/vibescribe-cli/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/azzenabidi/vibescribe-cli?display_name=tag)](https://github.com/azzenabidi/vibescribe-cli/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/azzenabidi/vibescribe-cli)](https://goreportcard.com/report/github.com/azzenabidi/vibescribe-cli)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

VibeScribe is a local command-line transcription tool for video and audio files. It uses [ffmpeg](https://ffmpeg.org/) for audio extraction and [whisper.cpp](https://github.com/ggml-org/whisper.cpp) for inference, then exports transcripts as TXT, SRT, VTT, JSON, or HTML.

Your media stays on your machine. VibeScribe only contacts Hugging Face when you download a named model; transcription and inference are fully offline.

## Contents

- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Usage](#usage)
- [Models](#models)
- [Privacy and security](#privacy-and-security)
- [How it works](#how-it-works)
- [Development](#development)
- [Releases](#releases)
- [Contributing](#contributing)
- [Support](#support)
- [Troubleshooting](#troubleshooting)
- [Project layout](#project-layout)
- [Acknowledgments](#acknowledgments)
- [License](#license)

## Features

- Local transcription with no cloud API or API key
- Video and audio input through ffmpeg
- Whisper models from `tiny` through `large-v3`
- Automatic language detection or a manually selected language
- TXT, SRT, VTT, JSON, and HTML exports
- Reusable model cache with atomic downloads
- Configurable inference threads and progress output
- SHA-256 checksums and cross-platform release archives

## Requirements

At runtime, VibeScribe needs:

- `ffmpeg` and `ffprobe` on `PATH`
- The whisper.cpp `whisper-cli` engine on `PATH`, or a path in `VIBESCRIBE_WHISPER_CLI`
- A Whisper GGML model, either cached or supplied as a local `.bin` file

Building from source requires Go 1.25 or newer. Building `whisper-cli` from source additionally requires Git, CMake, Make, and a C/C++ toolchain.

Release archives contain only VibeScribe. They do not bundle ffmpeg, whisper.cpp, or model weights.

## Installation

### Prebuilt release

Download the archive for your operating system and architecture from [GitHub Releases](https://github.com/azzenabidi/vibescribe-cli/releases/latest), then place `vibescribe` or `vibescribe.exe` on your `PATH`.

Release archives are available for Linux, macOS, and Windows on AMD64 and ARM64.

### Build from source

```bash
git clone https://github.com/azzenabidi/vibescribe-cli.git
cd vibescribe-cli
make build
./bin/vibescribe --version
```

On Linux, `sudo make install` copies the binary to `/usr/local/bin/vibescribe`.

To build directly with Go on any supported platform:

```bash
go build -trimpath -o vibescribe ./cmd
```

### Install whisper.cpp

Use an existing `whisper-cli` build, or build the pinned v1.9.4 engine from a source checkout:

```bash
make whisper-cli
export PATH="$PWD/bin:$PATH"
```

You can also point VibeScribe to a specific binary:

```bash
export VIBESCRIBE_WHISPER_CLI=/path/to/whisper-cli
```

The engine and ffmpeg are runtime dependencies; VibeScribe does not manage or update them automatically.

## Usage

Transcribe a file with automatic language detection:

```bash
vibescribe transcribe -i input.mp4
```

Choose a model, output formats, and an output directory:

```bash
vibescribe transcribe \
  -i talk.mp4 \
  -m small \
  -f srt,json,html \
  -o transcripts
```

| Flag | Default | Description |
| --- | --- | --- |
| `-i, --input` | Required | Input video or audio path |
| `-m, --model` | `base` | Model name or path to a GGML `.bin` model |
| `-f, --format` | `txt,srt` | Comma-separated formats: `txt`, `srt`, `vtt`, `json`, `html` |
| `-o, --output-dir` | `.` | Directory for transcript files |
| `-l, --language` | `auto` | Language code such as `en`, `fr`, or `es`, or `auto` |
| `-t, --threads` | All CPUs | Number of inference threads |
| `--yes` | `false` | Skip model download confirmation |
| `--keep-wav` | `false` | Keep the extracted WAV file |

Run `vibescribe --help` or `vibescribe transcribe --help` for the complete command reference.

Output files use the input filename without its extension. For example, `talk.mp4` with `--format srt` produces `talk.srt`.

## Models

List known and cached models:

```bash
vibescribe model list
```

Download a model before transcription:

```bash
vibescribe model download small
```

Models are stored in `$XDG_CACHE_HOME/vibescribe/models` or `~/.cache/vibescribe/models`. You can also pass any local GGML model path with `--model /path/to/model.bin`.

VibeScribe downloads named models over HTTPS and rejects empty responses. It does not currently verify model checksums or signatures; use a separately verified model when that assurance is required.

## Privacy and security

- Input media and generated transcripts are processed locally.
- VibeScribe does not upload media, audio, text, or model files.
- Named model downloads contact `huggingface.co` and are cached locally.
- Release archives include a SHA-256 checksum file for integrity verification.

## How it works

```text
media ──ffmpeg──▶ 16 kHz mono PCM WAV ──whisper-cli──▶ segments ──▶ transcript formats
```

1. `ffprobe` determines the media duration for progress reporting.
2. `ffmpeg` extracts a 16 kHz mono 16-bit PCM WAV track.
3. `whisper-cli` runs local inference and emits JSON with timestamped segments.
4. VibeScribe renders the selected transcript formats.

## Development

Run the same quality gate used by CI:

```bash
make check
```

This verifies modules, checks formatting, runs `go vet`, runs tests with the race detector, and builds the CLI.

Useful commands:

```bash
make fmt
make vet
make test
make build
```

The test suite is deterministic and does not require ffmpeg, whisper-cli, model downloads, or network access.

## Releases

Push a semantic version tag such as `v0.1.0`:

```bash
git tag -s v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

The release workflow verifies the tagged commit, builds Linux, macOS, and Windows binaries, creates archives and SHA-256 checksums, and opens a draft GitHub Release for review.

## Contributing

Contributions are welcome. Open an issue before starting a substantial change, keep pull requests focused, add or update tests and documentation, and run `make check` before submitting.

## Support

Use [GitHub Issues](https://github.com/azzenabidi/vibescribe-cli/issues) for reproducible bugs and feature requests. Include the VibeScribe version, operating system, architecture, exact command, and relevant ffmpeg or whisper.cpp version. Do not attach private media or model files.

## Troubleshooting

### `ffmpeg` or `ffprobe` is missing

Install ffmpeg and ensure both commands are available on `PATH`:

```bash
ffmpeg -version
ffprobe -version
```

### `whisper-cli` was not found

Install the engine, add it to `PATH`, or set `VIBESCRIBE_WHISPER_CLI` to its full path.

### A model is missing

Run `vibescribe model list`, then download the required model with `vibescribe model download <name>`. Alternatively, pass an existing `.bin` file to `--model`.

### No usable audio was produced

Confirm that the input contains an audio stream and that `ffprobe` can inspect it. Corrupt files or video-only inputs cannot be transcribed.

## Project layout

```text
cmd/            Cobra CLI commands
pkg/audio/      ffmpeg and ffprobe integration
pkg/model/      model resolution, cache, and downloads
pkg/transcriber whisper.cpp process and output parsing
pkg/exporter/   transcript format rendering
```

## Acknowledgments

- [whisper.cpp](https://github.com/ggml-org/whisper.cpp) provides local speech recognition.
- [FFmpeg](https://ffmpeg.org/) provides media demuxing and audio extraction.
- [Hugging Face](https://huggingface.co/ggerganov/whisper.cpp) hosts the default model downloads.

## License

VibeScribe CLI is available under the [MIT License](LICENSE). whisper.cpp and downloaded Whisper models are separate third-party components with their own licenses.

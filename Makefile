BINARY      := vibescribe
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X main.version=$(VERSION)
BUILD_DIR   := bin
CGO_ENABLED ?= 0
WHISPER_REF := v1.9.4

.PHONY: build modules fmt fmt-check vet test check install whisper-cli clean

build:
	@mkdir -p "$(BUILD_DIR)"
	CGO_ENABLED=$(CGO_ENABLED) go build -trimpath -ldflags "$(LDFLAGS)" -o "$(BUILD_DIR)/$(BINARY)" ./cmd

modules:
	go mod verify
	go mod tidy -diff

fmt:
	gofmt -w cmd pkg

fmt-check:
	@files="$$(gofmt -l cmd pkg)"; \
	if [ -n "$$files" ]; then \
		printf 'Unformatted files:\n%s\n' "$$files" >&2; \
		exit 1; \
	fi

vet:
	go vet ./...

test:
	go test -race -covermode=atomic ./...

check: modules fmt-check vet test build

install: build
	install -Dm755 "$(BUILD_DIR)/$(BINARY)" /usr/local/bin/$(BINARY)

# Build the whisper.cpp inference engine (whisper-cli) into bin/.
# VibeScribe expects to find whisper-cli on PATH or via VIBESCRIBE_WHISPER_CLI.
whisper-cli:
	@test -d .build/whisper.cpp || git clone --depth 1 --branch $(WHISPER_REF) https://github.com/ggml-org/whisper.cpp.git .build/whisper.cpp
	cmake -S .build/whisper.cpp -B .build/whisper.cpp/build \
		-DCMAKE_BUILD_TYPE=Release \
		-DWHISPER_BUILD_EXAMPLES=ON -DWHISPER_BUILD_TESTS=OFF -DWHISPER_BUILD_SERVER=OFF
	cmake --build .build/whisper.cpp/build --config Release -j$$(nproc)
	install -Dm755 .build/whisper.cpp/build/bin/whisper-cli $(BUILD_DIR)/whisper-cli

clean:
	rm -rf "$(BUILD_DIR)" .build

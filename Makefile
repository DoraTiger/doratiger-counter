MAIN_ENTRY := ./cmd/doratiger-counter
NAME := doratiger-counter
MODULE := github.com/DoraTiger/$(NAME)
VERSION_PACKAGE := $(MODULE)/internal/version

BUILD_VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILD_REPO ?= https://github.com/DoraTiger/$(NAME)

LD_FLAGS := -s -w
LD_FLAGS += -X '$(VERSION_PACKAGE).BuildVersion=$(BUILD_VERSION)'
LD_FLAGS += -X '$(VERSION_PACKAGE).BuildTime=$(BUILD_TIME)'
LD_FLAGS += -X '$(VERSION_PACKAGE).BuildRepo=$(BUILD_REPO)'

BUILD_FLAGS := -mod=readonly -trimpath
CGO_ENABLED ?= 0
BUILD_DIR := build
RELEASE_DIR := release

.PHONY: all build build-all check clean fmt-check release test vet

all: check build

build:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) go build $(BUILD_FLAGS) -ldflags "$(LD_FLAGS)" -o $(BUILD_DIR)/$(NAME) $(MAIN_ENTRY)

build-all: clean
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -ldflags "$(LD_FLAGS)" -o $(BUILD_DIR)/linux-amd64/$(NAME) $(MAIN_ENTRY)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -ldflags "$(LD_FLAGS)" -o $(BUILD_DIR)/linux-arm64/$(NAME) $(MAIN_ENTRY)

clean:
	rm -rf $(BUILD_DIR) $(RELEASE_DIR)

test:
	go test -race ./...

vet:
	go vet ./...

fmt-check:
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

check: fmt-check vet test

release: build-all
	bash scripts/release.sh $(NAME) $(BUILD_DIR) $(RELEASE_DIR)

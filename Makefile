.DEFAULT_GOAL := build

.PHONY: build clean install

build_flags := -trimpath

build:
	go build $(build_flags) -o bin/ ./cmd/psqlw

clean:
	@rm -rf bin

install:
	go install $(build_flags) ./cmd/psqlw


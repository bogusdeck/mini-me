.PHONY: all build test clean deb arch

VERSION ?= 0.1.0

all: build

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o mini-me ./cmd/mini-me

test:
	go test -v ./...

deb:
	./packaging/debian/build_deb.sh

arch:
	makepkg -si

clean:
	rm -rf dist/ mini-me

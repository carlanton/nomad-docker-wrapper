VERSION = 0.1
GITHASH = $(shell git rev-parse --short HEAD)

all:
	go build .

build-in-docker:
	docker run \
		-v $(PWD):/go/src/github.com/carlanton/nomad-docker-wrapper \
		-w /go/src/github.com/carlanton/nomad-docker-wrapper \
		golang:1.7 \
		go build \
			-ldflags "-X main.version=0.1 -X main.githash=$(GITHASH)" \
			.

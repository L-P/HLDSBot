DOCKER?=sudo docker

all: hldsbot

.PHONY: hldsbot
hldsbot:
	go build

.PHONY: image
image:
	$(DOCKER) build docker -f docker/hlds.dockerfile -t hlds:latest

.PHONY: test
test:
	go test ./...

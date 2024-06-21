DOCKER?=sudo docker

all:

.PHONY: image
image:
	$(DOCKER) build docker -f docker/hlds.dockerfile -t hlds:latest

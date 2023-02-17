# Makefile for building the secret-watcher server + docker image.
SHELL=bash
DOCKER_USERNAME=cmwylie19
TAG ?= v0.0.1
ARCH ?= amd64
# The stage to build, one of: dev, test, prod
# Default to dev if not set.
# dev is compiled for amd64 architecture (Kind)
# test is compiled for arm64 architecture (Rasberry Pi)
# prod is compiled for amd64 architecture (OpenShift)


IMAGE ?= ${DOCKER_USERNAME}/demo-blog:${TAG}

#---------------------------
# Build the secret-watcher binary

.PHONY: compile
compile: $(shell find . -name '*.go')
	GOARCH=${ARCH} CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o build/demo


#---------------------------
# Build the docker image

.PHONY: build/image
build/image: build/secret-watcher
	docker build -t $(IMAGE) build/
	rm build/secret-watcher

#--------------------------------
# Push the docker image to dockerhub
.PHONY: push-image
push-image: build/image
	docker push $(IMAGE)

#--------------------------------
all: build/secret-watcher build/image push-image
	@echo "Done building secret-watcher for ${ARCH}"
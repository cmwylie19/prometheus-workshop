# Makefile for building the secret-watcher server + docker image.
SHELL=bash
DOCKER_USERNAME=cmwylie19

# The stage to build, one of: dev, test, prod
# Default to dev if not set.
# dev is compiled for amd64 architecture (Kind)
# test is compiled for arm64 architecture (Rasberry Pi)
# prod is compiled for amd64 architecture (OpenShift)

ifndef ENVIRONMENT
	export ENVIRONMENT :=prod
endif

ifeq ($(ENVIRONMENT),dev)
	export ARCH :=amd64
else ifeq ($(ENVIRONMENT),test)
	export ARCH :=arm64
else ifeq ($(ENVIRONMENT),prod)
	export ARCH :=amd64
else
	$(error STAGE must be one of: dev, test, prod)
endif


IMAGE ?= ${DOCKER_USERNAME}/secret-watcher:${ARCH}

#---------------------------
# Build the secret-watcher binary

.PHONY: build/secret-watcher
build/secret-watcher: $(shell find . -name '*.go')
	GOARCH=${ARCH} CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o $@ ./cmd/secret-watcher


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
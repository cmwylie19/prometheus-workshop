# Makefile for building the secret-watcher server + docker image.
SHELL=bash
DOCKER_USERNAME=cmwylie19
TAG ?= v0.0.1
# The stage to build, one of: dev, test, prod
# Default to dev if not set.
# dev is compiled for amd64 architecture (Kind)
# test is compiled for arm64 architecture (Rasberry Pi)
# prod is compiled for amd64 architecture (OpenShift)


IMAGE ?= ${DOCKER_USERNAME}/demo-blog:${TAG}

#---------------------------
# Build the secret-watcher binary

.PHONY: compile
compile:
	GOARCH=amd64 GOOS=linux go build -o build/demo


#---------------------------
# Build the docker image

.PHONY: build
build: 
	docker build -t $(IMAGE) build/
	rm build/demo

#--------------------------------
# Push the docker image to dockerhub
.PHONY: push
push: 
	docker push $(IMAGE)

#--------------------------------
all: compile build push
	@echo "Done building demo for ${ARCH}"
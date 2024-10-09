# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# The playground builds Go from a bootstrap version for two reasons:
# - The playground deployment is triggered before the artifacts are
#   published for the latest version of Go.
# - The sandbox builds the Go standard library with a custom build
#   flag called faketime.

# GO_VERSION is provided by Cloud Build, and is set to the latest
# version of Go. See the configuration in the deploy directory.
ARG GO_VERSION=go1.22.6

# GO_BOOTSTRAP_VERSION is downloaded below and used to bootstrap the build from
# source. Therefore, this should be a version that is guaranteed to have
# published artifacts, such as the latest minor of the previous major Go
# release.
#
# See also https://go.dev/issue/69238.
ARG GO_BOOTSTRAP_VERSION=go1.22.6

############################################################################
# Build Go at GO_VERSION, and build faketime standard library.
FROM debian:buster AS build-go
LABEL maintainer="golang-dev@googlegroups.com"

ENV BUILD_DEPS 'curl git gcc patch libc6-dev ca-certificates'
RUN apt-get update && apt-get install -y ${BUILD_DEPS} --no-install-recommends

ENV GOPATH /go
ENV GOROOT_BOOTSTRAP=/usr/local/go-bootstrap

# https://docs.docker.com/reference/dockerfile/#understand-how-arg-and-from-interact
ARG GO_VERSION
ENV GO_VERSION ${GO_VERSION}
ARG GO_BOOTSTRAP_VERSION
ENV GO_BOOTSTRAP_VERSION ${GO_BOOTSTRAP_VERSION}

# Get a bootstrap version of Go for building GO_VERSION. At the time
# of this Dockerfile being built, GO_VERSION's artifacts may not yet
# be published.
RUN curl -sSL https://dl.google.com/go/$GO_BOOTSTRAP_VERSION.linux-amd64.tar.gz -o /tmp/go.tar.gz
RUN curl -sSL https://dl.google.com/go/$GO_BOOTSTRAP_VERSION.linux-amd64.tar.gz.sha256 -o /tmp/go.tar.gz.sha256
RUN echo "$(cat /tmp/go.tar.gz.sha256) /tmp/go.tar.gz" | sha256sum -c -
RUN mkdir -p $GOROOT_BOOTSTRAP
RUN tar --strip=1 -C $GOROOT_BOOTSTRAP -vxzf /tmp/go.tar.gz

RUN mkdir /gocache
ENV GOCACHE /gocache
ENV GO111MODULE on
ENV GOPROXY=https://proxy.golang.org

# Compile Go at target version in /usr/local/go.
WORKDIR /usr/local
RUN git clone https://go.googlesource.com/go go && cd go && git reset --hard $GO_VERSION
WORKDIR /usr/local/go/src
RUN ./make.bash

############################################################################
# Build playground web server.
FROM debian:buster AS build-playground

RUN apt-get update && apt-get install -y ca-certificates git --no-install-recommends
# Build playground from Go built at GO_VERSION.
COPY --from=build-go /usr/local/go /usr/local/go
ENV GOROOT /usr/local/go
ENV GOPATH /go
ENV PATH="/go/bin:/usr/local/go/bin:${PATH}"
# Cache dependencies for efficient Dockerfile building.
COPY go.mod /go/src/playground/go.mod
COPY go.sum /go/src/playground/go.sum
WORKDIR /go/src/playground
RUN go mod download

# Add and compile playground daemon.
COPY . /go/src/playground/
RUN go install

############################################################################
# Final stage.
FROM debian:buster

RUN apt-get update && apt-get install -y git ca-certificates --no-install-recommends

# Make a copy in /usr/local/go-faketime where the standard library
# is installed with -tags=faketime.
COPY --from=build-go /usr/local/go /usr/local/go-faketime

ENV CGO_ENABLED 0
ENV GOPATH /go
ENV GOROOT /usr/local/go-faketime
ARG GO_VERSION
ENV GO_VERSION ${GO_VERSION}
ENV PATH="/go/bin:/usr/local/go-faketime/bin:${PATH}"

WORKDIR /usr/local/go-faketime
# golang/go#57495: install std to warm the build cache. We only set
# GOCACHE=/gocache here to keep it as small as possible, since it must be
# copied on every build.
RUN GOCACHE=/gocache ./bin/go install --tags=faketime std
# Ignore the exit code. go vet std does not pass vet with the faketime
# patches, but it successfully caches results for when we vet user
# snippets.
RUN ./bin/go vet --tags=faketime std || true

RUN mkdir /app
COPY --from=build-playground /go/bin/playground /app
COPY edit.html /app
COPY static /app/static
COPY examples /app/examples
WORKDIR /app

EXPOSE 8080
ENTRYPOINT ["/app/playground"]

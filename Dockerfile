# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

FROM debian:buster AS go-faketime
LABEL maintainer="golang-dev@googlegroups.com"

ENV BUILD_DEPS 'curl git gcc patch libc6-dev ca-certificates'
RUN apt-get update && apt-get install -y ${BUILD_DEPS} --no-install-recommends

ENV GOPATH /go
ENV PATH /usr/local/go/bin:$GOPATH/bin:$PATH
ENV GO_BOOTSTRAP_VERSION go1.14.1

# Get a version of Go for building the playground
RUN curl -sSL https://dl.google.com/go/$GO_BOOTSTRAP_VERSION.linux-amd64.tar.gz -o /tmp/go.tar.gz
RUN curl -sSL https://dl.google.com/go/$GO_BOOTSTRAP_VERSION.linux-amd64.tar.gz.sha256 -o /tmp/go.tar.gz.sha256
RUN echo "$(cat /tmp/go.tar.gz.sha256) /tmp/go.tar.gz" | sha256sum -c -
RUN mkdir -p /usr/local/go
RUN tar --strip=1 -C /usr/local/go -vxzf /tmp/go.tar.gz

RUN mkdir /gocache
ENV GOCACHE /gocache
ENV GO111MODULE on
ENV GOPROXY=https://proxy.golang.org

# Compile Go at target sandbox version and install standard library with --tags=faketime.
WORKDIR /usr/local
# Don’t use the cached checkout if the HEAD commit is different.
ADD https://api.github.com/repos/golang/playground/git/refs/heads/dev.go2go version.json
RUN git clone https://go.googlesource.com/go go-faketime && cd go-faketime && git checkout dev.go2go
WORKDIR /usr/local/go-faketime/src
RUN ./make.bash
ENV GOROOT /usr/local/go-faketime
RUN ../bin/go install --tags=faketime std

FROM golang:1.14 as build-playground

# Compile Go using dev.go2go branch to get go2go-compatible go/format.
WORKDIR /usr/local
RUN git clone https://go.googlesource.com/go go2go && cd go2go && git checkout dev.go2go
WORKDIR /usr/local/go2go/src
RUN ./make.bash
ENV GOROOT /usr/local/go2go

WORKDIR /

ENV PATH /usr/local/go2go/bin:$PATH

COPY go.mod /go/src/playground/go.mod
COPY go.sum /go/src/playground/go.sum
WORKDIR /go/src/playground
RUN go mod download

# Add and compile playground daemon
COPY . /go/src/playground/
RUN go install

############################################################################
# Final stage.
FROM debian:buster

RUN apt-get update && apt-get install -y git ca-certificates --no-install-recommends

COPY --from=go-faketime /usr/local/go-faketime /usr/local/go-faketime

ENV GOPATH /go
ENV PATH /usr/local/go-faketime/bin:$GOPATH/bin:$PATH

RUN mkdir /app

COPY --from=build-playground /go/bin/playground /app
COPY edit.html /app
COPY static /app/static
COPY examples /app/examples
WORKDIR /app

# Whether we allow third-party imports via proxy.golang.org:
ENV ALLOW_PLAY_MODULE_DOWNLOADS true

EXPOSE 8080
ENTRYPOINT ["/app/playground"]

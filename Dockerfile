# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

ARG GO_VERSION=go1.14.1

FROM debian:buster AS go-faketime
LABEL maintainer="golang-dev@googlegroups.com"

ENV BUILD_DEPS 'curl git gcc patch libc6-dev ca-certificates'
RUN apt-get update && apt-get install -y ${BUILD_DEPS} --no-install-recommends

ENV GOPATH /go
ENV PATH /usr/local/go/bin:$GOPATH/bin:$PATH
ENV GO_BOOTSTRAP_VERSION go1.14.1
ARG GO_VERSION
ENV GO_VERSION ${GO_VERSION}

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
RUN git clone https://go.googlesource.com/go go-faketime && cd go-faketime && git reset --hard $GO_VERSION
WORKDIR /usr/local/go-faketime/src
RUN ./make.bash
ENV GOROOT /usr/local/go-faketime
RUN ../bin/go install --tags=faketime std

FROM golang:1.14 as build-playground

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

ARG GO_VERSION
ENV GO_VERSION ${GO_VERSION}
ENV GOPATH /go
ENV PATH /usr/local/go-faketime/bin:$GOPATH/bin:$PATH

# Add and compile tour packages
RUN go get \
    golang.org/x/tour/pic \
    golang.org/x/tour/reader \
    golang.org/x/tour/tree \
    golang.org/x/tour/wc \
    golang.org/x/talks/content/2016/applicative/google && \
    rm -rf $GOPATH/src/golang.org/x/tour/.git && \
    rm -rf $GOPATH/src/golang.org/x/talks/.git

# Add tour packages under their old import paths (so old snippets still work)
RUN mkdir -p $GOPATH/src/code.google.com/p/go-tour && \
    cp -R $GOPATH/src/golang.org/x/tour/* $GOPATH/src/code.google.com/p/go-tour/ && \
    sed -i 's_// import_// public import_' $(find $GOPATH/src/code.google.com/p/go-tour/ -name *.go) && \
    go install \
    code.google.com/p/go-tour/pic \
    code.google.com/p/go-tour/reader \
    code.google.com/p/go-tour/tree \
    code.google.com/p/go-tour/wc

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

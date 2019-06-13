# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

FROM debian:stretch AS nacl

RUN apt-get update && apt-get install -y --no-install-recommends curl bzip2 ca-certificates

RUN curl -s https://storage.googleapis.com/nativeclient-mirror/nacl/nacl_sdk/trunk.544461/naclsdk_linux.tar.bz2 | tar -xj -C /tmp --strip-components=2 pepper_67/tools/sel_ldr_x86_64

FROM debian:stretch AS build
LABEL maintainer="golang-dev@googlegroups.com"

ENV BUILD_DEPS 'curl git gcc patch libc6-dev ca-certificates'
RUN apt-get update && apt-get install -y ${BUILD_DEPS} --no-install-recommends

ENV GOPATH /go
ENV PATH /usr/local/go/bin:$GOPATH/bin:$PATH
ENV GOROOT_BOOTSTRAP /usr/local/gobootstrap
ARG GO_VERSION=go1.12.6
ENV GO_VERSION ${GO_VERSION}

# Fake time
COPY enable-fake-time.patch /usr/local/playground/
# Fake file system
COPY fake_fs.lst /usr/local/playground/

# Get the Go binary.
RUN curl -sSL https://dl.google.com/go/$GO_VERSION.linux-amd64.tar.gz -o /tmp/go.tar.gz
RUN curl -sSL https://dl.google.com/go/$GO_VERSION.linux-amd64.tar.gz.sha256 -o /tmp/go.tar.gz.sha256
RUN echo "$(cat /tmp/go.tar.gz.sha256) /tmp/go.tar.gz" | sha256sum -c -
RUN tar -C /usr/local/ -vxzf /tmp/go.tar.gz
# Make a copy for GOROOT_BOOTSTRAP, because we rebuild the toolchain and make.bash removes bin/go as its first step.
RUN cp -R /usr/local/go $GOROOT_BOOTSTRAP
# Apply the fake time and fake filesystem patches.
RUN patch /usr/local/go/src/runtime/rt0_nacl_amd64p32.s /usr/local/playground/enable-fake-time.patch
RUN cd /usr/local/go && go run misc/nacl/mkzip.go -p syscall /usr/local/playground/fake_fs.lst src/syscall/fstest_nacl.go
# Re-build the Go toolchain.
RUN cd /usr/local/go/src && GOOS=nacl GOARCH=amd64p32 ./make.bash --no-clean

RUN mkdir /gocache
ENV GOCACHE /gocache
ENV GO111MODULE on

COPY go.mod /go/src/playground/go.mod
COPY go.sum /go/src/playground/go.sum
WORKDIR /go/src/playground

# Pre-build some packages to speed final install later.
RUN go install cloud.google.com/go/compute/metadata
RUN go install cloud.google.com/go/datastore
RUN go install github.com/bradfitz/gomemcache/memcache
RUN go install golang.org/x/tools/godoc/static
RUN go install golang.org/x/tools/imports
RUN go install github.com/rogpeppe/go-internal/modfile
RUN go install github.com/rogpeppe/go-internal/txtar

# Add and compile playground daemon
COPY . /go/src/playground/
WORKDIR /go/src/playground
RUN go install

FROM debian:stretch

RUN apt-get update && apt-get install -y git ca-certificates --no-install-recommends

COPY --from=build /usr/local/go /usr/local/go
COPY --from=nacl /tmp/sel_ldr_x86_64 /usr/local/bin

ENV GOPATH /go
ENV PATH /usr/local/go/bin:$GOPATH/bin:$PATH

# Add and compile tour packages
RUN GOOS=nacl GOARCH=amd64p32 go get \
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

COPY --from=build /go/bin/playground /app
COPY edit.html /app
COPY static /app/static
WORKDIR /app

# Run tests
RUN /app/playground test

# Whether we allow third-party imports via proxy.golang.org:
ENV ALLOW_PLAY_MODULE_DOWNLOADS true

EXPOSE 8080
ENTRYPOINT ["/app/playground"]

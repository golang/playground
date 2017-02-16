# Copyright 2014 The Go Authors.  All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

FROM debian:jessie

ENV GOPATH /go
ENV PATH /usr/local/go/bin:$GOPATH/bin:$PATH
ENV GOROOT_BOOTSTRAP /usr/local/gobootstrap
ENV GO_VERSION 1.8

# Fake time
COPY enable-fake-time.patch /usr/local/sandbox/
# Fake file system
COPY fake_fs.lst /usr/local/sandbox/

RUN set -x && buildDeps='curl ca-certificates bzip2'; \
    apt-get update && apt-get install -y $buildDeps --no-install-recommends && rm -rf /var/lib/apt/lists/* && \
    curl -s https://storage.googleapis.com/nativeclient-mirror/nacl/nacl_sdk/44.0.2403.157/naclsdk_linux.tar.bz2 | tar -xj -C /usr/local/bin --strip-components=2 pepper_44/tools/sel_ldr_x86_64 && \
    apt-get purge -y --auto-remove $buildDeps

RUN set -x && buildDeps='curl ca-certificates gcc patch libc6-dev'; \
    apt-get update && apt-get install -y $buildDeps --no-install-recommends && rm -rf /var/lib/apt/lists/* && \
# Get the Go binary.
    curl -sSL https://storage.googleapis.com/golang/go$GO_VERSION.linux-amd64.tar.gz -o /tmp/go.tar.gz && \
    curl -sSL https://storage.googleapis.com/golang/go$GO_VERSION.linux-amd64.tar.gz.sha256 -o /tmp/go.tar.gz.sha256 && \
    echo "$(cat /tmp/go.tar.gz.sha256)  /tmp/go.tar.gz" | sha256sum -c - && \
    tar -C /usr/local/ -vxzf /tmp/go.tar.gz && \
    rm /tmp/go.tar.gz /tmp/go.tar.gz.sha256 && \

# Make a copy for GOROOT_BOOTSTRAP, because we rebuild the toolchain and make.bash removes bin/go as its first step.
    cp -R /usr/local/go $GOROOT_BOOTSTRAP && \

# Apply the fake time and fake filesystem patches.
    patch /usr/local/go/src/runtime/rt0_nacl_amd64p32.s /usr/local/sandbox/enable-fake-time.patch && \
    cd /usr/local/go && go run misc/nacl/mkzip.go -p syscall /usr/local/sandbox/fake_fs.lst src/syscall/fstest_nacl.go && \

# Re-build the Go toolchain.
    cd /usr/local/go/src && GOOS=nacl GOARCH=amd64p32 ./make.bash --no-clean && \

# Clean up.
    rm -rf $GOROOT_BOOTSTRAP && \
    apt-get purge -y --auto-remove $buildDeps

# Add and compile tour packages
RUN set -x && buildDeps='git ca-certificates'; \
    apt-get update && apt-get install -y $buildDeps --no-install-recommends && rm -rf /var/lib/apt/lists/* && \
    GOOS=nacl GOARCH=amd64p32 go get \
    golang.org/x/tour/pic \
    golang.org/x/tour/reader \
    golang.org/x/tour/tree \
    golang.org/x/tour/wc \
    golang.org/x/talks/2016/applicative/google && \
    rm -rf $GOPATH/src/golang.org/x/tour/.git && \
    rm -rf $GOPATH/src/golang.org/x/talks/.git && \
    apt-get purge -y --auto-remove $buildDeps

# Add tour packages under their old import paths (so old snippets still work)
RUN mkdir -p $GOPATH/src/code.google.com/p/go-tour && \
	cp -R $GOPATH/src/golang.org/x/tour/* $GOPATH/src/code.google.com/p/go-tour/ && \
	sed -i 's_// import_// public import_' $(find $GOPATH/src/code.google.com/p/go-tour/ -name *.go) && \
	go install \
		code.google.com/p/go-tour/pic \
		code.google.com/p/go-tour/reader \
		code.google.com/p/go-tour/tree \
		code.google.com/p/go-tour/wc

# Add and compile sandbox daemon
COPY . /go/src/sandbox/
RUN go install sandbox

# Run tests
RUN /go/bin/sandbox test

EXPOSE 8080
ENTRYPOINT ["/go/bin/sandbox"]

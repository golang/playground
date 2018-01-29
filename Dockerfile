# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
FROM debian:jessie
LABEL maintainer "golang-dev@googlegroups.com"

ENV GOPATH /go
ENV PATH /usr/local/go/bin:$GOPATH/bin:$PATH
ENV GOROOT_BOOTSTRAP /usr/local/gobootstrap
ENV GO_VERSION 1.9.3
ENV DEPS 'ca-certificates'
ENV BUILD_DEPS 'curl bzip2 git gcc patch libc6-dev'

# Fake time
COPY enable-fake-time.patch /usr/local/playground/
# Fake file system
COPY fake_fs.lst /usr/local/playground/

RUN set -x && \
    apt-get update && apt-get install -y ${BUILD_DEPS} ${DEPS} --no-install-recommends && rm -rf /var/lib/apt/lists/*

RUN curl -s https://storage.googleapis.com/nativeclient-mirror/nacl/nacl_sdk/49.0.2623.87/naclsdk_linux.tar.bz2 | tar -xj -C /usr/local/bin --strip-components=2 pepper_49/tools/sel_ldr_x86_64

# Get the Go binary.
RUN curl -sSL https://dl.google.com/go/go$GO_VERSION.linux-amd64.tar.gz -o /tmp/go.tar.gz && \
    curl -sSL https://dl.google.com/go/go$GO_VERSION.linux-amd64.tar.gz.sha256 -o /tmp/go.tar.gz.sha256 && \
    echo "$(cat /tmp/go.tar.gz.sha256) /tmp/go.tar.gz" | sha256sum -c - && \
    tar -C /usr/local/ -vxzf /tmp/go.tar.gz && \
    rm /tmp/go.tar.gz /tmp/go.tar.gz.sha256 && \
    # Make a copy for GOROOT_BOOTSTRAP, because we rebuild the toolchain and make.bash removes bin/go as its first step.
    cp -R /usr/local/go $GOROOT_BOOTSTRAP && \
    # Apply the fake time and fake filesystem patches.
    patch /usr/local/go/src/runtime/rt0_nacl_amd64p32.s /usr/local/playground/enable-fake-time.patch && \
    cd /usr/local/go && go run misc/nacl/mkzip.go -p syscall /usr/local/playground/fake_fs.lst src/syscall/fstest_nacl.go && \
    # Re-build the Go toolchain.
    cd /usr/local/go/src && GOOS=nacl GOARCH=amd64p32 ./make.bash --no-clean && \
    # Clean up.
    rm -rf $GOROOT_BOOTSTRAP

# Add and compile tour packages
RUN GOOS=nacl GOARCH=amd64p32 go get \
    golang.org/x/tour/pic \
    golang.org/x/tour/reader \
    golang.org/x/tour/tree \
    golang.org/x/tour/wc \
    golang.org/x/talks/2016/applicative/google && \
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

# BEGIN deps (run `make update-deps` to update)

# Repo cloud.google.com/go at 3051b91 (2017-12-06)
ENV REV=3051b919da3b8d62bc3a57ab4b353ca1c72402d5
RUN go get -d cloud.google.com/go/compute/metadata `#and 6 other pkgs` &&\
    (cd /go/src/cloud.google.com/go && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo github.com/bradfitz/gomemcache at 1952afa (2017-02-08)
ENV REV=1952afaa557dc08e8e0d89eafab110fb501c1a2b
RUN go get -d github.com/bradfitz/gomemcache/memcache &&\
    (cd /go/src/github.com/bradfitz/gomemcache && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo github.com/golang/protobuf at 1e59b77 (2017-11-13)
ENV REV=1e59b77b52bf8e4b449a57e6f79f21226d571845
RUN go get -d github.com/golang/protobuf/proto `#and 8 other pkgs` &&\
    (cd /go/src/github.com/golang/protobuf && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo github.com/googleapis/gax-go at 317e000 (2017-09-15)
ENV REV=317e0006254c44a0ac427cc52a0e083ff0b9622f
RUN go get -d github.com/googleapis/gax-go &&\
    (cd /go/src/github.com/googleapis/gax-go && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo golang.org/x/net at d866cfc (2017-12-12)
ENV REV=d866cfc389cec985d6fda2859936a575a55a3ab6
RUN go get -d golang.org/x/net/context `#and 8 other pkgs` &&\
    (cd /go/src/golang.org/x/net && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo golang.org/x/oauth2 at 876b1c6 (2018-01-03)
ENV REV=876b1c6ee618a9f8fa31ded3b27708d44b3153af
RUN go get -d golang.org/x/oauth2 `#and 5 other pkgs` &&\
    (cd /go/src/golang.org/x/oauth2 && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo golang.org/x/text at be25de4 (2017-12-07)
ENV REV=be25de41fadfae372d6470bda81ca6beb55ef551
RUN go get -d golang.org/x/text/secure/bidirule `#and 4 other pkgs` &&\
    (cd /go/src/golang.org/x/text && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo golang.org/x/tools at b790d0b (2018-01-05)
ENV REV=b790d0ba0332a621d0b58cfd69fa13bd3dc358d2
RUN go get -d golang.org/x/tools/go/ast/astutil `#and 3 other pkgs` &&\
    (cd /go/src/golang.org/x/tools && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo google.golang.org/api at 9a048ca (2017-12-07)
ENV REV=9a048cac3675aa589c62a35d7d42b25451dd15f1
RUN go get -d google.golang.org/api/googleapi `#and 6 other pkgs` &&\
    (cd /go/src/google.golang.org/api && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo google.golang.org/genproto at 7f0da29 (2017-11-23)
ENV REV=7f0da29060c682909f650ad8ed4e515bd74fa12a
RUN go get -d google.golang.org/genproto/googleapis/api/annotations `#and 4 other pkgs` &&\
    (cd /go/src/google.golang.org/genproto && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo google.golang.org/grpc at b8191e5 (2017-12-06)
ENV REV=b8191e57b23de650278db4d23bf596219e5f3665
RUN go get -d google.golang.org/grpc `#and 24 other pkgs` &&\
    (cd /go/src/google.golang.org/grpc && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Optimization to speed up iterative development, not necessary for correctness:
RUN go install cloud.google.com/go/compute/metadata \
	cloud.google.com/go/datastore \
	cloud.google.com/go/internal \
	cloud.google.com/go/internal/atomiccache \
	cloud.google.com/go/internal/fields \
	cloud.google.com/go/internal/version \
	github.com/bradfitz/gomemcache/memcache \
	github.com/golang/protobuf/proto \
	github.com/golang/protobuf/protoc-gen-go/descriptor \
	github.com/golang/protobuf/ptypes \
	github.com/golang/protobuf/ptypes/any \
	github.com/golang/protobuf/ptypes/duration \
	github.com/golang/protobuf/ptypes/struct \
	github.com/golang/protobuf/ptypes/timestamp \
	github.com/golang/protobuf/ptypes/wrappers \
	github.com/googleapis/gax-go \
	golang.org/x/net/context \
	golang.org/x/net/context/ctxhttp \
	golang.org/x/net/http2 \
	golang.org/x/net/http2/hpack \
	golang.org/x/net/idna \
	golang.org/x/net/internal/timeseries \
	golang.org/x/net/lex/httplex \
	golang.org/x/net/trace \
	golang.org/x/oauth2 \
	golang.org/x/oauth2/google \
	golang.org/x/oauth2/internal \
	golang.org/x/oauth2/jws \
	golang.org/x/oauth2/jwt \
	golang.org/x/text/secure/bidirule \
	golang.org/x/text/transform \
	golang.org/x/text/unicode/bidi \
	golang.org/x/text/unicode/norm \
	golang.org/x/tools/go/ast/astutil \
	golang.org/x/tools/godoc/static \
	golang.org/x/tools/imports \
	google.golang.org/api/googleapi \
	google.golang.org/api/googleapi/internal/uritemplates \
	google.golang.org/api/internal \
	google.golang.org/api/iterator \
	google.golang.org/api/option \
	google.golang.org/api/transport/grpc \
	google.golang.org/genproto/googleapis/api/annotations \
	google.golang.org/genproto/googleapis/datastore/v1 \
	google.golang.org/genproto/googleapis/rpc/status \
	google.golang.org/genproto/googleapis/type/latlng \
	google.golang.org/grpc \
	google.golang.org/grpc/balancer \
	google.golang.org/grpc/balancer/base \
	google.golang.org/grpc/balancer/roundrobin \
	google.golang.org/grpc/codes \
	google.golang.org/grpc/connectivity \
	google.golang.org/grpc/credentials \
	google.golang.org/grpc/credentials/oauth \
	google.golang.org/grpc/encoding \
	google.golang.org/grpc/grpclb/grpc_lb_v1/messages \
	google.golang.org/grpc/grpclog \
	google.golang.org/grpc/internal \
	google.golang.org/grpc/keepalive \
	google.golang.org/grpc/metadata \
	google.golang.org/grpc/naming \
	google.golang.org/grpc/peer \
	google.golang.org/grpc/resolver \
	google.golang.org/grpc/resolver/dns \
	google.golang.org/grpc/resolver/manual \
	google.golang.org/grpc/resolver/passthrough \
	google.golang.org/grpc/stats \
	google.golang.org/grpc/status \
	google.golang.org/grpc/tap \
	google.golang.org/grpc/transport
# END deps

RUN apt-get purge -y --auto-remove ${BUILD_DEPS}

# Add and compile playground daemon
COPY . /go/src/playground/
RUN go install playground

RUN mkdir /app

COPY edit.html /app
COPY static /app/static

WORKDIR /app

# Run tests
RUN /go/bin/playground test

EXPOSE 8080
ENTRYPOINT ["/go/bin/playground"]

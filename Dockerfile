# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
FROM debian:jessie as builder
LABEL maintainer "golang-dev@googlegroups.com"

ENV GOPATH /go
ENV PATH /usr/local/go/bin:$GOPATH/bin:$PATH
ENV GOROOT_BOOTSTRAP /usr/local/gobootstrap
ENV CGO_ENABLED=0
ENV GO_VERSION 1.11.1
ENV BUILD_DEPS 'curl bzip2 git gcc patch libc6-dev ca-certificates'

# Fake time
COPY enable-fake-time.patch /usr/local/playground/
# Fake file system
COPY fake_fs.lst /usr/local/playground/

RUN apt-get update && apt-get install -y ${BUILD_DEPS} --no-install-recommends

RUN curl -s https://storage.googleapis.com/nativeclient-mirror/nacl/nacl_sdk/trunk.544461/naclsdk_linux.tar.bz2 | tar -xj -C /tmp --strip-components=2 pepper_67/tools/sel_ldr_x86_64

# Get the Go binary.
RUN curl -sSL https://dl.google.com/go/go$GO_VERSION.linux-amd64.tar.gz -o /tmp/go.tar.gz
RUN curl -sSL https://dl.google.com/go/go$GO_VERSION.linux-amd64.tar.gz.sha256 -o /tmp/go.tar.gz.sha256
RUN echo "$(cat /tmp/go.tar.gz.sha256) /tmp/go.tar.gz" | sha256sum -c -
RUN tar -C /usr/local/ -vxzf /tmp/go.tar.gz
# Make a copy for GOROOT_BOOTSTRAP, because we rebuild the toolchain and make.bash removes bin/go as its first step.
RUN cp -R /usr/local/go $GOROOT_BOOTSTRAP
# Apply the fake time and fake filesystem patches.
RUN patch /usr/local/go/src/runtime/rt0_nacl_amd64p32.s /usr/local/playground/enable-fake-time.patch
RUN cd /usr/local/go && go run misc/nacl/mkzip.go -p syscall /usr/local/playground/fake_fs.lst src/syscall/fstest_nacl.go
# Re-build the Go toolchain.
RUN cd /usr/local/go/src && GOOS=nacl GOARCH=amd64p32 ./make.bash --no-clean

# BEGIN deps (run `make update-deps` to update)

# Repo cloud.google.com/go at 3afaae4 (2018-03-02)
ENV REV=3afaae429987a1884530d6018d6e963a932d28c0
RUN go get -d cloud.google.com/go/compute/metadata `#and 6 other pkgs` &&\
    (cd /go/src/cloud.google.com/go && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo github.com/bradfitz/gomemcache at 1952afa (2017-02-08)
ENV REV=1952afaa557dc08e8e0d89eafab110fb501c1a2b
RUN go get -d github.com/bradfitz/gomemcache/memcache &&\
    (cd /go/src/github.com/bradfitz/gomemcache && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo github.com/golang/protobuf at bbd03ef (2018-02-02)
ENV REV=bbd03ef6da3a115852eaf24c8a1c46aeb39aa175
RUN go get -d github.com/golang/protobuf/proto `#and 8 other pkgs` &&\
    (cd /go/src/github.com/golang/protobuf && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo github.com/googleapis/gax-go at 317e000 (2017-09-15)
ENV REV=317e0006254c44a0ac427cc52a0e083ff0b9622f
RUN go get -d github.com/googleapis/gax-go &&\
    (cd /go/src/github.com/googleapis/gax-go && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo golang.org/x/net at ae89d30 (2018-03-11)
ENV REV=ae89d30ce0c63142b652837da33d782e2b0a9b25
RUN go get -d golang.org/x/net/context `#and 8 other pkgs` &&\
    (cd /go/src/golang.org/x/net && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo golang.org/x/oauth2 at 2f32c3a (2018-02-28)
ENV REV=2f32c3ac0fa4fb807a0fcefb0b6f2468a0d99bd0
RUN go get -d golang.org/x/oauth2 `#and 5 other pkgs` &&\
    (cd /go/src/golang.org/x/oauth2 && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo golang.org/x/text at b7ef84a (2018-03-02)
ENV REV=b7ef84aaf62aa3e70962625c80a571ae7c17cb40
RUN go get -d golang.org/x/text/secure/bidirule `#and 4 other pkgs` &&\
    (cd /go/src/golang.org/x/text && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo golang.org/x/tools at 59602fd (2018-10-04)
ENV REV=59602fdee893255faef9b2cd3aa8f880ea85265c
RUN go get -d golang.org/x/tools/go/ast/astutil `#and 3 other pkgs` &&\
    (cd /go/src/golang.org/x/tools && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo google.golang.org/api at ab90adb (2018-02-22)
ENV REV=ab90adb3efa287b869ecb698db42f923cc734972
RUN go get -d google.golang.org/api/googleapi `#and 6 other pkgs` &&\
    (cd /go/src/google.golang.org/api && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo google.golang.org/genproto at 2c5e7ac (2018-03-02)
ENV REV=2c5e7ac708aaa719366570dd82bda44541ca2a63
RUN go get -d google.golang.org/genproto/googleapis/api/annotations `#and 4 other pkgs` &&\
    (cd /go/src/google.golang.org/genproto && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo google.golang.org/grpc at f0a1202 (2018-02-28)
ENV REV=f0a1202acdc5c4702be05098d5ff8e9b3b444442
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
	google.golang.org/grpc/encoding/proto \
	google.golang.org/grpc/grpclb/grpc_lb_v1/messages \
	google.golang.org/grpc/grpclog \
	google.golang.org/grpc/internal \
	google.golang.org/grpc/keepalive \
	google.golang.org/grpc/metadata \
	google.golang.org/grpc/naming \
	google.golang.org/grpc/peer \
	google.golang.org/grpc/resolver \
	google.golang.org/grpc/resolver/dns \
	google.golang.org/grpc/resolver/passthrough \
	google.golang.org/grpc/stats \
	google.golang.org/grpc/status \
	google.golang.org/grpc/tap \
	google.golang.org/grpc/transport
# END deps

# Add and compile playground daemon
COPY . /go/src/playground/
RUN go install playground

FROM debian:jessie

RUN apt-get update && apt-get install -y git ca-certificates --no-install-recommends

COPY --from=builder /usr/local/go /usr/local/go
COPY --from=builder /tmp/sel_ldr_x86_64 /usr/local/bin

ENV GOPATH /go
ENV PATH /usr/local/go/bin:$GOPATH/bin:$PATH

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

RUN mkdir /app

COPY --from=builder /go/bin/playground /app
COPY edit.html /app
COPY static /app/static
WORKDIR /app

# Run tests
RUN /app/playground test

EXPOSE 8080
ENTRYPOINT ["/app/playground"]

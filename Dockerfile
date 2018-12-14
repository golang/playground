# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
FROM golang:1.11.3
LABEL maintainer "golang-dev@googlegroups.com"

ENV GOPATH /go
ENV PATH /usr/local/go/bin:$GOPATH/bin:$PATH
ENV GO_VERSION 1.11.3
ENV BUILD_DEPS 'curl git patch libc6-dev bzip2 ca-certificates'

RUN apt-get update && apt-get install -y ${BUILD_DEPS} --no-install-recommends

# #Fake time
# COPY enable-fake-time.patch /usr/local/playground/
# COPY strict-time.patch /usr/local/playground/
# # Fake file system
# COPY fake_fs.lst /usr/local/playground/
# 
# # Apply the fake time and fake filesystem patches.
# RUN patch -f -u /usr/local/go/src/runtime/rt0_nacl_amd64p32.s /usr/local/playground/enable-fake-time.patch
# RUN patch -f -u -p1 -d /usr/local/go </usr/local/playground/strict-time.patch
# RUN cd /usr/local/go && go run misc/nacl/mkzip.go -p syscall /usr/local/playground/fake_fs.lst src/syscall/fstest_nacl.go


RUN curl -s https://storage.googleapis.com/nativeclient-mirror/nacl/nacl_sdk/trunk.544461/naclsdk_linux.tar.bz2 | tar -xj -C /tmp --strip-components=2 pepper_67/tools/sel_ldr_x86_64
RUN cp /tmp/sel_ldr_x86_64 /usr/local/bin

ENV DEP_VERSION 0.5.0
RUN curl -L -s https://github.com/golang/dep/releases/download/v${DEP_VERSION}/dep-linux-amd64 -o $GOPATH/bin/dep \
  && chmod +x $GOPATH/bin/dep

# Set up application
ENV APP_ROOT $GOPATH/src/github.com/rerost/playground
RUN ln -s $APP_ROOT/ /app
WORKDIR /app

# Install dep
COPY Gopkg.toml ${APP_ROOT}/
COPY Gopkg.lock ${APP_ROOT}/

RUN dep ensure -v -vendor-only

# Add and compile playground daemon
COPY . ${APP_ROOT}/
RUN go install

COPY edit.html /app
COPY static /app/static

# Run tests
# RUN playground test

EXPOSE 8080
ENTRYPOINT ["playground"]

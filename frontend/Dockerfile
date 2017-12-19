# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
FROM golang:1.9.2
LABEL maintainer "golang-dev@googlegroups.com"

# BEGIN deps (run `make update-deps` to update)

# Repo github.com/golang/protobuf at 1e59b77 (2017-11-13)
ENV REV=1e59b77b52bf8e4b449a57e6f79f21226d571845
RUN go get -d github.com/golang/protobuf/proto &&\
    (cd /go/src/github.com/golang/protobuf && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo golang.org/x/net at faacc1b (2017-12-07)
ENV REV=faacc1b5e36e3ff02cbec9661c69ac63dd5a83ad
RUN go get -d golang.org/x/net/context &&\
    (cd /go/src/golang.org/x/net && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo golang.org/x/tools at 5d8e38b (2017-12-10)
ENV REV=5d8e38b9550d35d893ff8276756b4118ec1bf360
RUN go get -d golang.org/x/tools/go/ast/astutil `#and 3 other pkgs` &&\
    (cd /go/src/golang.org/x/tools && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo google.golang.org/appengine at 24e4144 (2017-09-21)
ENV REV=24e4144ec923c2374f6b06610c0df16a9222c3d9
RUN go get -d google.golang.org/appengine `#and 10 other pkgs` &&\
    (cd /go/src/google.golang.org/appengine && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Optimization to speed up iterative development, not necessary for correctness:
RUN go install github.com/golang/protobuf/proto \
	golang.org/x/net/context \
	golang.org/x/tools/go/ast/astutil \
	golang.org/x/tools/godoc/static \
	golang.org/x/tools/imports \
	google.golang.org/appengine \
	google.golang.org/appengine/datastore \
	google.golang.org/appengine/internal \
	google.golang.org/appengine/internal/app_identity \
	google.golang.org/appengine/internal/base \
	google.golang.org/appengine/internal/datastore \
	google.golang.org/appengine/internal/log \
	google.golang.org/appengine/internal/modules \
	google.golang.org/appengine/internal/remote_api \
	google.golang.org/appengine/log
# END deps

# Add and compile frontend daemon
COPY . /go/src/frontend/
RUN go install frontend

RUN mkdir /app

COPY edit.html /app
COPY static /app/static

WORKDIR /app

EXPOSE 8080
ENTRYPOINT ["/go/bin/frontend"]

# Copyright 2026 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Build from the repository root:
#	docker build -f snippetstore/Dockerfile .

FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /snippetstore ./snippetstore

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /snippetstore /snippetstore
ENTRYPOINT ["/snippetstore"]

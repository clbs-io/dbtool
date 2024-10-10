# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder
ARG TARGETOS TARGETARCH
ARG VERSION=v0.0.0

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a -o dbtool -ldflags="-X 'main.Version=$VERSION'" ./cmd/dbtool

FROM alpine:latest AS dbtool

RUN apk --no-cache add ca-certificates

COPY --from=builder /build/dbtool /usr/local/bin/dbtool

CMD [ "ash" ]

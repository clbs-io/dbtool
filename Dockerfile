# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.25.7-alpine AS builder
ARG TARGETOS TARGETARCH
ARG VERSION=v0.0.0

WORKDIR /build

RUN apk update && apk add --no-cache git ca-certificates tzdata && update-ca-certificates

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a -o dbtool -ldflags="-X 'main.Version=$VERSION'" ./cmd/dbtool

FROM scratch AS dbtool

WORKDIR /srv

COPY --from=builder /build/dbtool /usr/local/bin/dbtool
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
USER 1001

CMD [ "/usr/local/bin/dbtool" ]

# syntax=docker/dockerfile:1

# renovate: datasource=docker depName=anchore/syft
FROM --platform=$BUILDPLATFORM anchore/syft:v1.49.0 AS syft

FROM --platform=$BUILDPLATFORM golang:1.26.4-alpine AS builder
ARG TARGETOS TARGETARCH
ARG VERSION=v0.0.0

WORKDIR /build

RUN apk update && apk add --no-cache git ca-certificates tzdata && update-ca-certificates

COPY --from=syft /syft /usr/local/bin/syft

COPY go.mod go.sum ./

RUN go mod download

COPY . .

# Scanned in the builder, because licence text is read from the Go module cache, which is
# absent from the final image and is not recorded in the binary. A cache mount for
# /go/pkg/mod would empty that cache and reduce licence coverage to near zero while still
# emitting a valid-looking SBOM.
# Scanned before the build so only the go.mod cataloger runs: a compiled binary in the
# context makes syft also run the binary cataloger, which re-lists every module without
# licence data and duplicates the whole component set.
# Only the .spdx.json suffix is auto-detected by Trivy and Grype; the CycloneDX copy keeps
# an inert name so scanners do not count every component twice.
RUN syft scan . -q --exclude "**/.*/**" \
      -o "spdx-json=/build/sbom/dbtool.spdx.json" \
      -o "cyclonedx-json=/build/sbom/dbtool.cyclonedx-json"

RUN GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0 go build -a -o dbtool -ldflags="-X 'main.Version=$VERSION'" ./cmd/dbtool

FROM scratch AS dbtool

WORKDIR /srv

COPY --from=builder /build/dbtool /usr/local/bin/dbtool
COPY --from=builder /build/sbom /app/sbom
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
USER 1001

CMD [ "/usr/local/bin/dbtool" ]

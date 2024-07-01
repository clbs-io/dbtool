FROM golang:1.22-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o dbtool ./cmd/dbtool

FROM alpine:3.19 AS runtime

RUN apk add --no-cache ca-certificates

COPY --from=builder /build/dbtool /usr/local/bin/dbtool

CMD [ "ash" ]

FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git gcc musl-dev

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN go build -o benchmark .

# ──────────────────────────────────────────────
FROM alpine:latest

RUN apk add --no-cache ca-certificates bash

WORKDIR /app
COPY --from=builder /build/benchmark .

# Results are written here; mount a host directory to retrieve them.
RUN mkdir -p /results

CMD ["/app/benchmark"]

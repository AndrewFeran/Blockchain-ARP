FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

WORKDIR /build

# Copy Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY *.go ./

# Build the router binary
RUN go build -o router-monitor .

# ============================================
# Final stage
# ============================================
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    tcpdump \
    libpcap \
    iproute2 \
    iputils \
    bash \
    ca-certificates

WORKDIR /app

# Copy built binary
COPY --from=builder /build/router-monitor .

# Copy startup script
COPY start-router.sh .
RUN chmod +x start-router.sh

# Copy helper scripts
COPY capture-arp.sh .
RUN chmod +x capture-arp.sh

CMD ["/app/start-router.sh"]

FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

WORKDIR /build

# Copy Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY *.go ./

# Build the node binary
RUN go build -o node-sync .

# ============================================
# Final stage
# ============================================
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    iproute2 \
    iputils \
    net-tools \
    arping \
    bash \
    ca-certificates \
    curl

WORKDIR /app

# Copy built binary
COPY --from=builder /build/node-sync .

# Copy startup script
COPY start-node.sh .
RUN chmod +x start-node.sh

# Copy helper scripts
COPY arp-manager.sh .
RUN chmod +x arp-manager.sh

# Copy attack scripts
COPY spoof-attack.sh .
RUN chmod +x spoof-attack.sh

CMD ["/app/start-node.sh"]

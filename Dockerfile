FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o chaincode

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/chaincode .
ENV CHAINCODE_SERVER_ADDRESS=0.0.0.0:9999
CMD ["./chaincode"]

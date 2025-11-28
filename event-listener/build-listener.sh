#!/bin/bash
# Build the Go event listener

echo "🔨 Building event listener..."
go build -o event-listener event-listener.go

if [ $? -eq 0 ]; then
    echo "✅ Event listener built successfully"
    echo "   Run with: ./event-listener"
else
    echo "❌ Build failed"
    exit 1
fi
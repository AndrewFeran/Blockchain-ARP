#!/usr/bin/env bash
echo "=== Running containers ==="
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}"
echo ""
echo "=== Fabric images ==="
docker images | grep hyperledger

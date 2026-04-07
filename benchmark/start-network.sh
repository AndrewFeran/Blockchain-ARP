#!/usr/bin/env bash
set -euo pipefail

FABRIC_DIR="/mnt/c/Users/Perky/OneDrive/Desktop/barp/fabric-samples"
CHAINCODE_DIR="/mnt/c/Users/Perky/OneDrive/Desktop/barp/Blockchain-ARP/chaincode"
export PATH=$PATH:/usr/local/go/bin:$FABRIC_DIR/bin
export IMAGE_TAG=2.5.0
export CA_IMAGE_TAG=1.5.7

echo "============================================================"
echo "  Starting Docker daemon"
echo "============================================================"
service docker start 2>/dev/null || true
sleep 3
docker ps > /dev/null
echo "✅ Docker ready"

echo ""
echo "============================================================"
echo "  Fix chaincode go.sum"
echo "============================================================"
cd "$CHAINCODE_DIR"
go mod tidy
echo "✅ go.sum updated"

echo ""
echo "============================================================"
echo "  Starting Fabric test-network (IMAGE_TAG=2.5.0)"
echo "============================================================"
cd "$FABRIC_DIR/test-network"
./network.sh down 2>/dev/null || true
IMAGE_TAG=2.5.0 CA_IMAGE_TAG=1.5.7 ./network.sh up createChannel -c mychannel -ca
echo "✅ Network up"

echo ""
echo "============================================================"
echo "  Running containers"
echo "============================================================"
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}"

echo ""
echo "============================================================"
echo "  Deploying chaincode via manual lifecycle (avoids image tag issues)"
echo "============================================================"
bash /mnt/c/Users/Perky/OneDrive/Desktop/barp/Blockchain-ARP/benchmark/fix-and-deploy.sh

echo ""
echo "NETWORK_READY"

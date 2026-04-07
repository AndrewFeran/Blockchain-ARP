#!/usr/bin/env bash
set -e
images=(
  "hyperledger/fabric-orderer:2.5.0"
  "hyperledger/fabric-ccenv:2.5.0"
  "hyperledger/fabric-baseos:2.5.0"
  "hyperledger/fabric-tools:2.5.0"
  "hyperledger/fabric-ca:1.5.7"
)
for img in "${images[@]}"; do
  echo "Pulling $img..."
  docker pull "$img" 2>&1 | tail -1
done
echo "ALL_IMAGES_DONE"

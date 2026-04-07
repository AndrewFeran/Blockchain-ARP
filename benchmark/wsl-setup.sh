#!/usr/bin/env bash
set -euo pipefail
export PATH=$PATH:/usr/local/go/bin
export DEBIAN_FRONTEND=noninteractive

echo "============================================================"
echo "  STEP 1: Install Go"
echo "============================================================"
if ! command -v go &>/dev/null; then
    cd /tmp
    wget -q https://go.dev/dl/go1.23.4.linux-amd64.tar.gz
    sudo tar -C /usr/local -xzf go1.23.4.linux-amd64.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    echo "✅ Go installed: $(go version)"
else
    echo "✅ Go already installed: $(go version)"
fi

echo ""
echo "============================================================"
echo "  STEP 2: Install Docker Engine"
echo "============================================================"
if ! command -v docker &>/dev/null; then
    sudo apt-get update -qq
    sudo apt-get install -y -qq ca-certificates curl gnupg lsb-release
    sudo install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /tmp/docker.gpg
    sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg /tmp/docker.gpg
    sudo chmod a+r /etc/apt/keyrings/docker.gpg
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" \
        | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
    sudo apt-get update -qq
    sudo apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-compose-plugin
    sudo usermod -aG docker "$USER"
    echo "✅ Docker installed"
else
    echo "✅ Docker already installed: $(docker --version)"
fi

echo ""
echo "============================================================"
echo "  STEP 3: Start Docker daemon"
echo "============================================================"
if ! sudo docker info &>/dev/null 2>&1; then
    sudo service docker start
    sleep 5
fi
sudo docker info --format '✅ Docker running: {{.ServerVersion}}'

echo ""
echo "============================================================"
echo "  STEP 4: Install jq + git, download fabric-samples"
echo "============================================================"
sudo apt-get install -y -qq jq git
FABRIC_DIR="/mnt/c/Users/Perky/OneDrive/Desktop/barp/fabric-samples"
if [[ ! -d "$FABRIC_DIR" ]]; then
    cd /mnt/c/Users/Perky/OneDrive/Desktop/barp
    curl -sSL https://bit.ly/2ysbOFE | bash -s -- 2.5.0 1.5.7
    echo "✅ fabric-samples downloaded"
else
    echo "✅ fabric-samples already present"
fi

echo ""
echo "============================================================"
echo "  STEP 5: Start Fabric test-network"
echo "============================================================"
export PATH="$FABRIC_DIR/bin:$PATH"
cd "$FABRIC_DIR/test-network"
./network.sh down 2>/dev/null || true
./network.sh up createChannel -c mychannel -ca
echo "✅ Network up, channel created"

echo ""
echo "============================================================"
echo "  STEP 6: Deploy arptracker chaincode"
echo "============================================================"
CHAINCODE_DIR="/mnt/c/Users/Perky/OneDrive/Desktop/barp/Blockchain-ARP/chaincode"
./network.sh deployCC \
    -ccn arptracker \
    -ccp "$CHAINCODE_DIR" \
    -ccl go \
    -c mychannel
echo "✅ Chaincode deployed"

echo ""
echo "============================================================"
echo "  SETUP COMPLETE"
echo "============================================================"

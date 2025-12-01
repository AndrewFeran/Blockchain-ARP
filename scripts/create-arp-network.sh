#!/bin/bash
# Create ARP Test LAN - Virtual Docker Network for ARP Traffic
# This network allows peers to communicate and generate real ARP packets

echo "============================================================"
echo "  Creating ARP Test LAN Network"
echo "============================================================"
echo ""

# Check if network already exists
if docker network ls | grep -q arp-test-lan; then
    echo "⚠️  Network 'arp-test-lan' already exists. Removing..."
    docker network rm arp-test-lan
fi

# Create Docker bridge network
echo "🌐 Creating Docker bridge network 'arp-test-lan'..."
docker network create --driver bridge --subnet 192.168.100.0/24 arp-test-lan
if [ $? -ne 0 ]; then
    echo "❌ Failed to create network!"
    exit 1
fi
echo "✅ Network created with subnet 192.168.100.0/24"
echo ""

# Connect peer containers to ARP network
echo "🔗 Connecting peer containers to ARP network..."

# Org1 peer
echo "  Connecting peer0.org1.example.com (192.168.100.1)..."
docker network connect --ip 192.168.100.1 arp-test-lan peer0.org1.example.com
if [ $? -ne 0 ]; then
    echo "  ⚠️  Warning: Could not connect Org1 peer (may not be running yet)"
fi

# Org2 peer
echo "  Connecting peer0.org2.example.com (192.168.100.2)..."
docker network connect --ip 192.168.100.2 arp-test-lan peer0.org2.example.com
if [ $? -ne 0 ]; then
    echo "  ⚠️  Warning: Could not connect Org2 peer (may not be running yet)"
fi

# Org3 peer
echo "  Connecting peer0.org3.example.com (192.168.100.3)..."
docker network connect --ip 192.168.100.3 arp-test-lan peer0.org3.example.com
if [ $? -ne 0 ]; then
    echo "  ⚠️  Warning: Could not connect Org3 peer (may not be running yet)"
fi

echo ""
echo "============================================================"
echo "🎉 ARP Test LAN Created Successfully!"
echo "============================================================"
echo ""
echo "Network Details:"
echo "  Name:    arp-test-lan"
echo "  Subnet:  192.168.100.0/24"
echo "  Org1:    192.168.100.1"
echo "  Org2:    192.168.100.2"
echo "  Org3:    192.168.100.3"
echo ""
echo "Verify with: docker network inspect arp-test-lan"
echo ""

#!/bin/bash

echo "============================================================"
echo "  🎭 ARP Spoofing Attack Demo"
echo "============================================================"
echo ""
echo "This demo will simulate an ARP spoofing attack and show"
echo "how the blockchain detects and prevents it."
echo ""

# Check if attacker container exists
if ! docker ps -a | grep -q "lan-attacker"; then
    echo "❌ Attacker container not running!"
    echo ""
    echo "Start with attacker profile:"
    echo "  docker-compose --profile testing up -d attacker"
    echo ""
    exit 1
fi

echo "📊 Current network state:"
echo "  Dashboard: http://localhost:5000"
echo ""

read -p "Press Enter to continue..."

echo ""
echo "Step 1: Establishing legitimate ARP entries..."
echo "        Node1 will ping Node2 to create legitimate mapping"
echo ""

docker exec lan-node-1 ping -c 2 10.5.0.11

echo ""
echo "✅ Legitimate mapping established"
echo "   Check dashboard - you should see 'New Device' events"
echo ""

read -p "Press Enter to launch the attack..."

echo ""
echo "Step 2: 🚨 LAUNCHING ARP SPOOFING ATTACK!"
echo "        Attacker claims to be Node1 with fake MAC"
echo ""

docker exec lan-attacker /app/spoof-attack.sh 10.5.0.10 aa:bb:cc:dd:ee:99

echo ""
echo "Step 3: 🛡️  Blockchain Detection"
echo "        Router captures spoofed ARP"
echo "        Chaincode detects MAC address change"
echo "        Event emitted: 'spoofing'"
echo ""

echo "✅ Check the dashboard at http://localhost:5000"
echo "   You should see a RED ALERT for spoofing detection!"
echo ""
echo "Step 4: 🔒 Protection in Action"
echo "        Nodes REJECT the fake MAC address"
echo "        They keep the legitimate entry from blockchain"
echo ""

echo ""
echo "View logs:"
echo "  Router:  docker logs blockchain-router"
echo "  Node1:   docker logs lan-node-1"
echo "  Node2:   docker logs lan-node-2"
echo ""

#!/bin/bash
# Demo Orchestration Script for Classroom Presentation
# This script sets up and runs the complete multi-org ARP detection demo

echo "============================================================"
echo "  🎓 Blockchain-ARP Demo - Classroom Presentation"
echo "============================================================"
echo ""
echo "This demo will:"
echo "  1. Clean up any existing setup"
echo "  2. Set up a 3-organization Hyperledger Fabric network"
echo "  3. Create a simulated LAN for ARP traffic"
echo "  4. Start monitoring agents for each organization"
echo "  5. Generate legitimate network traffic"
echo "  6. Enable malicious mode on Org3 (on command)"
echo ""
read -p "Press Enter to start the demo..."
echo ""

# Change to project directory
cd ~/fabric/arp-chaincode || { echo "❌ Project directory not found"; exit 1; }

# Step 0: Clean up any existing setup
echo "============================================================"
echo "📋 Step 0/6: Cleaning up any existing setup"
echo "============================================================"
echo ""

# Run the comprehensive cleanup script
bash scripts/cleanup-demo.sh

echo "✅ Previous setup cleaned up"
echo ""

# Step 1: Setup 3-org network
echo "============================================================"
echo "📋 Step 1/6: Setting up 3-organization Fabric network"
echo "============================================================"
echo "This will take approximately 2-3 minutes..."
echo ""

bash scripts/setup-3org-network.sh
if [ $? -ne 0 ]; then
    echo "❌ Network setup failed!"
    exit 1
fi
echo ""

# Step 2: Create ARP test LAN
echo "============================================================"
echo "📋 Step 2/6: Creating ARP test LAN network"
echo "============================================================"
echo ""

bash scripts/create-arp-network.sh
if [ $? -ne 0 ]; then
    echo "❌ ARP network creation failed!"
    exit 1
fi
echo ""

# Step 3: Build Docker images
echo "============================================================"
echo "📋 Step 3/6: Building Docker images"
echo "============================================================"
echo ""

echo "Building ARP monitor image..."
cd arp-monitor
docker build -t arp-monitor:latest .
if [ $? -ne 0 ]; then
    echo "❌ Failed to build monitor image!"
    exit 1
fi
cd ..

echo "Building traffic generator image..."
cd traffic-generator
docker build -t traffic-generator:latest .
if [ $? -ne 0 ]; then
    echo "❌ Failed to build traffic generator image!"
    exit 1
fi
cd ..

echo "Building attacker image..."
cd attacker
docker build -t arp-attacker:latest .
if [ $? -ne 0 ]; then
    echo "❌ Failed to build attacker image!"
    exit 1
fi
cd ..

echo "✅ All images built successfully"
echo ""

# Step 4: Start event listener and dashboard
echo "============================================================"
echo "📋 Step 4/6: Starting event listener and dashboard"
echo "============================================================"
echo ""

# Start event listener
echo "Starting event listener..."
cd event-listener
nohup go run event-listener.go > /tmp/event-listener.log 2>&1 &
EVENT_LISTENER_PID=$!
echo "Event listener PID: $EVENT_LISTENER_PID"
cd ..

# Wait for event listener to initialize
sleep 3

# Start dashboard
echo "Starting Flask dashboard..."
cd dashboard
nohup python3 app.py > /tmp/dashboard.log 2>&1 &
DASHBOARD_PID=$!
echo "Dashboard PID: $DASHBOARD_PID"
cd ..

# Wait for dashboard to start
sleep 3

echo "✅ Event listener and dashboard started"
echo "📊 Dashboard URL: http://localhost:5000"
echo ""

# Step 5: Start monitoring agents and traffic generators
echo "============================================================"
echo "📋 Step 5/6: Starting monitoring agents and traffic"
echo "============================================================"
echo ""

docker-compose -f docker-compose-monitors.yaml up -d
if [ $? -ne 0 ]; then
    echo "❌ Failed to start monitoring containers!"
    exit 1
fi

echo "✅ All monitoring agents and traffic generators started"
echo ""

# Step 6: Demo instructions
echo "============================================================"
echo "🎉 DEMO IS READY!"
echo "============================================================"
echo ""
echo "📊 Open dashboard in browser: http://localhost:5000"
echo ""
echo "You should now see:"
echo "  ✅ ARP traffic being reported by all 3 organizations"
echo "  ✅ Organization statistics showing equal participation"
echo "  ✅ Events showing normal ARP activity"
echo ""
echo "============================================================"
echo "🔴 PHASE 2: Trigger Byzantine Attack"
echo "============================================================"
echo ""
echo "When ready to demonstrate the attack, run:"
echo ""
echo "  docker-compose -f docker-compose-monitors.yaml stop monitor-org3"
echo "  docker-compose -f docker-compose-monitors.yaml run -d \\
    -e MALICIOUS_MODE=true --name monitor-org3 monitor-org3"
echo ""
echo "This will enable malicious reporting from Org3 (Laptop)."
echo "The dashboard will show conflicting reports between organizations."
echo ""
echo "============================================================"
echo "📝 Monitoring Commands"
echo "============================================================"
echo ""
echo "View logs:"
echo "  docker logs -f monitor-org1       # Org1 monitor"
echo "  docker logs -f monitor-org2       # Org2 monitor"
echo "  docker logs -f monitor-org3       # Org3 monitor (malicious)"
echo "  docker logs -f traffic-org1       # Traffic generator"
echo "  tail -f /tmp/event-listener.log   # Event listener"
echo "  tail -f /tmp/dashboard.log        # Dashboard"
echo ""
echo "============================================================"
echo "🛑 Cleanup"
echo "============================================================"
echo ""
echo "To stop and clean up everything:"
echo "  bash scripts/cleanup-demo.sh"
echo ""
echo "============================================================"
echo ""
echo "🎓 Demo is running! Good luck with your presentation!"
echo ""

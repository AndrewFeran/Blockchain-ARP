# ⚡ Quick Start Guide - Blockchain ARP

Get the simulated LAN running in under 5 minutes.

---

## Prerequisites Check

```bash
# Check Docker
docker --version
docker-compose --version

# Check if you have Fabric installed
ls ~/fabric/fabric-samples/test-network
```

If missing, install Hyperledger Fabric:
```bash
mkdir -p ~/fabric && cd ~/fabric
curl -sSLO https://raw.githubusercontent.com/hyperledger/fabric/main/scripts/install-fabric.sh
chmod +x install-fabric.sh
./install-fabric.sh docker binary samples
```

---

## Step-by-Step Setup

### 1️⃣ Start Hyperledger Fabric Network

```bash
cd ~/fabric/fabric-samples/test-network
./network.sh up createChannel -c mychannel
```

**Expected output:** Channel 'mychannel' created successfully

### 2️⃣ Deploy ARP Chaincode

```bash
./network.sh deployCC -ccn arptracker -ccp ~/fabric/arp-chaincode/chaincode -ccl go
```

**Expected output:** Chaincode 'arptracker' deployed successfully

### 3️⃣ Start Simulated LAN

```bash
cd ~/fabric/arp-chaincode
chmod +x *.sh
./start-simulated-lan.sh
```

**Expected output:**
```
✅ Simulated LAN is running!

Services:
  🌐 Dashboard:  http://localhost:5000
  🛡️  Router:     blockchain-router (10.5.0.1)
  🖥️  Node 1:     lan-node-1 (10.5.0.10)
  🖥️  Node 2:     lan-node-2 (10.5.0.11)
  🖥️  Node 3:     lan-node-3 (10.5.0.12)
```

### 4️⃣ Open Dashboard

Open browser: **http://localhost:5000**

You should see the dashboard with live ARP events!

### 5️⃣ Run Attack Demo

```bash
./demo-spoofing-attack.sh
```

Follow the interactive prompts to see:
1. Legitimate ARP mappings
2. Spoofing attack launched
3. Blockchain detection (RED ALERT on dashboard)
4. Protection in action

---

## Verify It's Working

### Check Container Status

```bash
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
```

Should show:
- `blockchain-dashboard` (port 5000)
- `blockchain-router`
- `lan-node-1`
- `lan-node-2`
- `lan-node-3`

### View Live Logs

**Router (capturing ARP):**
```bash
docker logs -f blockchain-router
```

You should see:
```
📨 ARP Request: 10.5.0.10 (aa:bb:cc:dd:ee:01) -> 10.5.0.11
✅ Recorded to blockchain: 10.5.0.10 -> aa:bb:cc:dd:ee:01
```

**Node (syncing from blockchain):**
```bash
docker logs -f lan-node-1
```

You should see:
```
🆕 NEW DEVICE: 10.5.0.11 -> aa:bb:cc:dd:ee:02
```

### Check Dashboard

Visit **http://localhost:5000** - you should see:
- Live event feed
- Statistics (events, detections)
- Color-coded alerts

---

## Manual Testing

### Generate ARP Traffic

```bash
# Node1 pings Node2
docker exec lan-node-1 ping -c 3 10.5.0.11
```

**Result:** Dashboard shows new ARP events

### View ARP Tables

```bash
# Router's view
docker exec blockchain-router ip neigh show

# Node's synchronized view
docker exec lan-node-1 ip neigh show
```

They should match!

### Launch Manual Attack

```bash
# Start attacker container
docker-compose --profile testing up -d attacker

# Run attack
docker exec lan-attacker /app/spoof-attack.sh 10.5.0.10 aa:bb:cc:dd:ee:99
```

**Result:** 🚨 Dashboard shows spoofing alert in RED

---

## Troubleshooting

### Dashboard shows nothing

**Check Flask is running:**
```bash
curl http://localhost:5000/api/stats
```

**Restart dashboard:**
```bash
docker-compose restart dashboard
```

### Nodes not connecting to blockchain

**Check peer connectivity:**
```bash
docker exec lan-node-1 nc -zv host.docker.internal 9051
```

If fails:
```bash
# Check if Fabric network is running
docker ps | grep peer0.org
```

### Router not capturing ARP

**Check container has network privileges:**
```bash
docker inspect blockchain-router | grep -i privileged
```

Should show `"Privileged": true`

**Manually trigger ARP:**
```bash
docker exec lan-node-1 arping -c 1 10.5.0.11
```

---

## Stop Everything

### Stop Simulated LAN

```bash
./stop-simulated-lan.sh
```

### Stop Fabric Network

```bash
cd ~/fabric/fabric-samples/test-network
./network.sh down
```

---

## Common Commands Reference

```bash
# View all logs
docker-compose logs -f

# View specific service
docker-compose logs -f router
docker-compose logs -f node1

# Restart a service
docker-compose restart router

# Rebuild after code changes
docker-compose build
docker-compose up -d

# Exec into container
docker exec -it blockchain-router /bin/bash
docker exec -it lan-node-1 /bin/bash

# Check ARP table
docker exec lan-node-1 ip neigh show

# Manual ARP management
docker exec lan-node-1 /app/arp-manager.sh show
```

---

## Next Steps

✅ System is running - what now?

1. **Explore the Dashboard** - Watch real-time ARP synchronization
2. **Run Attack Demos** - Test spoofing detection
3. **Read the Code** - Understand how it works:
   - [router-monitor.go](router/router-monitor.go) - ARP capture
   - [node-sync.go](node/node-sync.go) - Blockchain sync
   - [arp-chaincode.go](chaincode/arp-chaincode.go) - Detection logic

4. **Read Full Documentation:**
   - [SIMULATED-LAN-GUIDE.md](SIMULATED-LAN-GUIDE.md) - Complete guide
   - [README.md](README.md) - Project overview

---

**Need help?** Check the full guides or review container logs for error messages.

**Built with Hyperledger Fabric** | **Ready in 5 Minutes**

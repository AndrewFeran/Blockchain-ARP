# 🔧 Troubleshooting Guide

## Common Issues and Solutions

### 1. "Cannot join channel - already exists"

**Cause:** Network already running from previous demo

**Solution:**
```bash
# Run cleanup script
cd ~/fabric/arp-chaincode/scripts
./cleanup-demo.sh

# Then re-run demo
./demo.sh
```

### 2. Chaincode deployment fails

**Symptoms:** Error during `deployCCAAS` step

**Solution:**
```bash
# Check if chaincode image exists
docker images | grep arptracker

# Rebuild chaincode image
cd ~/fabric/arp-chaincode/chaincode
docker build -t arptracker_ccaas_image:latest .

# Retry deployment
cd ~/fabric/fabric-samples/test-network
./network.sh deployCCAAS -ccn arptracker -ccp ~/fabric/arp-chaincode/chaincode
```

### 3. "Cannot connect to Docker daemon"

**Solution:**
```bash
# Start Docker service
sudo systemctl start docker

# Add your user to docker group (if not already)
sudo usermod -aG docker $USER
newgrp docker
```

### 4. Peer containers not running

**Check status:**
```bash
docker ps | grep peer
```

**If missing:**
```bash
cd ~/fabric/fabric-samples/test-network
./network.sh down
./network.sh up createChannel -ca
```

### 5. ARP network creation fails

**Symptoms:** "Network arp-test-lan already exists"

**Solution:**
```bash
# Remove existing network
docker network rm arp-test-lan

# Disconnect peers if still connected
docker network disconnect arp-test-lan peer0.org1.example.com 2>/dev/null
docker network disconnect arp-test-lan peer0.org2.example.com 2>/dev/null
docker network disconnect arp-test-lan peer0.org3.example.com 2>/dev/null

# Re-run creation script
cd ~/fabric/arp-chaincode/scripts
./create-arp-network.sh
```

### 6. Monitoring agents not starting

**Check logs:**
```bash
docker logs monitor-org1
docker logs monitor-org2
docker logs monitor-org3
```

**Common issues:**
- **Permission denied on network:** Agent needs `NET_ADMIN` capability (already in docker-compose)
- **Cannot find peer:** Check peer is running: `docker ps | grep peer`
- **Fabric credentials missing:** Ensure test-network generated crypto material

**Solution:**
```bash
# Restart monitoring containers
docker-compose -f docker-compose-monitors.yaml down
docker-compose -f docker-compose-monitors.yaml up -d
```

### 7. Dashboard not showing events

**Check event listener:**
```bash
tail -f /tmp/event-listener.log
```

**Check dashboard:**
```bash
tail -f /tmp/dashboard.log
```

**Verify connectivity:**
```bash
# Test dashboard API
curl http://localhost:5000/api/stats

# Test event submission
curl -X POST http://localhost:5000/api/event \
  -H "Content-Type: application/json" \
  -d '{"eventType":"test","ipAddress":"1.2.3.4","macAddress":"AA:BB:CC:DD:EE:FF","timestamp":"2024-01-01T00:00:00Z","message":"Test"}'
```

### 8. Org3 not appearing in network

**Verify Org3 peer:**
```bash
docker ps | grep org3
```

**Check channel membership:**
```bash
cd ~/fabric/fabric-samples/test-network
peer channel getinfo -c mychannel
```

**If missing, re-add Org3:**
```bash
cd ~/fabric/fabric-samples/test-network/addOrg3
./addOrg3.sh up -c mychannel
```

### 9. Port conflicts (7050, 7051, 9051, 11051, 5000)

**Find what's using the port:**
```bash
sudo lsof -i :7051
sudo lsof -i :5000
```

**Kill conflicting process:**
```bash
# For blockchain ports
cd ~/fabric/fabric-samples/test-network
./network.sh down

# For dashboard
pkill -f "app.py"
```

### 10. Docker out of disk space

**Check disk usage:**
```bash
docker system df
```

**Clean up:**
```bash
# Remove unused containers
docker container prune -f

# Remove unused images
docker image prune -a -f

# Remove unused volumes
docker volume prune -f

# Nuclear option (removes everything)
docker system prune -a --volumes -f
```

## Complete Reset

If everything is broken, start fresh:

```bash
# 1. Stop everything
cd ~/fabric/arp-chaincode/scripts
./cleanup-demo.sh

# 2. Remove all Docker artifacts
docker stop $(docker ps -aq)
docker rm $(docker ps -aq)
docker network prune -f
docker volume prune -f

# 3. Remove test-network artifacts
cd ~/fabric/fabric-samples/test-network
./network.sh down
rm -rf organizations/fabric-ca/*/msp
rm -rf channel-artifacts/*
rm -rf system-genesis-block/*

# 4. Start fresh
cd ~/fabric/arp-chaincode/scripts
./demo.sh
```

## Getting Help

If issues persist:

1. **Check logs:**
   - Peer logs: `docker logs peer0.org1.example.com`
   - Orderer logs: `docker logs orderer.example.com`
   - Chaincode logs: `docker logs <chaincode-container>`
   - Event listener: `tail -f /tmp/event-listener.log`
   - Dashboard: `tail -f /tmp/dashboard.log`

2. **Verify prerequisites:**
   - Docker version: `docker --version` (need 20.10+)
   - Docker Compose: `docker-compose --version` (need 2.0+)
   - Go version: `go version` (need 1.21+)
   - Python: `python3 --version` (need 3.8+)

3. **Check system resources:**
   - Disk space: `df -h`
   - Memory: `free -h`
   - Running containers: `docker ps`

4. **Review documentation:**
   - [Hyperledger Fabric Docs](https://hyperledger-fabric.readthedocs.io/)
   - [Test Network Tutorial](https://hyperledger-fabric.readthedocs.io/en/latest/test_network.html)

## Quick Diagnostics Script

Create this script to check system status:

```bash
#!/bin/bash
echo "=== Docker Status ==="
docker ps

echo -e "\n=== Networks ==="
docker network ls | grep -E "fabric|arp"

echo -e "\n=== Fabric Peers ==="
docker ps --filter "name=peer"

echo -e "\n=== Monitoring Agents ==="
docker ps --filter "name=monitor"

echo -e "\n=== Ports in Use ==="
sudo lsof -i :7050,7051,9051,11051,5000 2>/dev/null

echo -e "\n=== Disk Space ==="
docker system df

echo -e "\n=== Recent Logs ==="
docker logs --tail 5 peer0.org1.example.com 2>/dev/null
```

Save as `check-status.sh`, make executable with `chmod +x`, and run when troubleshooting.

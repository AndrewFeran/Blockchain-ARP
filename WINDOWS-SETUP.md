# 🪟 Windows Setup Guide

This project was designed for Linux but can be run on Windows using **WSL2** (Windows Subsystem for Linux).

---

## Prerequisites for Windows

### 1. Install WSL2

Open PowerShell as Administrator:

```powershell
wsl --install
```

Restart your computer when prompted.

### 2. Install Docker Desktop for Windows

Download and install from: https://www.docker.com/products/docker-desktop/

**Important:** In Docker Desktop settings:
- ✅ Enable "Use the WSL 2 based engine"
- ✅ Enable integration with your WSL distro (usually Ubuntu)

### 3. Open WSL Terminal

```powershell
wsl
```

You're now in a Linux environment!

---

## Setup in WSL

From your WSL terminal:

### 1. Install Prerequisites

```bash
# Update package list
sudo apt update

# Install required tools
sudo apt install -y curl git
```

### 2. Install Hyperledger Fabric

```bash
mkdir -p ~/fabric
cd ~/fabric
curl -sSLO https://raw.githubusercontent.com/hyperledger/fabric/main/scripts/install-fabric.sh
chmod +x install-fabric.sh
./install-fabric.sh docker binary samples
```

### 3. Clone This Repository

```bash
cd ~/fabric
git clone <your-repo-url> arp-chaincode
cd arp-chaincode
```

### 4. Make Scripts Executable

```bash
chmod +x *.sh
chmod +x router/*.sh
chmod +x node/*.sh
```

---

## Running the Project

Follow the normal Linux instructions:

### Start Fabric Network

```bash
cd ~/fabric/fabric-samples/test-network
./network.sh up createChannel -c mychannel
./network.sh deployCC -ccn arptracker -ccp ~/fabric/arp-chaincode/chaincode -ccl go
```

### Start Simulated LAN

```bash
cd ~/fabric/arp-chaincode
./start-simulated-lan.sh
```

### Access Dashboard

Open your **Windows browser** to:
```
http://localhost:5000
```

Docker Desktop forwards ports from WSL to Windows automatically!

---

## Important Notes

### File Paths

Your Windows files are accessible in WSL at:
```
/mnt/c/Users/YourName/...
```

Your WSL files are accessible in Windows at:
```
\\wsl$\Ubuntu\home\username\...
```

### Docker Commands

All Docker commands work the same in WSL as on Linux:
```bash
docker ps
docker logs blockchain-router
docker exec -it lan-node-1 /bin/bash
```

### Performance

WSL2 provides near-native Linux performance. Docker containers run efficiently.

---

## Troubleshooting

### "Docker daemon not running"

1. Make sure Docker Desktop is running in Windows
2. Check WSL integration is enabled in Docker Desktop settings
3. Restart WSL: `wsl --shutdown` in PowerShell, then `wsl` again

### "Permission denied" on scripts

```bash
chmod +x *.sh
```

### Port 5000 already in use

Windows might have services on port 5000. Change in `docker-compose.yml`:
```yaml
dashboard:
  ports:
    - "5001:5000"  # Changed from 5000:5000
```

Then access: `http://localhost:5001`

### WSL out of memory

Increase WSL memory limit. Create/edit `.wslconfig` in Windows:
```
C:\Users\YourName\.wslconfig
```

Content:
```ini
[wsl2]
memory=4GB
processors=2
```

Restart WSL.

---

## Alternative: Git Bash (Limited Support)

If you can't use WSL, you can try Git Bash, but functionality will be limited:

1. Install Git for Windows (includes Git Bash)
2. Install Docker Desktop for Windows
3. Use the `.sh` scripts from Git Bash

**Note:** This is not fully tested and may have issues. WSL2 is recommended.

---

## VS Code Integration

Install the "Remote - WSL" extension in VS Code to edit files directly in WSL:

1. Install VS Code in Windows
2. Install "Remote - WSL" extension
3. In WSL terminal: `code .`
4. VS Code opens with WSL integration!

---

## Summary

**For Windows users:**
1. ✅ Install WSL2
2. ✅ Install Docker Desktop with WSL2 integration
3. ✅ Follow Linux instructions inside WSL
4. ✅ Access dashboard from Windows browser

Everything else works exactly the same as on Linux!

---

**Questions?** Check the main guides:
- [QUICKSTART.md](QUICKSTART.md)
- [SIMULATED-LAN-GUIDE.md](SIMULATED-LAN-GUIDE.md)

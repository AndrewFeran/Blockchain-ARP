# 📁 Project Structure

```
Blockchain-ARP/
├── .gitignore                    # Git ignore rules
├── README.md                     # Main project documentation
├── STRUCTURE.md                  # This file
│
├── chaincode/                    # Hyperledger Fabric Smart Contract
│   ├── arp-chaincode.go          # Main chaincode logic (Go)
│   ├── Dockerfile                # Chaincode container image
│   ├── go.mod                    # Go dependencies
│   └── go.sum                    # Go dependency checksums
│
├── dashboard/                    # Web Dashboard (Flask)
│   ├── app.py                    # Flask web server
│   ├── requirements.txt          # Python dependencies
│   ├── README.md                 # Dashboard documentation
│   └── templates/
│       └── index.html            # Dashboard UI
│
├── event-listener/               # Real-time Event Listener (Go)
│   ├── event-listener.go         # Event listener using Fabric Gateway SDK
│   └── build-listener.sh         # Build script
│
└── scripts/                      # Automation Scripts
    ├── reset-and-start.sh        # Complete system setup (one command)
    ├── stop-all.sh               # Shutdown script
    └── README.md                 # Scripts documentation
```

## 🎯 Component Purposes

### `/chaincode`
Smart contract that runs on the blockchain. Detects ARP spoofing by comparing MAC addresses for the same IP over time. Emits events when changes are detected.

### `/dashboard`
Web interface for viewing ARP events in real-time. Shows statistics and alerts for spoofing attacks.

### `/event-listener`
Bridges the blockchain and dashboard. Listens to blockchain events and forwards them to the Flask API.

### `/scripts`
Automation scripts for starting, stopping, and managing the entire system.

## 🚀 Quick Start

```bash
# 1. Start the blockchain and deploy chaincode
cd scripts
./reset-and-start.sh

# 2. Start Flask dashboard (new terminal)
cd dashboard
python3 app.py

# 3. Build and run event listener (new terminal)
cd event-listener
./build-listener.sh
./event-listener

# 4. Access dashboard
open http://localhost:5000
```

## 📝 Notes

- All paths in scripts are relative to `~/fabric/arp-chaincode` on your Azure VM
- The event listener uses the Fabric Gateway SDK for real-time event monitoring
- Chaincode is deployed as an external service (CCaaS) for better performance
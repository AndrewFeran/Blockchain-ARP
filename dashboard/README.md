# ARP Detection Dashboard

Simple Flask dashboard that receives ARP detection events from the blockchain chaincode.

## Setup

```bash
# Install Flask
pip install -r requirements.txt

# Run the dashboard
python app.py
```

The dashboard will start on **http://localhost:5000**

## How It Works

1. **Chaincode** detects ARP events (new device, MAC change, valid update)
2. **Chaincode** sends HTTP POST to `http://localhost:5000/api/event`
3. **Dashboard** displays events in real-time
4. **Auto-refreshes** every 2 seconds

## Event Types

- 🆕 **new** - New device detected on network
- 🚨 **spoofing** - MAC address changed for an IP (potential attack!)
- ✅ **match** - Valid ARP update (MAC matches ledger)

## API Endpoints

- `GET /` - Dashboard web interface
- `POST /api/event` - Receive events from chaincode
- `GET /api/events` - Get all events (JSON)
- `GET /api/stats` - Get statistics (JSON)

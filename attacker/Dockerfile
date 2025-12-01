FROM python:3.10-slim

# Install system dependencies
RUN apt-get update && apt-get install -y \
    tcpdump \
    net-tools \
    iputils-ping \
    iproute2 \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Install scapy
RUN pip install --no-cache-dir scapy==2.5.0

# Copy attack script
COPY arp-spoof.py .

# Make script executable
RUN chmod +x arp-spoof.py

# Default: just sleep (manual triggering)
CMD ["sleep", "infinity"]

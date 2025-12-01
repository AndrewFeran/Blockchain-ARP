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

# Copy requirements and install Python dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy monitoring scripts
COPY monitor.py .
COPY monitor-malicious.py .

# Make scripts executable
RUN chmod +x monitor.py monitor-malicious.py

# Create fabric directories for mounting
RUN mkdir -p /fabric/bin /fabric/config

# Default command (can be overridden)
CMD ["python3", "monitor.py"]

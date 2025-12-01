FROM python:3.10-slim

# Install system dependencies
RUN apt-get update && apt-get install -y \
    iputils-ping \
    net-tools \
    iproute2 \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy traffic generator script
COPY generator.py .

# Make script executable
RUN chmod +x generator.py

# Run the generator
CMD ["python3", "generator.py"]

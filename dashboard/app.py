from flask import Flask, render_template, request, jsonify
from datetime import datetime
import json

app = Flask(__name__)

# Store events in memory (simple approach)
events = []
MAX_EVENTS = 1000

@app.route('/')
def index():
    return render_template('index.html')

@app.route('/api/event', methods=['POST'])
def receive_event():
    """Receive ARP detection events from chaincode"""
    try:
        event = request.json
        event['received_at'] = datetime.now().isoformat()

        # Add to beginning of list (newest first)
        events.insert(0, event)

        # Keep only last MAX_EVENTS
        if len(events) > MAX_EVENTS:
            events.pop()

        # Log to console
        event_type = event.get('eventType', 'unknown')
        ip = event.get('ipAddress', 'N/A')
        mac = event.get('macAddress', 'N/A')

        if event_type == 'spoofing':
            print(f"🚨 SPOOFING DETECTED! IP: {ip}, Old: {event.get('previousMAC')}, New: {mac}")
        elif event_type == 'new':
            print(f"🆕 New device: IP: {ip}, MAC: {mac}")
        else:
            print(f"✅ Valid: IP: {ip}, MAC: {mac}")

        return jsonify({"status": "success"}), 200
    except Exception as e:
        print(f"Error receiving event: {e}")
        return jsonify({"status": "error", "message": str(e)}), 400

@app.route('/api/events', methods=['GET'])
def get_events():
    """Get all events for the dashboard"""
    limit = request.args.get('limit', 100, type=int)
    return jsonify(events[:limit])

@app.route('/api/stats', methods=['GET'])
def get_stats():
    """Get statistics"""
    total = len(events)
    spoofing = sum(1 for e in events if e.get('eventType') == 'spoofing')
    new_devices = sum(1 for e in events if e.get('eventType') == 'new')
    matches = sum(1 for e in events if e.get('eventType') == 'match')

    return jsonify({
        'total': total,
        'spoofing': spoofing,
        'new_devices': new_devices,
        'matches': matches
    })

@app.route('/api/org-stats', methods=['GET'])
def get_org_stats():
    """Get statistics by organization"""
    org_counts = {}

    for event in events:
        # Extract organization from recordedBy field
        recorded_by = event.get('recordedBy', 'Unknown')
        org_counts[recorded_by] = org_counts.get(recorded_by, 0) + 1

    return jsonify(org_counts)

if __name__ == '__main__':
    print("🚀 ARP Detection Dashboard Starting...")
    print("📊 Dashboard: http://localhost:5000")
    print("📡 API Endpoint: http://localhost:5000/api/event")
    app.run(host='0.0.0.0', port=5000, debug=True)

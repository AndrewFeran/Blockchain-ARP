import os
from datetime import datetime

from flask import Flask, jsonify, render_template, request

app = Flask(__name__)

# Store events in memory for the live demo dashboard.
events = []
MAX_EVENTS = 1000


@app.route("/")
def index():
    return render_template("index.html")


@app.route("/api/event", methods=["POST"])
def receive_event():
    """Receive ARP detection events from chaincode."""
    try:
        event = request.json
        event["received_at"] = datetime.now().isoformat()

        # Add to beginning of list (newest first)
        events.insert(0, event)

        if len(events) > MAX_EVENTS:
            events.pop()

        event_type = event.get("eventType", "unknown")
        ip = event.get("ipAddress", "N/A")
        mac = event.get("macAddress", "N/A")

        if event_type == "spoofing":
            print(f"SPOOFING DETECTED! IP: {ip}, Old: {event.get('previousMAC')}, New: {mac}")
        elif event_type == "new":
            print(f"New device: IP: {ip}, MAC: {mac}")
        elif event_type == "expired":
            print(f"Expired mapping: IP: {ip}, MAC: {mac}")
        else:
            print(f"Valid: IP: {ip}, MAC: {mac}")

        return jsonify({"status": "success"}), 200
    except Exception as exc:
        print(f"Error receiving event: {exc}")
        return jsonify({"status": "error", "message": str(exc)}), 400


@app.route("/api/events", methods=["GET"])
def get_events():
    """Get all events for the dashboard."""
    limit = request.args.get("limit", 100, type=int)
    return jsonify(events[:limit])


@app.route("/api/stats", methods=["GET"])
def get_stats():
    """Get dashboard event statistics."""
    total = len(events)
    spoofing = sum(1 for event in events if event.get("eventType") == "spoofing")
    new_devices = sum(1 for event in events if event.get("eventType") == "new")
    matches = sum(1 for event in events if event.get("eventType") == "match")
    expired = sum(1 for event in events if event.get("eventType") == "expired")

    return jsonify(
        {
            "total": total,
            "spoofing": spoofing,
            "new_devices": new_devices,
            "matches": matches,
            "expired": expired,
        }
    )


if __name__ == "__main__":
    print("ARP Detection Dashboard Starting...")
    print("Dashboard: http://localhost:5000")
    print("API Endpoint: http://localhost:5000/api/event")
    debug = os.getenv("FLASK_DEBUG", "0").lower() in {"1", "true", "yes"}
    app.run(host="0.0.0.0", port=5000, debug=debug, use_reloader=False)

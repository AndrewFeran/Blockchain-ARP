import unittest

import app as dashboard


class DashboardAPITest(unittest.TestCase):
    def setUp(self):
        dashboard.events.clear()
        self.client = dashboard.app.test_client()

    def test_receive_event_stores_newest_first_and_counts_expired(self):
        first = {
            "eventType": "new",
            "ipAddress": "10.5.0.10",
            "macAddress": "aa:bb:cc:dd:ee:10",
            "timestamp": "2026-05-31T21:00:00Z",
        }
        second = {
            "eventType": "expired",
            "ipAddress": "10.5.0.10",
            "macAddress": "aa:bb:cc:dd:ee:10",
            "timestamp": "2026-05-31T21:01:00Z",
        }

        self.assertEqual(self.client.post("/api/event", json=first).status_code, 200)
        self.assertEqual(self.client.post("/api/event", json=second).status_code, 200)

        events = self.client.get("/api/events").get_json()
        self.assertEqual(events[0]["eventType"], "expired")
        self.assertEqual(events[1]["eventType"], "new")

        stats = self.client.get("/api/stats").get_json()
        self.assertEqual(stats["total"], 2)
        self.assertEqual(stats["new_devices"], 1)
        self.assertEqual(stats["expired"], 1)
        self.assertEqual(stats["spoofing"], 0)
        self.assertEqual(stats["matches"], 0)


if __name__ == "__main__":
    unittest.main()

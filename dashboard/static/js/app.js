function formatTime(timestamp) {
    const date = new Date(timestamp);
    return date.toLocaleString();
}

function loadStats() {
    fetch('/api/stats')
        .then(response => response.json())
        .then(stats => {
            document.getElementById('stat-total').textContent = stats.total;
            document.getElementById('stat-spoofing').textContent = stats.spoofing;
            document.getElementById('stat-new').textContent = stats.new_devices;
            document.getElementById('stat-match').textContent = stats.matches;
        })
        .catch(error => console.error('Error loading stats:', error));
}

function loadOrgStats() {
    fetch('/api/org-stats')
        .then(response => response.json())
        .then(orgStats => {
            const container = document.getElementById('org-stats-container');

            if (Object.keys(orgStats).length === 0) {
                container.innerHTML = '<div class="no-events">No data yet...</div>';
                return;
            }

            // Create org stat cards
            container.innerHTML = Object.entries(orgStats).map(([org, count]) => `
                <div class="org-stat-card org-${org.toLowerCase().replace(/[^a-z0-9]/g, '')}">
                    <div class="org-name">${org}</div>
                    <div class="org-count">${count} reports</div>
                </div>
            `).join('');
        })
        .catch(error => console.error('Error loading org stats:', error));
}

function loadEvents() {
    fetch('/api/events')
        .then(response => response.json())
        .then(events => {
            const container = document.getElementById('events-list');

            if (events.length === 0) {
                container.innerHTML = '<div class="no-events">No events yet. Waiting for ARP traffic...</div>';
                return;
            }

            container.innerHTML = events.map(event => `
                        <div class="event ${event.eventType}">
                            <div class="event-header">
                                <span class="event-type ${event.eventType}">${event.eventType}</span>
                                <span class="event-time">${formatTime(event.timestamp)}</span>
                            </div>
                            <div class="event-details">
                                <div class="event-detail">
                                    <strong>IP:</strong> ${event.ipAddress}
                                </div>
                                <div class="event-detail">
                                    <strong>MAC:</strong> ${event.macAddress}
                                </div>
                                ${event.previousMAC ? `
                                <div class="event-detail">
                                    <strong>Previous MAC:</strong> ${event.previousMAC}
                                </div>
                                ` : ''}
                                <div class="event-detail">
                                    <strong>Node:</strong> ${event.recordedBy}
                                </div>
                                ${event.hostname ? `
                                <div class="event-detail">
                                    <strong>Hostname:</strong> ${event.hostname}
                                </div>
                                ` : ''}
                            </div>
                            <div class="event-message">${event.message}</div>
                        </div>
                    `).join('');
        })
        .catch(error => console.error('Error loading events:', error));

    loadStats();
    loadOrgStats();
}

// Load events immediately
loadEvents();

// Auto-refresh every 2 seconds
setInterval(loadEvents, 2000);
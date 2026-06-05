const state = {
    events: [],
    selectedKey: null,
    filter: 'all',
    hasLoaded: false,
};

const eventMeta = {
    spoofing: {
        label: 'Spoofing',
        detail: 'MAC conflict detected',
        icon: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3l9 16H3L12 3z"/><path d="M12 9v4M12 17h.01"/></svg>',
    },
    new: {
        label: 'New',
        detail: 'New mapping registered',
        icon: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 17h16"/><path d="M6 17V7h12v10"/><path d="M12 10v5M9.5 12.5h5"/></svg>',
    },
    match: {
        label: 'Valid',
        detail: 'Mapping matches ledger',
        icon: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3l7 3v5c0 4.6-2.8 8.8-7 10-4.2-1.2-7-5.4-7-10V6l7-3z"/><path d="M8.5 12.1l2.2 2.2 4.8-5"/></svg>',
    },
    expired: {
        label: 'Expired',
        detail: 'Mapping aged out',
        icon: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 7v5l3 2"/><path d="M21 12a9 9 0 1 1-9-9 9 9 0 0 1 9 9z"/></svg>',
    },
};

function escapeHtml(value) {
    return String(value ?? '')
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
}

function eventKey(event, index) {
    return [
        event.timestamp,
        event.received_at,
        event.eventType,
        event.ipAddress,
        event.macAddress,
        index,
    ].map(part => part ?? '').join('|');
}

function parseDate(value) {
    if (!value) return null;
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? null : date;
}

function formatTime(value, fallback = 'Unknown') {
    const date = parseDate(value);
    if (!date) return fallback;
    return date.toLocaleString([], {
        month: 'short',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
    });
}

function formatRelative(value) {
    const date = parseDate(value);
    if (!date) return 'Waiting';
    const seconds = Math.max(0, Math.round((Date.now() - date.getTime()) / 1000));
    if (seconds < 5) return 'just now';
    if (seconds < 60) return `${seconds}s ago`;
    const minutes = Math.round(seconds / 60);
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.round(minutes / 60);
    if (hours < 24) return `${hours}h ago`;
    return formatTime(value);
}

function setRefreshState(label, paused = false) {
    document.getElementById('refresh-state').textContent = label;
    document.getElementById('live-state').classList.toggle('paused', paused);
    document.getElementById('live-label').textContent = paused ? 'Paused' : 'Live';
}

function updateStats(stats) {
    document.getElementById('stat-total').textContent = stats.total ?? 0;
    document.getElementById('stat-spoofing').textContent = stats.spoofing ?? 0;
    document.getElementById('stat-new').textContent = stats.new_devices ?? 0;
    document.getElementById('stat-match').textContent = stats.matches ?? 0;
    document.getElementById('stat-expired').textContent = stats.expired ?? 0;
    document.getElementById('spoofing-kpi').classList.toggle('active-alert', Number(stats.spoofing) > 0);
}

function filteredEvents() {
    if (state.filter === 'all') return state.events;
    return state.events.filter(event => event.eventType === state.filter);
}

function renderEvent(event, key, selected) {
    const type = event.eventType || 'match';
    const meta = eventMeta[type] || eventMeta.match;
    const hostname = event.hostname || 'unresolved';
    const node = event.recordedBy || 'unknown observer';

    return `
        <button class="event-row ${escapeHtml(type)} ${selected ? 'selected' : ''}" type="button" data-key="${escapeHtml(key)}">
            <span class="event-icon ${escapeHtml(type)}">${meta.icon}</span>
            <span class="event-main">
                <span class="event-topline">
                    <span class="event-title">${escapeHtml(meta.detail)}</span>
                    <span class="event-time">${escapeHtml(formatRelative(event.received_at || event.timestamp))}</span>
                </span>
                <span class="event-address">
                    <code>${escapeHtml(event.ipAddress || '0.0.0.0')}</code>
                    <span class="muted">to</span>
                    <code>${escapeHtml(event.macAddress || 'unknown-mac')}</code>
                </span>
                <span class="event-context">
                    <span>${escapeHtml(hostname)}</span>
                    <span>${escapeHtml(node)}</span>
                    ${event.trialId ? `<span>trial ${escapeHtml(event.trialId)}</span>` : ''}
                </span>
            </span>
            <span class="severity-chip ${escapeHtml(type)}">${escapeHtml(meta.label)}</span>
        </button>
    `;
}

function renderEvents() {
    const container = document.getElementById('events-list');
    const countLabel = document.getElementById('event-count-label');
    const events = filteredEvents();

    container.classList.remove('loading');
    countLabel.textContent = state.events.length
        ? `${events.length} shown from ${state.events.length} recent events`
        : 'No events received from the Flask API';

    if (!state.events.length) {
        container.innerHTML = `
            <div class="empty-state">
                <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 17h16"/><path d="M6 17V7h12v10"/><path d="M8 11h8"/></svg>
                <strong>No ARP events yet</strong>
                <span>Waiting for router observations and Fabric event sync.</span>
            </div>
        `;
        state.selectedKey = null;
        renderDetail(null);
        return;
    }

    if (!events.length) {
        container.innerHTML = `
            <div class="empty-state">
                <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 5h16M7 12h10M10 19h4"/></svg>
                <strong>No matching events</strong>
                <span>The selected filter has no entries in the recent event window.</span>
            </div>
        `;
        renderDetail(findSelectedEvent());
        return;
    }

    if (!state.selectedKey || !findSelectedEvent()) {
        state.selectedKey = eventKey(events[0], state.events.indexOf(events[0]));
    }

    container.innerHTML = events
        .map(event => {
            const key = eventKey(event, state.events.indexOf(event));
            return renderEvent(event, key, key === state.selectedKey);
        })
        .join('');

    container.querySelectorAll('.event-row').forEach(row => {
        row.addEventListener('click', () => {
            state.selectedKey = row.dataset.key;
            renderEvents();
            renderDetail(findSelectedEvent());
        });
    });

    renderDetail(findSelectedEvent());
}

function findSelectedEvent() {
    return state.events.find((event, index) => eventKey(event, index) === state.selectedKey) || null;
}

function detailRow(label, value, mono = false) {
    return `
        <div class="detail-row">
            <span>${escapeHtml(label)}</span>
            <strong class="${mono ? 'mono' : ''}">${escapeHtml(value || 'N/A')}</strong>
        </div>
    `;
}

function renderDetail(event) {
    const detail = document.getElementById('event-detail');
    const subtitle = document.getElementById('detail-subtitle');
    const severity = document.getElementById('detail-severity');

    if (!event) {
        subtitle.textContent = 'Select an event to inspect';
        severity.className = 'severity-chip neutral';
        severity.textContent = 'None';
        detail.className = 'detail-content empty-detail';
        detail.innerHTML = `
            <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M12 3l7 3v5c0 4.6-2.8 8.8-7 10-4.2-1.2-7-5.4-7-10V6l7-3z"/>
                <path d="M9 12h6"/>
            </svg>
            <p>No event selected</p>
        `;
        return;
    }

    const type = event.eventType || 'match';
    const meta = eventMeta[type] || eventMeta.match;
    subtitle.textContent = `${event.ipAddress || 'Unknown IP'} observed by ${event.recordedBy || 'unknown node'}`;
    severity.className = `severity-chip ${type}`;
    severity.textContent = meta.label;
    detail.className = `detail-content ${type}`;
    detail.innerHTML = `
        <div class="detail-summary">
            <span class="detail-icon ${escapeHtml(type)}">${meta.icon}</span>
            <div>
                <strong>${escapeHtml(meta.detail)}</strong>
                <p>${escapeHtml(event.message || 'No event message supplied.')}</p>
            </div>
        </div>
        <div class="detail-grid">
            ${detailRow('IP Address', event.ipAddress, true)}
            ${detailRow('Current MAC', event.macAddress, true)}
            ${event.previousMAC ? detailRow('Previous MAC', event.previousMAC, true) : ''}
            ${detailRow('Hostname', event.hostname || 'unresolved')}
            ${detailRow('Recorded By', event.recordedBy)}
            ${detailRow('Timestamp', formatTime(event.timestamp))}
            ${detailRow('Received', formatTime(event.received_at))}
            ${event.trialId ? detailRow('Trial ID', event.trialId, true) : ''}
        </div>
    `;
}

function renderAlertBanner() {
    const banner = document.getElementById('active-alert');
    const activeSpoof = state.events.find(event => event.eventType === 'spoofing');

    if (!activeSpoof) {
        banner.classList.add('hidden');
        banner.textContent = '';
        return;
    }

    banner.classList.remove('hidden');
    banner.innerHTML = `
        <strong>Active spoofing alert</strong>
        <span class="alert-flow">
            <code>${escapeHtml(activeSpoof.ipAddress || 'unknown-ip')}</code>
            <span>changed from</span>
            <code>${escapeHtml(activeSpoof.previousMAC || 'unknown')}</code>
            <span>to</span>
            <code>${escapeHtml(activeSpoof.macAddress || 'unknown')}</code>
        </span>
    `;
}

function authoritativeRows() {
    const byIp = new Map();

    state.events.forEach(event => {
        const ip = event.ipAddress;
        if (!ip || byIp.has(ip)) return;
        const type = event.eventType || 'match';
        byIp.set(ip, {
            ip,
            mac: type === 'spoofing' ? (event.previousMAC || event.macAddress) : event.macAddress,
            hostname: event.hostname || 'unresolved',
            lastSeen: event.received_at || event.timestamp,
            state: type === 'spoofing' ? 'conflict' : type,
        });
    });

    return Array.from(byIp.values()).sort((a, b) => {
        const aTime = parseDate(a.lastSeen)?.getTime() || 0;
        const bTime = parseDate(b.lastSeen)?.getTime() || 0;
        return bTime - aTime;
    });
}

function renderArpTable() {
    const body = document.getElementById('arp-table-body');
    const label = document.getElementById('arp-count-label');
    const rows = authoritativeRows();

    label.textContent = rows.length
        ? `${rows.length} mappings derived from latest events`
        : 'Derived from latest authoritative events';

    if (!rows.length) {
        body.innerHTML = '<tr class="table-empty"><td colspan="5">Waiting for authoritative mappings.</td></tr>';
        return;
    }

    body.innerHTML = rows.map(row => `
        <tr>
            <td><code>${escapeHtml(row.ip)}</code></td>
            <td><code>${escapeHtml(row.mac || 'unknown-mac')}</code></td>
            <td>${escapeHtml(row.hostname)}</td>
            <td>${escapeHtml(formatTime(row.lastSeen))}</td>
            <td><span class="state-badge ${escapeHtml(row.state)}">${escapeHtml(row.state)}</span></td>
        </tr>
    `).join('');
}

function updateLastEvent() {
    const latest = state.events[0];
    document.getElementById('last-event-time').textContent = latest
        ? formatRelative(latest.received_at || latest.timestamp)
        : 'Waiting';
}

function renderAll() {
    renderEvents();
    renderAlertBanner();
    renderArpTable();
    updateLastEvent();
}

async function loadDashboard() {
    setRefreshState('Syncing');

    try {
        const [eventsResponse, statsResponse] = await Promise.all([
            fetch('/api/events?limit=100'),
            fetch('/api/stats'),
        ]);

        if (!eventsResponse.ok || !statsResponse.ok) {
            throw new Error('Dashboard API request failed');
        }

        state.events = await eventsResponse.json();
        updateStats(await statsResponse.json());
        state.hasLoaded = true;
        setRefreshState('2s');
        renderAll();
    } catch (error) {
        console.error('Error loading dashboard:', error);
        setRefreshState('Paused', true);
        const container = document.getElementById('events-list');
        container.classList.remove('loading');
        container.innerHTML = `
            <div class="empty-state error">
                <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3l9 16H3L12 3z"/><path d="M12 9v4M12 17h.01"/></svg>
                <strong>Dashboard API unavailable</strong>
                <span>Check the Flask service and retry the refresh.</span>
            </div>
        `;
    }
}

document.getElementById('refresh-button').addEventListener('click', loadDashboard);

document.querySelectorAll('.filter-tab').forEach(button => {
    button.addEventListener('click', () => {
        document.querySelectorAll('.filter-tab').forEach(tab => tab.classList.remove('active'));
        button.classList.add('active');
        state.filter = button.dataset.filter;
        renderEvents();
    });
});

loadDashboard();
setInterval(loadDashboard, 2000);

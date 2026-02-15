let allUsers = [];

async function loadData() {
    try {
        const users = await window.go.main.App.GetFollowingList();
        const stats = await window.go.main.App.GetStats();

        allUsers = users || [];
        document.getElementById('total-count').textContent = stats.total_following;
        document.getElementById('last-fetch').textContent = stats.last_fetch_at
            ? 'Last: ' + new Date(stats.last_fetch_at).toLocaleTimeString()
            : 'Not fetched yet';

        renderTable(allUsers);
    } catch (err) {
        console.error('Error loading data:', err);
        document.getElementById('table-body').innerHTML =
            '<tr><td colspan="7" class="loading">Error loading data</td></tr>';
    }
}

function renderTable(users) {
    const tbody = document.getElementById('table-body');

    if (!users || users.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" class="loading">No data yet. Fetching...</td></tr>';
        return;
    }

    tbody.innerHTML = users.map(u => `
        <tr>
            <td>${u.profile_image_url
                ? `<img class="avatar" src="${u.profile_image_url}" alt="" loading="lazy">`
                : '<div class="avatar"></div>'}</td>
            <td>
                <div class="user-cell">
                    <span class="user-name">
                        ${escapeHtml(u.name)}${u.verified ? '<span class="verified-badge">&#x2713;</span>' : ''}
                    </span>
                    <span class="user-handle">@${escapeHtml(u.username)}</span>
                </div>
            </td>
            <td class="desc-cell" title="${escapeHtml(u.description)}">${escapeHtml(u.description)}</td>
            <td class="num-cell">${formatNumber(u.followers_count)}</td>
            <td class="num-cell">${formatNumber(u.following_count)}</td>
            <td class="num-cell">${formatNumber(u.tweet_count)}</td>
            <td class="loc-cell">${escapeHtml(u.location)}</td>
        </tr>
    `).join('');
}

function filterTable() {
    const query = document.getElementById('search').value.toLowerCase();
    const filtered = allUsers.filter(u =>
        u.username.toLowerCase().includes(query) ||
        u.name.toLowerCase().includes(query) ||
        (u.description && u.description.toLowerCase().includes(query))
    );
    renderTable(filtered);
}

function sortTable() {
    const sortBy = document.getElementById('sort-by').value;
    const sortDir = document.getElementById('sort-dir').value;

    const sorted = [...allUsers].sort((a, b) => {
        let va = a[sortBy];
        let vb = b[sortBy];

        if (typeof va === 'string') {
            va = va.toLowerCase();
            vb = vb.toLowerCase();
            return sortDir === 'asc' ? va.localeCompare(vb) : vb.localeCompare(va);
        }
        return sortDir === 'asc' ? va - vb : vb - va;
    });

    renderTable(sorted);
}

function sortBy(field) {
    const sortByEl = document.getElementById('sort-by');
    const sortDirEl = document.getElementById('sort-dir');

    if (sortByEl.value === field) {
        sortDirEl.value = sortDirEl.value === 'desc' ? 'asc' : 'desc';
    } else {
        sortByEl.value = field;
        sortDirEl.value = 'desc';
    }
    sortTable();
}

async function fetchNow() {
    const btn = document.getElementById('fetch-btn');
    btn.disabled = true;
    btn.textContent = 'Fetching...';

    try {
        const result = await window.go.main.App.FetchNow();
        console.log(result);
        await loadData();
    } catch (err) {
        console.error('Error fetching:', err);
    } finally {
        btn.disabled = false;
        btn.textContent = 'Fetch Now';
    }
}

function formatNumber(n) {
    if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M';
    if (n >= 1000) return (n / 1000).toFixed(1) + 'K';
    return String(n);
}

function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Initial load
document.addEventListener('DOMContentLoaded', () => {
    // Wait a moment for Wails bindings to be ready
    setTimeout(loadData, 500);

    // Refresh data every 5 minutes
    setInterval(loadData, 5 * 60 * 1000);
});

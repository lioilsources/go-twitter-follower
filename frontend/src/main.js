let allUsers = [];
let allFollowers = [];

// --- Account management ---

async function loadAccounts() {
    try {
        const accounts = await window.go.main.App.GetAccounts();
        const selected = await window.go.main.App.GetSelectedAccount();
        const select = document.getElementById('account-select');

        if (!accounts || accounts.length === 0) {
            select.innerHTML = '<option value="">No accounts</option>';
            return;
        }

        select.innerHTML = accounts.map(a =>
            `<option value="${a.user_id}" ${a.user_id === selected ? 'selected' : ''}>@${escapeHtml(a.username)}</option>`
        ).join('');
    } catch (err) {
        console.error('Error loading accounts:', err);
    }
}

async function switchAccount(userID) {
    if (!userID) return;
    await window.go.main.App.SelectAccount(userID);
    await loadData();
}

async function addAccount() {
    const usernameInput = document.getElementById('new-username');
    const bearerInput = document.getElementById('new-bearer');
    const btn = document.getElementById('add-account-btn');
    const username = usernameInput.value.trim().replace(/^@/, '');
    const bearer = bearerInput.value.trim();

    if (!username || !bearer) return;

    btn.disabled = true;
    btn.textContent = 'Adding...';

    try {
        await window.go.main.App.AddNewAccount(username, bearer);
        usernameInput.value = '';
        bearerInput.value = '';
        await loadAccounts();
        await loadAccountList();
        await loadData();
    } catch (err) {
        alert('Error adding account: ' + err);
    } finally {
        btn.disabled = false;
        btn.textContent = 'Add Account';
    }
}

async function removeAccount(userID) {
    if (!confirm('Remove this account?')) return;
    try {
        await window.go.main.App.RemoveAccountByID(userID);
        await loadAccounts();
        await loadAccountList();
        await loadData();
    } catch (err) {
        alert('Error removing account: ' + err);
    }
}

function toggleAccountManager() {
    const modal = document.getElementById('account-modal');
    const isVisible = modal.classList.contains('visible');
    if (isVisible) {
        modal.classList.remove('visible');
    } else {
        modal.classList.add('visible');
        loadAccountList();
    }
}

async function loadAccountList() {
    try {
        const accounts = await window.go.main.App.GetAccounts();
        const list = document.getElementById('account-list');

        if (!accounts || accounts.length === 0) {
            list.innerHTML = '<div class="no-accounts">No accounts added yet.</div>';
            return;
        }

        list.innerHTML = accounts.map(a => `
            <div class="account-item">
                <span>@${escapeHtml(a.username)}</span>
                <button onclick="removeAccount('${a.user_id}')" class="remove-btn">Remove</button>
            </div>
        `).join('');
    } catch (err) {
        console.error('Error loading account list:', err);
    }
}

// --- Stats display ---

function updateStatsDisplay(stats, label) {
    document.getElementById('total-count').textContent = stats.total_count || 0;
    document.getElementById('stats-label').textContent = label;
    document.getElementById('last-fetch').textContent = stats.last_fetch_at
        ? 'Fetched: ' + new Date(stats.last_fetch_at).toLocaleDateString()
        : 'Not fetched yet';

    const nextEl = document.getElementById('next-fetch');
    if (stats.cache_expires_at) {
        const expires = new Date(stats.cache_expires_at);
        const now = new Date();
        if (expires > now) {
            nextEl.textContent = 'Next: ' + expires.toLocaleDateString();
        } else {
            nextEl.textContent = 'Cache expired';
        }
    } else {
        nextEl.textContent = '';
    }
}

// --- Data loading ---

async function loadData() {
    try {
        const users = await window.go.main.App.GetFollowingList();
        const stats = await window.go.main.App.GetStats();

        allUsers = users || [];
        updateStatsDisplay(stats, 'following');
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
        tbody.innerHTML = '<tr><td colspan="7" class="loading">No data yet. Click Fetch Now to start.</td></tr>';
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

// --- Followers ---

async function loadFollowers() {
    try {
        const users = await window.go.main.App.GetFollowersList();
        const stats = await window.go.main.App.GetFollowersStats();

        allFollowers = users || [];
        updateStatsDisplay(stats, 'followers');
        renderFollowersTable(allFollowers);
    } catch (err) {
        console.error('Error loading followers:', err);
        document.getElementById('followers-body').innerHTML =
            '<tr><td colspan="7" class="loading">Error loading data</td></tr>';
    }
}

function renderFollowersTable(users) {
    const tbody = document.getElementById('followers-body');

    if (!users || users.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" class="loading">No data yet. Click Fetch Now to start.</td></tr>';
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

function filterFollowersTable() {
    const query = document.getElementById('followers-search').value.toLowerCase();
    const filtered = allFollowers.filter(u =>
        u.username.toLowerCase().includes(query) ||
        u.name.toLowerCase().includes(query) ||
        (u.description && u.description.toLowerCase().includes(query))
    );
    renderFollowersTable(filtered);
}

function sortFollowersTable() {
    const sortByVal = document.getElementById('followers-sort-by').value;
    const sortDir = document.getElementById('followers-sort-dir').value;

    const sorted = [...allFollowers].sort((a, b) => {
        let va = a[sortByVal];
        let vb = b[sortByVal];

        if (typeof va === 'string') {
            va = va.toLowerCase();
            vb = vb.toLowerCase();
            return sortDir === 'asc' ? va.localeCompare(vb) : vb.localeCompare(va);
        }
        return sortDir === 'asc' ? va - vb : vb - va;
    });

    renderFollowersTable(sorted);
}

function sortFollowersBy(field) {
    const sortByEl = document.getElementById('followers-sort-by');
    const sortDirEl = document.getElementById('followers-sort-dir');

    if (sortByEl.value === field) {
        sortDirEl.value = sortDirEl.value === 'desc' ? 'asc' : 'desc';
    } else {
        sortByEl.value = field;
        sortDirEl.value = 'desc';
    }
    sortFollowersTable();
}

async function fetchNow() {
    const btn = document.getElementById('fetch-btn');
    btn.disabled = true;
    btn.textContent = 'Fetching...';

    try {
        const isListsTab = document.getElementById('tab-lists').classList.contains('active');
        const isFollowersTab = document.getElementById('tab-followers').classList.contains('active');
        if (isListsTab) {
            const result = await window.go.main.App.FetchListsNow();
            console.log(result);
            await loadLists();
        } else if (isFollowersTab) {
            const result = await window.go.main.App.FetchFollowersNow();
            console.log(result);
            await loadFollowers();
        } else {
            const result = await window.go.main.App.FetchNow();
            console.log(result);
            await loadData();
        }
    } catch (err) {
        console.error('Error fetching:', err);
    } finally {
        btn.disabled = false;
        btn.textContent = 'Fetch Now';
    }
}

// --- Tab switching ---

function switchTab(tab) {
    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(t => t.classList.remove('active'));

    document.querySelector(`.tab[onclick="switchTab('${tab}')"]`).classList.add('active');
    document.getElementById('tab-' + tab).classList.add('active');

    if (tab === 'lists') {
        loadLists();
    } else if (tab === 'followers') {
        loadFollowers();
    } else if (tab === 'following') {
        loadData();
    }
}

// --- Lists ---

let allListMembers = [];

async function loadLists() {
    const grid = document.getElementById('lists-grid');
    grid.innerHTML = '<div class="loading">Loading lists...</div>';

    // Reset to grid view
    document.getElementById('lists-grid-view').style.display = '';
    document.getElementById('list-members-view').style.display = 'none';

    try {
        const stats = await window.go.main.App.GetListsStats();
        updateStatsDisplay(stats, 'lists');

        const lists = await window.go.main.App.GetOwnedLists();
        if (!lists || lists.length === 0) {
            grid.innerHTML = '<div class="loading">No lists found. Click Fetch Now to start.</div>';
            return;
        }

        grid.innerHTML = lists.map(l => `
            <div class="list-card" onclick="viewListMembers('${l.id}', '${escapeHtml(l.name)}')">
                <div class="list-card-name">${escapeHtml(l.name)}${l.private ? ' <span class="list-private-badge">Private</span>' : ''}</div>
                <div class="list-card-desc">${escapeHtml(l.description)}</div>
                <div class="list-card-meta">${l.member_count} members</div>
            </div>
        `).join('');
    } catch (err) {
        console.error('Error loading lists:', err);
        grid.innerHTML = '<div class="loading">Error loading lists</div>';
    }
}

async function viewListMembers(listId, listName) {
    document.getElementById('lists-grid-view').style.display = 'none';
    document.getElementById('list-members-view').style.display = '';
    document.getElementById('list-members-title').textContent = listName;
    document.getElementById('list-members-body').innerHTML =
        '<tr><td colspan="8" class="loading">Loading members...</td></tr>';
    document.getElementById('list-members-search').value = '';

    try {
        const members = await window.go.main.App.GetListMembers(listId);
        allListMembers = members || [];
        renderListMembers(allListMembers);
    } catch (err) {
        console.error('Error loading list members:', err);
        document.getElementById('list-members-body').innerHTML =
            '<tr><td colspan="8" class="loading">Error loading members</td></tr>';
    }
}

function backToLists() {
    document.getElementById('list-members-view').style.display = 'none';
    document.getElementById('lists-grid-view').style.display = '';
    allListMembers = [];
}

function renderListBadges(lists) {
    if (!lists || lists.length === 0) return '';
    return lists.map(l => `<span class="list-badge">${escapeHtml(l)}</span>`).join(' ');
}

function renderListMembers(users) {
    const tbody = document.getElementById('list-members-body');
    if (!users || users.length === 0) {
        tbody.innerHTML = '<tr><td colspan="8" class="loading">No members</td></tr>';
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
            <td class="lists-cell">${renderListBadges(u.lists)}</td>
        </tr>
    `).join('');
}

function filterListMembers() {
    const query = document.getElementById('list-members-search').value.toLowerCase();
    const filtered = allListMembers.filter(u =>
        u.username.toLowerCase().includes(query) ||
        u.name.toLowerCase().includes(query) ||
        (u.description && u.description.toLowerCase().includes(query))
    );
    renderListMembers(filtered);
}

// --- Utilities ---

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

// --- Initial load ---

document.addEventListener('DOMContentLoaded', () => {
    setTimeout(async () => {
        await loadAccounts();
        await loadData();
    }, 500);
});

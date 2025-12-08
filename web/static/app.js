let authToken = localStorage.getItem('authToken');
let currentUser = localStorage.getItem('currentUser');

// Initialize app
document.addEventListener('DOMContentLoaded', () => {
    if (authToken && currentUser) {
        showMainSection();
        loadUserUrls();
    }
});

function showToast(message) {
    const toast = document.getElementById('toast');
    toast.textContent = message;
    toast.classList.add('show');
    setTimeout(() => {
        toast.classList.remove('show');
    }, 3000);
}

async function login() {
    const username = document.getElementById('username').value.trim();

    if (!username) {
        showToast('Please enter a username');
        return;
    }

    try {
        const response = await fetch('/api/login', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ username })
        });

        if (!response.ok) {
            throw new Error('Login failed');
        }

        const data = await response.json();
        authToken = data.token;
        currentUser = data.user_id;

        localStorage.setItem('authToken', authToken);
        localStorage.setItem('currentUser', currentUser);

        showMainSection();
        loadUserUrls();
        showToast('Login successful!');
    } catch (error) {
        showToast('Login failed: ' + error.message);
    }
}

function logout() {
    authToken = null;
    currentUser = null;
    localStorage.removeItem('authToken');
    localStorage.removeItem('currentUser');

    document.getElementById('loginSection').style.display = 'block';
    document.getElementById('mainSection').style.display = 'none';
    document.getElementById('username').value = '';
    showToast('Logged out successfully');
}

function showMainSection() {
    document.getElementById('loginSection').style.display = 'none';
    document.getElementById('mainSection').style.display = 'block';
    document.getElementById('currentUser').textContent = currentUser;
}

async function createShortUrl() {
    const originalUrl = document.getElementById('originalUrl').value.trim();
    const customAlias = document.getElementById('customAlias').value.trim();

    if (!originalUrl) {
        showToast('Please enter a URL');
        return;
    }

    // Basic URL validation
    if (!originalUrl.startsWith('http://') && !originalUrl.startsWith('https://')) {
        showToast('URL must start with http:// or https://');
        return;
    }

    try {
        const response = await fetch('/api/shorten', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${authToken}`
            },
            body: JSON.stringify({
                url: originalUrl,
                custom_alias: customAlias
            })
        });

        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(errorText || 'Failed to create short URL');
        }

        const data = await response.json();

        // Show result
        document.getElementById('shortUrl').value = data.short_url;
        document.getElementById('result').style.display = 'block';

        // Clear inputs
        document.getElementById('originalUrl').value = '';
        document.getElementById('customAlias').value = '';

        showToast('Short URL created successfully!');

        // Refresh URL list
        setTimeout(() => loadUserUrls(), 500);
    } catch (error) {
        showToast('Error: ' + error.message);
    }
}

async function loadUserUrls() {
    try {
        const response = await fetch('/api/urls', {
            method: 'GET',
            headers: {
                'Authorization': `Bearer ${authToken}`
            }
        });

        if (!response.ok) {
            throw new Error('Failed to load URLs');
        }

        const data = await response.json();
        displayUrls(data.urls || []);
    } catch (error) {
        showToast('Error loading URLs: ' + error.message);
    }
}

function displayUrls(urls) {
    const urlList = document.getElementById('urlList');

    if (urls.length === 0) {
        urlList.innerHTML = '<p style="text-align: center; color: #999;">No URLs yet. Create your first short URL!</p>';
        return;
    }

    urlList.innerHTML = urls.map(url => `
        <div class="url-item">
            <div class="url-item-header">
                <a href="${url.short_url}" class="url-short" target="_blank">${url.short_url}</a>
                <button class="btn-secondary" onclick="loadStats('${url.short_code}')">Stats</button>
            </div>
            <div class="url-original">${url.original_url}</div>
            <div class="url-stats">
                Created: ${new Date(url.created_at * 1000).toLocaleString()}
                <span id="stats-${url.short_code}"></span>
            </div>
        </div>
    `).join('');
}

async function loadStats(shortCode) {
    try {
        const response = await fetch(`/api/stats?code=${shortCode}`, {
            method: 'GET'
        });

        if (!response.ok) {
            throw new Error('Failed to load stats');
        }

        const data = await response.json();
        const statsElement = document.getElementById(`stats-${shortCode}`);

        if (statsElement) {
            statsElement.innerHTML = ` | Clicks: ${data.stats.total_clicks} (${data.stats.unique_clicks} unique)`;
        }

        showToast(`Stats loaded: ${data.stats.total_clicks} total clicks`);
    } catch (error) {
        showToast('Error loading stats: ' + error.message);
    }
}

function copyToClipboard() {
    const shortUrl = document.getElementById('shortUrl');
    shortUrl.select();
    shortUrl.setSelectionRange(0, 99999); // For mobile devices

    try {
        document.execCommand('copy');
        showToast('Copied to clipboard!');
    } catch (err) {
        // Fallback for modern browsers
        navigator.clipboard.writeText(shortUrl.value).then(() => {
            showToast('Copied to clipboard!');
        }).catch(() => {
            showToast('Failed to copy');
        });
    }
}

// Handle Enter key in login
document.addEventListener('DOMContentLoaded', () => {
    const usernameInput = document.getElementById('username');
    if (usernameInput) {
        usernameInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                login();
            }
        });
    }
});

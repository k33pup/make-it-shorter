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

function showRegisterForm() {
    document.getElementById('loginForm').classList.add('hidden');
    document.getElementById('registerForm').classList.remove('hidden');
    document.getElementById('authTitle').textContent = 'Register';
}

function showLoginForm() {
    document.getElementById('registerForm').classList.add('hidden');
    document.getElementById('loginForm').classList.remove('hidden');
    document.getElementById('authTitle').textContent = 'Login';
}

async function register() {
    const username = document.getElementById('regUsername').value.trim();
    const password = document.getElementById('regPassword').value.trim();
    const passwordConfirm = document.getElementById('regPasswordConfirm').value.trim();

    if (!username || !password) {
        showToast('Username and password are required');
        return;
    }

    if (username.length < 3 || username.length > 50) {
        showToast('Username must be between 3 and 50 characters');
        return;
    }

    if (password.length < 6) {
        showToast('Password must be at least 6 characters');
        return;
    }

    if (password !== passwordConfirm) {
        showToast('Passwords do not match');
        return;
    }

    try {
        const response = await fetch('/api/register', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ username, password })
        });

        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(errorText || 'Registration failed');
        }

        const data = await response.json();
        authToken = data.token;
        currentUser = data.user_id;

        localStorage.setItem('authToken', authToken);
        localStorage.setItem('currentUser', currentUser);

        // Clear form
        document.getElementById('regUsername').value = '';
        document.getElementById('regPassword').value = '';
        document.getElementById('regPasswordConfirm').value = '';

        showMainSection();
        loadUserUrls();
        showToast('Registration successful! Welcome ' + currentUser + '!');
    } catch (error) {
        showToast('Registration failed: ' + error.message);
    }
}

async function login() {
    const username = document.getElementById('username').value.trim();
    const password = document.getElementById('password').value.trim();

    if (!username || !password) {
        showToast('Please enter username and password');
        return;
    }

    try {
        const response = await fetch('/api/login', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ username, password })
        });

        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(errorText || 'Login failed');
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

    document.getElementById('authSection').classList.remove('hidden');
    document.getElementById('mainSection').classList.add('hidden');
    showLoginForm();
    document.getElementById('username').value = '';
    document.getElementById('password').value = '';
    showToast('Logged out successfully');
}

function showMainSection() {
    document.getElementById('authSection').classList.add('hidden');
    document.getElementById('mainSection').classList.remove('hidden');
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
        document.getElementById('result').classList.remove('hidden');

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
        urlList.innerHTML = '<p class="no-urls-message">No URLs yet. Create your first short URL!</p>';
        return;
    }

    urlList.innerHTML = urls.map(url => `
        <div class="url-item">
            <div class="url-item-header">
                <a href="${url.short_url}" class="url-short" target="_blank">${url.short_url}</a>
                <button class="btn-secondary stats-btn" data-shortcode="${url.short_code}">Stats</button>
            </div>
            <div class="url-original">${url.original_url}</div>
            <div class="url-stats">
                Created: ${new Date(url.created_at * 1000).toLocaleString()}
                <span id="stats-${url.short_code}"></span>
            </div>
        </div>
    `).join('');

    // Add event listeners to all stats buttons
    document.querySelectorAll('.stats-btn').forEach(btn => {
        btn.addEventListener('click', function() {
            loadStats(this.dataset.shortcode);
        });
    });
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

// Setup event listeners
document.addEventListener('DOMContentLoaded', () => {
    // Login button
    const loginBtn = document.getElementById('loginBtn');
    if (loginBtn) {
        loginBtn.addEventListener('click', login);
    }

    // Register button
    const registerBtn = document.getElementById('registerBtn');
    if (registerBtn) {
        registerBtn.addEventListener('click', register);
    }

    // Show register form link
    const showRegisterLink = document.getElementById('showRegisterLink');
    if (showRegisterLink) {
        showRegisterLink.addEventListener('click', (e) => {
            e.preventDefault();
            showRegisterForm();
        });
    }

    // Show login form link
    const showLoginLink = document.getElementById('showLoginLink');
    if (showLoginLink) {
        showLoginLink.addEventListener('click', (e) => {
            e.preventDefault();
            showLoginForm();
        });
    }

    // Logout button
    const logoutBtn = document.getElementById('logoutBtn');
    if (logoutBtn) {
        logoutBtn.addEventListener('click', logout);
    }

    // Shorten URL button
    const shortenBtn = document.getElementById('shortenBtn');
    if (shortenBtn) {
        shortenBtn.addEventListener('click', createShortUrl);
    }

    // Copy button
    const copyBtn = document.getElementById('copyBtn');
    if (copyBtn) {
        copyBtn.addEventListener('click', copyToClipboard);
    }

    // Refresh URLs button
    const refreshBtn = document.getElementById('refreshBtn');
    if (refreshBtn) {
        refreshBtn.addEventListener('click', loadUserUrls);
    }

    // Handle Enter key in login form
    const usernameInput = document.getElementById('username');
    const passwordInput = document.getElementById('password');

    if (usernameInput) {
        usernameInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                login();
            }
        });
    }

    if (passwordInput) {
        passwordInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                login();
            }
        });
    }

    // Handle Enter key in registration form
    const regUsernameInput = document.getElementById('regUsername');
    const regPasswordInput = document.getElementById('regPassword');
    const regPasswordConfirmInput = document.getElementById('regPasswordConfirm');

    if (regUsernameInput) {
        regUsernameInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                register();
            }
        });
    }

    if (regPasswordInput) {
        regPasswordInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                register();
            }
        });
    }

    if (regPasswordConfirmInput) {
        regPasswordConfirmInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                register();
            }
        });
    }

    // Handle Enter key in URL form
    const originalUrlInput = document.getElementById('originalUrl');
    const customAliasInput = document.getElementById('customAlias');

    if (originalUrlInput) {
        originalUrlInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                createShortUrl();
            }
        });
    }

    if (customAliasInput) {
        customAliasInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                createShortUrl();
            }
        });
    }
});

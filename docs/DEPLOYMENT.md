# Deployment Guide: Distributed URL Shortener

This guide provides detailed instructions for deploying the URL Shortener application both locally and on a remote server.

## Table of Contents

1. [Local Deployment](#1-local-deployment)
2. [Remote Server Deployment](#2-remote-server-deployment)
3. [Production Configuration](#3-production-configuration)
4. [Troubleshooting](#4-troubleshooting)
5. [Maintenance](#5-maintenance)

---

## 1. Local Deployment

### Prerequisites

- Docker (20.10+)
- Docker Compose (2.0+)
- Go 1.21+ (for development)
- Git

### Installation

#### 1.1 Install Docker

**macOS:**
```bash
# Download Docker Desktop from https://www.docker.com/products/docker-desktop
# Or use Homebrew
brew install --cask docker
```

**Linux (Ubuntu/Debian):**
```bash
# Update package index
sudo apt-get update

# Install dependencies
sudo apt-get install -y ca-certificates curl gnupg lsb-release

# Add Docker's official GPG key
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg

# Set up repository
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# Install Docker Engine
sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

# Add your user to docker group
sudo usermod -aG docker $USER
newgrp docker
```

**Windows:**
```
Download and install Docker Desktop from:
https://www.docker.com/products/docker-desktop
```

#### 1.2 Verify Installation

```bash
docker --version
# Expected: Docker version 20.10.0 or higher

docker-compose --version
# Expected: Docker Compose version 2.0.0 or higher
```

### Building and Running Locally

#### Option 1: Using Docker Compose (Recommended)

```bash
# Clone the repository (if not already done)
git clone <repository-url>
cd network

# Build and start all services
docker-compose up --build

# Or run in detached mode
docker-compose up --build -d

# View logs
docker-compose logs -f

# View logs for specific service
docker-compose logs -f gateway
docker-compose logs -f urlservice
docker-compose logs -f analytics
```

#### Option 2: Using Make

```bash
# Build images
make build

# Run services
make run

# Development mode (build + run)
make dev

# Stop services
make stop

# View logs
make logs
```

### Accessing the Application

Once all services are running:

1. **Web Interface:** http://localhost:8080
2. **API Gateway:** http://localhost:8080/api/*
3. **Health Check:** http://localhost:8080

**Test the application:**

```bash
# Health check
curl http://localhost:8080

# Login
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"username": "testuser"}'

# Save the token from response
TOKEN="eyJhbGciOiJIUzI1NiIs..."

# Create short URL
curl -X POST http://localhost:8080/api/shorten \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"url": "https://www.example.com/very/long/url"}'

# Get user URLs
curl http://localhost:8080/api/urls \
  -H "Authorization: Bearer $TOKEN"

# Get statistics
curl "http://localhost:8080/api/stats?code=abc123"
```

### Stopping the Application

```bash
# Stop services (keeps data)
docker-compose down

# Stop and remove all data
docker-compose down -v

# Or use Make
make stop
make clean
```

---

## 2. Remote Server Deployment

### 2.1 Choose a Server Provider

**Recommended providers:**

1. **DigitalOcean** (Simple, $6/month)
   - Go to: https://www.digitalocean.com
   - Create Droplet → Ubuntu 22.04 LTS
   - Choose: Basic plan ($6/mo, 1GB RAM, 1 CPU)

2. **Hetzner** (Cheap, €4.51/month)
   - Go to: https://www.hetzner.com
   - Cloud → Create Server → Ubuntu 22.04
   - Choose: CX11 (€4.51/mo, 2GB RAM, 1 vCPU)

3. **AWS EC2** (Flexible, free tier available)
   - Go to: https://aws.amazon.com
   - EC2 → Launch Instance → Ubuntu 22.04
   - Choose: t2.micro (free tier)

4. **Google Cloud** (Free tier available)
   - Go to: https://cloud.google.com
   - Compute Engine → Create Instance
   - Choose: e2-micro (free tier)

### 2.2 Initial Server Setup

#### Connect to Server

```bash
# Replace with your server IP
ssh root@YOUR_SERVER_IP

# Or if using a key
ssh -i ~/.ssh/your_key.pem ubuntu@YOUR_SERVER_IP
```

#### Update System

```bash
# Update package lists
sudo apt-get update

# Upgrade packages
sudo apt-get upgrade -y

# Install essential tools
sudo apt-get install -y curl git ufw
```

#### Setup Firewall

```bash
# Allow SSH
sudo ufw allow 22/tcp

# Allow HTTP
sudo ufw allow 80/tcp

# Allow HTTPS
sudo ufw allow 443/tcp

# Enable firewall
sudo ufw --force enable

# Check status
sudo ufw status
```

#### Create Non-Root User (Recommended)

```bash
# Create user
sudo adduser deployer

# Add to sudo group
sudo usermod -aG sudo deployer

# Switch to new user
su - deployer
```

### 2.3 Install Docker on Server

```bash
# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Add user to docker group
sudo usermod -aG docker $USER

# Apply group changes
newgrp docker

# Install Docker Compose
sudo apt-get install -y docker-compose-plugin

# Verify installation
docker --version
docker compose version
```

### 2.4 Deploy Application

#### Clone Repository

```bash
# Create application directory
mkdir -p ~/apps
cd ~/apps

# Clone repository
git clone <repository-url> url-shortener
cd url-shortener

# Or upload files using SCP
# From your local machine:
# scp -r ./network user@YOUR_SERVER_IP:~/apps/url-shortener
```

#### Configure Environment

```bash
# Create .env file for production
cat > .env << EOF
JWT_SECRET=$(openssl rand -hex 32)
REDIS_PASSWORD=$(openssl rand -hex 16)
DOMAIN=your-domain.com
EOF

# Protect .env file
chmod 600 .env
```

#### Build and Run

```bash
# Build all services
docker compose build

# Start services in detached mode
docker compose up -d

# Check status
docker compose ps

# View logs
docker compose logs -f
```

#### Verify Services

```bash
# Check if all containers are running
docker ps

# Expected output: 4 containers
# - url_shortener_gateway
# - url_shortener_urlservice
# - url_shortener_analytics
# - url_shortener_redis

# Test locally on server
curl http://localhost:8080

# Check logs
docker compose logs gateway | tail -20
docker compose logs urlservice | tail -20
docker compose logs analytics | tail -20
```

### 2.5 Setup Domain and DNS

#### Configure DNS Records

1. Go to your domain registrar (Namecheap, GoDaddy, etc.)
2. Add DNS A record:
   - **Type:** A
   - **Host:** @ (or subdomain like "short")
   - **Value:** YOUR_SERVER_IP
   - **TTL:** 300

3. Wait for DNS propagation (5-30 minutes)

4. Verify DNS:
```bash
# On your local machine
nslookup your-domain.com
dig your-domain.com
```

### 2.6 Setup HTTPS with Let's Encrypt

#### Install Certbot

```bash
# Install Certbot
sudo apt-get install -y certbot

# Stop gateway temporarily
docker compose stop gateway
```

#### Obtain Certificate

```bash
# Get certificate
sudo certbot certonly --standalone -d your-domain.com

# Certificate files will be at:
# /etc/letsencrypt/live/your-domain.com/fullchain.pem
# /etc/letsencrypt/live/your-domain.com/privkey.pem
```

#### Configure Nginx Reverse Proxy

Instead of modifying the Go code, use Nginx:

```bash
# Install Nginx
sudo apt-get install -y nginx

# Create Nginx configuration
sudo nano /etc/nginx/sites-available/url-shortener
```

**Nginx Configuration:**

```nginx
server {
    listen 80;
    server_name your-domain.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name your-domain.com;

    # SSL Configuration
    ssl_certificate /etc/letsencrypt/live/your-domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/your-domain.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    # Security Headers
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Frame-Options "DENY" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # Rate Limiting
    limit_req_zone $binary_remote_addr zone=api_limit:10m rate=10r/s;
    limit_req zone=api_limit burst=20 nodelay;

    # Proxy to Docker container
    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
    }

    # Logging
    access_log /var/log/nginx/url-shortener-access.log;
    error_log /var/log/nginx/url-shortener-error.log;
}
```

**Enable Configuration:**

```bash
# Enable site
sudo ln -s /etc/nginx/sites-available/url-shortener /etc/nginx/sites-enabled/

# Remove default site
sudo rm /etc/nginx/sites-enabled/default

# Test configuration
sudo nginx -t

# Restart Nginx
sudo systemctl restart nginx

# Enable Nginx on boot
sudo systemctl enable nginx
```

#### Restart Gateway

```bash
cd ~/apps/url-shortener
docker compose up -d
```

#### Setup Auto-Renewal

```bash
# Test renewal
sudo certbot renew --dry-run

# Certbot automatically creates a cron job
# Verify:
sudo systemctl status certbot.timer
```

### 2.7 Configure Docker Compose for Production

Update `docker-compose.yml`:

```yaml
version: '3.8'

services:
  redis:
    image: redis:7-alpine
    container_name: url_shortener_redis
    command: redis-server --requirepass ${REDIS_PASSWORD}
    volumes:
      - redis_data:/data
    networks:
      - urlshortener
    restart: always

  urlservice:
    build:
      context: .
      dockerfile: services/urlservice/Dockerfile
    container_name: url_shortener_urlservice
    depends_on:
      - redis
    networks:
      - urlshortener
    environment:
      - REDIS_ADDR=redis:6379
      - REDIS_PASSWORD=${REDIS_PASSWORD}
    restart: always

  analytics:
    build:
      context: .
      dockerfile: services/analytics/Dockerfile
    container_name: url_shortener_analytics
    depends_on:
      - redis
    networks:
      - urlshortener
    environment:
      - REDIS_ADDR=redis:6379
      - REDIS_PASSWORD=${REDIS_PASSWORD}
    restart: always

  gateway:
    build:
      context: .
      dockerfile: services/gateway/Dockerfile
    container_name: url_shortener_gateway
    ports:
      - "127.0.0.1:8080:8080"  # Only bind to localhost
    depends_on:
      - urlservice
      - analytics
      - redis
    networks:
      - urlshortener
    environment:
      - REDIS_ADDR=redis:6379
      - REDIS_PASSWORD=${REDIS_PASSWORD}
      - JWT_SECRET=${JWT_SECRET}
      - DOMAIN=${DOMAIN}
    restart: always

networks:
  urlshortener:
    driver: bridge

volumes:
  redis_data:
```

Apply changes:

```bash
docker compose down
docker compose up -d --build
```

### 2.8 Setup Monitoring

#### Install monitoring tools

```bash
# Install htop for system monitoring
sudo apt-get install -y htop

# View system resources
htop
```

#### Monitor Docker

```bash
# View container stats
docker stats

# View logs
docker compose logs -f --tail=100

# Check container health
docker compose ps
```

#### Setup Log Rotation

```bash
# Create logrotate config
sudo nano /etc/logrotate.d/docker-containers

# Add:
/var/lib/docker/containers/*/*.log {
    rotate 7
    daily
    compress
    missingok
    delaycompress
    copytruncate
}
```

---

## 3. Production Configuration

### 3.1 Environment Variables

Create `.env` file with production values:

```bash
# Security
JWT_SECRET=<generate-with-openssl-rand-hex-32>
REDIS_PASSWORD=<generate-with-openssl-rand-hex-16>

# Domain
DOMAIN=your-domain.com

# Redis
REDIS_ADDR=redis:6379
REDIS_MAX_RETRIES=3

# Rate Limiting
RATE_LIMIT_REQUESTS=100
RATE_LIMIT_WINDOW=60

# Logging
LOG_LEVEL=info
```

### 3.2 Security Hardening

#### Update Server Regularly

```bash
# Create update script
cat > ~/update.sh << 'EOF'
#!/bin/bash
sudo apt-get update
sudo apt-get upgrade -y
sudo apt-get autoremove -y
docker system prune -af
EOF

chmod +x ~/update.sh

# Run weekly via cron
crontab -e
# Add: 0 2 * * 0 ~/update.sh
```

#### Setup Fail2Ban

```bash
# Install Fail2Ban
sudo apt-get install -y fail2ban

# Configure
sudo nano /etc/fail2ban/jail.local
```

Add:
```ini
[sshd]
enabled = true
port = 22
maxretry = 3
bantime = 3600

[nginx-limit-req]
enabled = true
port = http,https
logpath = /var/log/nginx/*error.log
maxretry = 5
```

```bash
# Start Fail2Ban
sudo systemctl enable fail2ban
sudo systemctl start fail2ban

# Check status
sudo fail2ban-client status
```

### 3.3 Backup Strategy

#### Backup Script

```bash
# Create backup directory
mkdir -p ~/backups

# Create backup script
cat > ~/backup.sh << 'EOF'
#!/bin/bash
BACKUP_DIR=~/backups
DATE=$(date +%Y%m%d_%H%M%S)

# Backup Redis data
docker exec url_shortener_redis redis-cli -a $REDIS_PASSWORD --rdb /data/backup.rdb
docker cp url_shortener_redis:/data/backup.rdb $BACKUP_DIR/redis_$DATE.rdb

# Backup application code
tar -czf $BACKUP_DIR/app_$DATE.tar.gz ~/apps/url-shortener

# Keep only last 7 backups
find $BACKUP_DIR -name "*.rdb" -mtime +7 -delete
find $BACKUP_DIR -name "*.tar.gz" -mtime +7 -delete

echo "Backup completed: $DATE"
EOF

chmod +x ~/backup.sh

# Schedule daily backups
crontab -e
# Add: 0 3 * * * ~/backup.sh
```

---

## 4. Troubleshooting

### Common Issues

#### Services Not Starting

```bash
# Check logs
docker compose logs

# Check specific service
docker compose logs gateway

# Restart services
docker compose restart

# Rebuild if needed
docker compose up -d --build
```

#### Port Already in Use

```bash
# Find process using port
sudo lsof -i :8080
sudo netstat -tulpn | grep 8080

# Kill process
sudo kill -9 <PID>
```

#### Redis Connection Issues

```bash
# Check Redis is running
docker compose ps redis

# Test Redis connection
docker exec -it url_shortener_redis redis-cli ping

# Check Redis logs
docker compose logs redis
```

#### DNS Not Resolving

```bash
# Check DNS propagation
nslookup your-domain.com
dig your-domain.com

# Clear DNS cache (local)
sudo systemd-resolve --flush-caches
```

#### SSL Certificate Issues

```bash
# Test certificate
sudo certbot certificates

# Renew certificate
sudo certbot renew

# Check Nginx error logs
sudo tail -f /var/log/nginx/error.log
```

### Performance Issues

```bash
# Check system resources
htop
free -h
df -h

# Check Docker resources
docker stats

# Increase Docker memory limit (if needed)
# Edit /etc/docker/daemon.json
{
  "default-ulimits": {
    "nofile": {
      "Name": "nofile",
      "Hard": 64000,
      "Soft": 64000
    }
  }
}

sudo systemctl restart docker
```

---

## 5. Maintenance

### Regular Tasks

#### Daily

```bash
# Check logs for errors
docker compose logs --tail=100 | grep -i error

# Monitor disk usage
df -h
```

#### Weekly

```bash
# Update system packages
sudo apt-get update && sudo apt-get upgrade -y

# Clean Docker
docker system prune -f

# Check backups
ls -lh ~/backups
```

#### Monthly

```bash
# Review Nginx logs
sudo tail -1000 /var/log/nginx/access.log | less

# Review firewall rules
sudo ufw status verbose

# Update Docker images
docker compose pull
docker compose up -d
```

### Scaling

#### Horizontal Scaling

```bash
# Scale URL service to 3 instances
docker compose up -d --scale urlservice=3

# Scale Analytics service
docker compose up -d --scale analytics=2
```

#### Load Balancing

Use Nginx upstream:

```nginx
upstream backend {
    least_conn;
    server localhost:8080;
    server localhost:8081;
    server localhost:8082;
}

server {
    location / {
        proxy_pass http://backend;
    }
}
```

### Updating the Application

```bash
# Pull latest changes
cd ~/apps/url-shortener
git pull origin main

# Rebuild and restart
docker compose up -d --build

# Verify
docker compose ps
curl https://your-domain.com
```

---

## Quick Reference

### Essential Commands

```bash
# Start services
docker compose up -d

# Stop services
docker compose down

# View logs
docker compose logs -f

# Restart service
docker compose restart gateway

# Rebuild service
docker compose up -d --build gateway

# Check status
docker compose ps

# Execute command in container
docker compose exec gateway sh
```

### Useful URLs

- **Application:** https://your-domain.com
- **Server:** ssh user@YOUR_SERVER_IP
- **DNS Check:** https://dnschecker.org
- **SSL Test:** https://www.ssllabs.com/ssltest/

### Emergency Contacts

- Server Provider Support
- Domain Registrar Support
- Your team members

---

## Conclusion

Your URL Shortener is now deployed and accessible on the internet! Remember to:

1. Regularly update your server and application
2. Monitor logs and system resources
3. Keep backups
4. Review security practices
5. Monitor uptime and performance

For additional help, refer to:
- Docker documentation: https://docs.docker.com
- Nginx documentation: https://nginx.org/en/docs/
- Let's Encrypt: https://letsencrypt.org/docs/

# Distributed URL Shortener

A distributed URL shortening service built with Go microservices architecture.

> **⚠️ Security Notice:** Before deploying, please review [SECURITY_FIXES.md](SECURITY_FIXES.md) for critical security configurations required.

## Architecture

- **API Gateway** (port 8080) - HTTP API for frontend
- **URL Service** (port 8081) - URL creation and storage
- **Analytics Service** (port 8082) - Click statistics and analytics
- **PostgreSQL** (port 5433) - User database
- **Redis** (port 6379) - Caching and rate limiting
- **Frontend** - Simple web interface

## Quick Start

### Prerequisites
1. Copy `.env.example` to `.env`:
```bash
cp .env.example .env
```

2. **REQUIRED:** Edit `.env` and set a strong `JWT_SECRET`:
```bash
# Generate a strong secret (Linux/Mac)
openssl rand -base64 32

# Then paste it in .env file
JWT_SECRET=<your-generated-secret>
```

### Start the Application

```bash
# For local testing (without nginx)
docker-compose -f docker-compose.local.yml up --build

# For production (with nginx and SSL)
docker-compose up --build

# Access the application
open http://localhost:8080
```

## Authentication

### Registration
New users can register via the web interface or API:
```bash
curl -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{"username":"yourname","password":"yourpass"}'
```

Requirements:
- Username: 3-50 characters, must be unique
- Password: minimum 6 characters

### Test Credentials

Pre-created test accounts:

- **Admin user:**
  - Username: `admin`
  - Password: `admin123`

- **Regular user:**
  - Username: `user`
  - Password: `user123`

## Services Communication

- Frontend ↔ API Gateway: HTTP/REST
- API Gateway ↔ URL Service: gRPC
- API Gateway ↔ Analytics Service: gRPC
- API Gateway ↔ PostgreSQL: SQL
- All services ↔ Redis: Redis protocol

## Security Features

- User registration system
- Duplicate username prevention
- JWT authentication with username and password
- Password hashing with bcrypt (cost factor 10)
- PostgreSQL database for secure user storage
- Rate limiting (100 requests per minute per IP)
- Input validation and sanitization
- HTTPS ready with TLS support
- SQL injection prevention with parameterized queries
- XSS protection

## Documentation

- [Network Architecture & Security](./docs/NETWORK.md)
- [Deployment Guide](./docs/DEPLOYMENT.md)

## Tech Stack

- Go 1.23+
- gRPC & Protocol Buffers
- PostgreSQL 15
- Redis 7
- Docker & Docker Compose
- HTML/CSS/JavaScript (Vanilla)
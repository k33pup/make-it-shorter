# Distributed URL Shortener

A distributed URL shortening service built with Go microservices architecture.

## Architecture

- **API Gateway** (port 8080) - HTTP API for frontend
- **URL Service** (port 8081) - URL creation and storage
- **Analytics Service** (port 8082) - Click statistics and analytics
- **Redis** (port 6379) - Caching and rate limiting
- **Frontend** - Simple web interface

## Quick Start

```bash
# Build and run all services
docker-compose up --build

# Access the application
open http://localhost:8080
```

## Services Communication

- Frontend ↔ API Gateway: HTTP/REST
- API Gateway ↔ URL Service: gRPC
- API Gateway ↔ Analytics Service: gRPC
- All services ↔ Redis: Redis protocol

## Security Features

- JWT authentication
- Rate limiting (10 requests per minute per IP)
- Input validation and sanitization
- HTTPS ready with TLS support
- SQL injection prevention
- XSS protection

## Documentation

- [Network Architecture & Security](./docs/NETWORK.md)
- [Deployment Guide](./docs/DEPLOYMENT.md)

## Tech Stack

- Go 1.21+
- gRPC
- Redis
- Docker & Docker Compose
- HTML/CSS/JavaScript
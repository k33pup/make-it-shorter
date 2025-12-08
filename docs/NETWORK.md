# Network Architecture & Security Documentation

## 1. System Overview

The URL Shortener is built using a distributed microservices architecture with three main services communicating over the network:

```
┌──────────┐
│  Client  │
│ (Browser)│
└────┬─────┘
     │ HTTP/HTTPS
     │ Port 8080
     ▼
┌─────────────────┐
│  API Gateway    │◄────────┐
│  Port: 8080     │         │
│  Protocol: HTTP │         │
└────┬────────────┘         │
     │                      │ Redis Protocol
     │ gRPC                 │ Port: 6379
     │                      │
     ├──────────────────────┼──────────────┐
     │                      │              │
     ▼                      ▼              ▼
┌─────────────┐      ┌─────────────┐   ┌────────┐
│ URL Service │      │ Analytics   │   │ Redis  │
│ Port: 8081  │      │ Service     │   │ Cache  │
│ Proto: gRPC │      │ Port: 8082  │   │        │
└─────────────┘      │ Proto: gRPC │   └────────┘
                     └─────────────┘
```

## 2. Network Communication Protocols

### 2.1 Frontend ↔ API Gateway (HTTP/REST)

**Protocol:** HTTP/1.1 (upgradeable to HTTPS with TLS)

**Endpoints:**

| Method | Endpoint | Description | Auth Required |
|--------|----------|-------------|---------------|
| POST | `/api/login` | User authentication | No |
| POST | `/api/shorten` | Create short URL | Yes (JWT) |
| GET | `/api/urls` | Get user's URLs | Yes (JWT) |
| GET | `/api/stats?code={code}` | Get click statistics | No |
| GET | `/s/{code}` | Redirect to original URL | No |
| GET | `/` | Serve static files | No |

**Request/Response Format:**

```
POST /api/shorten HTTP/1.1
Host: localhost:8080
Content-Type: application/json
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...

{
  "url": "https://example.com/very/long/url",
  "custom_alias": "mylink"
}

---

HTTP/1.1 200 OK
Content-Type: application/json
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block

{
  "short_code": "mylink",
  "short_url": "http://localhost:8080/s/mylink",
  "original_url": "https://example.com/very/long/url",
  "created_at": 1701936000
}
```

### 2.2 API Gateway ↔ URL Service (gRPC)

**Protocol:** gRPC over HTTP/2

**Network Details:**
- Transport: TCP
- Serialization: Protocol Buffers (protobuf)
- Connection: Persistent HTTP/2 connection with multiplexing
- Address: `urlservice:8081`

**RPC Methods:**

```protobuf
service URLService {
  rpc CreateShortURL(CreateShortURLRequest) returns (CreateShortURLResponse);
  rpc GetOriginalURL(GetOriginalURLRequest) returns (GetOriginalURLResponse);
  rpc GetUserURLs(GetUserURLsRequest) returns (GetUserURLsResponse);
}
```

**Network Flow Example:**

```
1. Gateway opens TCP connection to urlservice:8081
2. HTTP/2 connection established with ALPN negotiation
3. Gateway sends CreateShortURL RPC:
   - Binary protobuf payload (60-200 bytes typically)
   - Compressed with gzip
   - Multiplexed stream ID: 1
4. URL Service processes and responds:
   - Binary protobuf response (80-250 bytes)
   - Same stream ID: 1
5. Connection remains open for subsequent requests
```

### 2.3 API Gateway ↔ Analytics Service (gRPC)

**Protocol:** gRPC over HTTP/2

**Network Details:**
- Transport: TCP
- Address: `analytics:8082`
- Connection: Persistent

**RPC Methods:**

```protobuf
service AnalyticsService {
  rpc RecordClick(RecordClickRequest) returns (RecordClickResponse);
  rpc GetClickStats(GetClickStatsRequest) returns (GetClickStatsResponse);
}
```

**Async Communication Pattern:**

The RecordClick RPC is called asynchronously (in a goroutine) to avoid blocking the redirect response:

```go
go func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    analyticsClient.RecordClick(ctx, &pb.RecordClickRequest{...})
}()

// Immediately redirect user without waiting
http.Redirect(w, r, originalURL, http.StatusMovedPermanently)
```

### 2.4 Services ↔ Redis (Redis Protocol)

**Protocol:** RESP (Redis Serialization Protocol)

**Network Details:**
- Transport: TCP
- Port: 6379
- Connection: Connection pooling managed by go-redis client
- Default pool size: 10 connections per CPU

**Usage Patterns:**

1. **Rate Limiting:**
```
Key: rate_limit:{ip_address}
Commands:
- INCR rate_limit:192.168.1.1
- EXPIRE rate_limit:192.168.1.1 60
```

2. **URL Caching:**
```
Key: url:{short_code}
Commands:
- SET url:abc123 "https://example.com" EX 86400
- GET url:abc123
```

3. **Analytics:**
```
Keys:
- clicks:total:{short_code}    # Counter
- clicks:unique:{short_code}   # Set of IPs
- clicks:daily:{short_code}:{date}  # Daily counter

Commands:
- INCR clicks:total:abc123
- SADD clicks:unique:abc123 "192.168.1.1"
- INCR clicks:daily:abc123:2024-12-08
```

## 3. Network Security Measures

### 3.1 Authentication & Authorization

**JWT (JSON Web Tokens):**

```
Algorithm: HS256 (HMAC with SHA-256)
Secret: Configurable (default: "your-secret-key-change-in-production")
Expiry: 24 hours
Claims: {user_id, iat, exp}
```

**Security Properties:**
- Tokens are signed, preventing tampering
- Includes expiration time
- Validated on every authenticated request
- Stored client-side (localStorage)

**Token Flow:**

```
1. User sends credentials → Gateway
2. Gateway validates and generates JWT
3. JWT sent to client in response
4. Client includes JWT in Authorization header
5. Gateway validates JWT on each request
6. User ID extracted from valid token claims
```

### 3.2 Rate Limiting

**Implementation:** Redis-backed sliding window

**Configuration:**
- Window: 60 seconds
- Max requests: 100 per window per IP
- Granularity: Per IP address

**Algorithm:**

```
1. Extract client IP (X-Forwarded-For > X-Real-IP > RemoteAddr)
2. Key: rate_limit:{ip}
3. INCR rate_limit:{ip}
4. If count == 1: EXPIRE rate_limit:{ip} 60
5. If count > 100: Return 429 Too Many Requests
6. Set headers: X-RateLimit-Limit, X-RateLimit-Remaining
```

**Protection Against:**
- DoS attacks
- Brute force attempts
- Resource exhaustion
- API abuse

### 3.3 Input Validation & Sanitization

**URL Validation:**

```go
1. Check URL not empty and under 2048 chars
2. Parse URL structure (scheme, host, path)
3. Verify scheme is http or https only
4. Ensure host is present
5. Block localhost/127.0.0.1/0.0.0.0 (SSRF prevention)
6. Sanitize: remove <, >, ", ' characters
```

**Short Code Validation:**

```go
Regex: ^[a-zA-Z0-9_-]{3,10}$
- Only alphanumeric, dash, underscore
- Length: 3-10 characters
- Prevents: Path traversal, special characters, XSS
```

**Protection Against:**
- SSRF (Server-Side Request Forgery)
- XSS (Cross-Site Scripting)
- Path traversal
- SQL injection (even though we don't use SQL)
- Command injection

### 3.4 HTTP Security Headers

**Implemented Headers:**

```http
X-Content-Type-Options: nosniff
  → Prevents MIME-type sniffing attacks

X-Frame-Options: DENY
  → Prevents clickjacking by blocking iframe embedding

X-XSS-Protection: 1; mode=block
  → Enables browser XSS filter (legacy browsers)

Content-Security-Policy: default-src 'self'
  → Only load resources from same origin

Access-Control-Allow-Origin: *
  → CORS policy (configurable for production)
```

### 3.5 gRPC Security

**Current Implementation:** Insecure credentials (development)

**Production Recommendations:**

```go
// TLS credentials
creds, _ := credentials.NewClientTLSFromFile("server.crt", "")
conn, _ := grpc.Dial("urlservice:8081", grpc.WithTransportCredentials(creds))

// Mutual TLS (mTLS)
cert, _ := tls.LoadX509KeyPair("client.crt", "client.key")
creds := credentials.NewTLS(&tls.Config{
    Certificates: []tls.Certificate{cert},
})
```

**Benefits of mTLS:**
- Service-to-service authentication
- Encrypted communication
- Man-in-the-middle prevention

### 3.6 Redis Security

**Current Setup:** No authentication (internal network only)

**Production Recommendations:**

```go
redis.NewClient(&redis.Options{
    Addr:     "redis:6379",
    Password: os.Getenv("REDIS_PASSWORD"),
    TLS: &tls.Config{
        MinVersion: tls.VersionTLS12,
    },
})
```

**Additional Measures:**
- Enable Redis AUTH
- Use TLS for Redis connections
- Restrict Redis to internal network only
- Disable dangerous commands (FLUSHALL, CONFIG, etc.)

## 4. Network Attack Mitigation

### 4.1 DDoS Protection

**Implemented:**
- Rate limiting per IP (100 req/min)
- Timeout on all gRPC calls (5 seconds)
- Connection pooling to prevent exhaustion

**Additional Recommendations:**
- Use reverse proxy (Nginx/Caddy) with rate limiting
- Implement IP blacklisting
- Use CDN (Cloudflare) for additional protection
- Set up monitoring and alerts

### 4.2 SSRF Prevention

**Implemented:**
```go
// Block internal/localhost URLs
if strings.Contains(parsedURL.Host, "localhost") ||
   strings.Contains(parsedURL.Host, "127.0.0.1") ||
   strings.Contains(parsedURL.Host, "0.0.0.0") {
    return fmt.Errorf("localhost URLs are not allowed")
}
```

**Additional Protection:**
- Block private IP ranges (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
- Validate DNS responses
- Use allowlist for permitted domains

### 4.3 XSS Prevention

**Multiple Layers:**

1. **Input Sanitization:**
```go
input = strings.ReplaceAll(input, "<", "")
input = strings.ReplaceAll(input, ">", "")
input = strings.ReplaceAll(input, "\"", "")
input = strings.ReplaceAll(input, "'", "")
```

2. **Content-Type Headers:**
```http
Content-Type: application/json
X-Content-Type-Options: nosniff
```

3. **CSP Header:**
```http
Content-Security-Policy: default-src 'self'
```

### 4.4 Injection Attacks

**Prevention:**
- All data stored in memory (no SQL injection risk)
- Input validation with regex patterns
- Protobuf serialization (type-safe)
- No shell command execution
- No eval() or similar functions

## 5. Network Monitoring & Logging

**Implemented Logging:**

```
[Service] Action details
Examples:
- [Gateway] Received request: POST /api/shorten
- [URLService] CreateShortURL: url=https://example.com user=john
- [Analytics] RecordClick: code=abc123 ip=192.168.1.1
```

**Production Recommendations:**

1. **Structured Logging:**
   - JSON format
   - Include request IDs
   - Log levels (DEBUG, INFO, WARN, ERROR)

2. **Metrics Collection:**
   - Request latency
   - Error rates
   - Active connections
   - Redis hit/miss ratio

3. **Distributed Tracing:**
   - OpenTelemetry/Jaeger
   - Trace requests across services
   - Identify bottlenecks

4. **Alerting:**
   - High error rates
   - Unusual traffic patterns
   - Service downtime
   - Resource exhaustion

## 6. Network Configuration

**Docker Network:**

```yaml
networks:
  urlshortener:
    driver: bridge
```

**Network Isolation:**
- All services on same Docker network
- Only Gateway exposes ports externally (8080)
- Internal services (8081, 8082) not exposed to host
- Redis (6379) accessible only within Docker network

**Service Discovery:**
- DNS-based (Docker's embedded DNS)
- Service names resolve to container IPs
- Automatic health checks and restarts

## 7. HTTPS Configuration (Production)

**Step 1: Obtain SSL Certificate**

```bash
# Using Let's Encrypt
certbot certonly --standalone -d yourdomain.com
```

**Step 2: Update Gateway Code**

```go
func main() {
    // ... existing setup ...

    log.Println("API Gateway started on :8080 (HTTPS)")
    err := http.ListenAndServeTLS(
        ":8080",
        "/etc/letsencrypt/live/yourdomain.com/fullchain.pem",
        "/etc/letsencrypt/live/yourdomain.com/privkey.pem",
        handler,
    )
    if err != nil {
        log.Fatalf("Failed to start server: %v", err)
    }
}
```

**Step 3: Update Docker Compose**

```yaml
gateway:
  volumes:
    - /etc/letsencrypt:/etc/letsencrypt:ro
  ports:
    - "443:8080"
```

## 8. Security Checklist for Production

- [ ] Change JWT secret to strong random value
- [ ] Enable HTTPS with valid TLS certificates
- [ ] Implement mTLS for gRPC communication
- [ ] Add Redis authentication
- [ ] Set up firewall rules
- [ ] Implement IP allowlisting/blocklisting
- [ ] Add comprehensive logging
- [ ] Set up monitoring and alerting
- [ ] Regular security audits
- [ ] Keep dependencies updated
- [ ] Implement backup and disaster recovery
- [ ] Add CAPTCHA for public endpoints
- [ ] Implement user registration with email verification
- [ ] Add API usage quotas per user
- [ ] Set up intrusion detection system (IDS)

## 9. Network Performance Optimization

**Implemented:**
- gRPC for efficient binary serialization
- HTTP/2 multiplexing
- Redis caching with 24h TTL
- Connection pooling
- Async analytics recording

**Additional Recommendations:**
- CDN for static assets
- Database connection pooling
- Request/response compression
- Load balancing across instances
- Geographic distribution

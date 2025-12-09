# Quick Security Setup Guide

## Before First Run - REQUIRED STEPS

### 1. Generate JWT Secret
```bash
# Generate a strong random secret
openssl rand -base64 32
```

### 2. Create Environment File
```bash
# Copy example file
cp .env.example .env

# Edit .env and add your JWT_SECRET
nano .env  # or use your preferred editor
```

### 3. Minimum Required Configuration

Your `.env` file must contain:
```bash
JWT_SECRET=<paste-your-generated-secret-here>
```

### 4. Start the Application
```bash
# Build and start all services
docker-compose up --build

# Or for local development without nginx
docker-compose -f docker-compose.local.yml up --build
```

## Production Deployment - Additional Steps

### 1. Update Production Variables in `.env`
```bash
# Production domain
DOMAIN_NAME=yourdomain.com
ALLOWED_ORIGIN=https://yourdomain.com

# Strong database password
POSTGRES_PASSWORD=<strong-random-password>

# Keep JWT_SECRET from step 1 above
JWT_SECRET=<your-secret-from-step-1>
```

### 2. SSL Certificates
Place your SSL certificates in `/home/deploy/certs/`:
```bash
sudo mkdir -p /home/deploy/certs
sudo cp fullchain.pem /home/deploy/certs/
sudo cp privkey.pem /home/deploy/certs/
```

### 3. Deploy
```bash
docker-compose up -d --build
```

## Verify Security Settings

### Check JWT Secret is Loaded
```bash
docker-compose logs gateway | grep JWT
# Should NOT see any panic messages
```

### Test CORS
```bash
# Should return 401 or 200, not CORS error
curl -H "Origin: https://yourdomain.com" http://localhost:8080/api/urls
```

### Test SSRF Protection
```bash
# Login first and get token
TOKEN=$(curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r .token)

# Try to create short URL to localhost (should fail)
curl -X POST http://localhost:8080/api/shorten \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"url":"http://127.0.0.1:8080"}'

# Should return error: "loopback IP addresses are not allowed"
```

## Common Issues

### "JWT_SECRET environment variable is not set"
- You didn't create `.env` file
- JWT_SECRET is not set in `.env`
- Solution: Follow steps 1-2 above

### Database connection failed
- PostgreSQL service is not ready
- Solution: Wait a few seconds and try again, or check `docker-compose logs postgres`

### CORS errors in browser
- ALLOWED_ORIGIN doesn't match your domain
- Solution: Update ALLOWED_ORIGIN in `.env` and restart

## Security Checklist

Before going to production:
- [ ] JWT_SECRET is set and random (32+ characters)
- [ ] ALLOWED_ORIGIN is set to your production domain
- [ ] POSTGRES_PASSWORD is changed from default
- [ ] SSL certificates are in place
- [ ] Default test users password changed or disabled
- [ ] Tested SSRF protection
- [ ] Tested rate limiting
- [ ] Reviewed SECURITY_FIXES.md

## Need More Details?

See [SECURITY_FIXES.md](SECURITY_FIXES.md) for complete documentation.

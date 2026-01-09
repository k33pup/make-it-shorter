# SSL Certificates

This directory contains SSL certificates for HTTPS support.

## Development

For local development, self-signed certificates are automatically generated when you run:

```bash
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout nginx/ssl/make-it-shorter.key \
  -out nginx/ssl/make-it-shorter.crt \
  -subj "/C=RU/ST=Moscow/L=Moscow/O=MakeItShorter/OU=Dev/CN=make-it-shorter.ru"
```

These certificates are for testing only and will show a browser warning.

## Production

For production deployment, replace the self-signed certificates with real ones from a Certificate Authority (CA).

### Option 1: Let's Encrypt (Free)

Use certbot to obtain free SSL certificates:

```bash
# Install certbot
sudo apt-get install certbot

# Generate certificates
sudo certbot certonly --standalone -d your-domain.com -d www.your-domain.com

# Copy certificates to this directory
sudo cp /etc/letsencrypt/live/your-domain.com/fullchain.pem nginx/ssl/make-it-shorter.crt
sudo cp /etc/letsencrypt/live/your-domain.com/privkey.pem nginx/ssl/make-it-shorter.key
```

### Option 2: Commercial CA

1. Generate a Certificate Signing Request (CSR)
2. Purchase a certificate from a CA
3. Place the certificate files in this directory with the names:
   - `make-it-shorter.crt` (certificate)
   - `make-it-shorter.key` (private key)

## Important Notes

- **Never commit real SSL certificates to version control**
- Keep your private key secure and never share it
- Certificates are excluded from git via `.gitignore`
- Update certificates before they expire

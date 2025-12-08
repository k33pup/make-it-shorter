#!/bin/bash

# Загружаем переменные из .env
if [ -f .env ]; then
  export $(cat .env | xargs)
else
  echo "Error: .env file not found."
  exit 1
fi

if [ -z "$DOMAIN_NAME" ]; then
  echo "Error: DOMAIN_NAME is not set in .env"
  exit 1
fi

# Determine docker compose command
if [ -x "$(command -v docker-compose)" ]; then
  DOCKER_COMPOSE="docker-compose"
elif docker compose version >/dev/null 2>&1; then
  DOCKER_COMPOSE="docker compose"
else
  echo 'Error: docker-compose is not installed.' >&2
  exit 1
fi

domains=($DOMAIN_NAME)
rsa_key_size=4096
data_path="./certbot"
email="$EMAIL" 
staging=0 # Set to 1 if you're testing your setup to avoid hitting request limits

if [ -d "$data_path" ]; then
  read -p "Existing data found for $domains. Continue and replace existing certificate? (y/N) " decision
  if [ "$decision" != "Y" ] && [ "$decision" != "y" ]; then
    exit
  fi
fi

if [ ! -e "$data_path/conf/options-ssl-nginx.conf" ] || [ ! -e "$data_path/conf/ssl-dhparams.pem" ]; then
  echo "### Downloading recommended TLS parameters ..."
  mkdir -p "$data_path/conf"
  curl -s https://raw.githubusercontent.com/certbot/certbot/master/certbot-nginx/certbot_nginx/_internal/tls_configs/options-ssl-nginx.conf > "$data_path/conf/options-ssl-nginx.conf"
  curl -s https://raw.githubusercontent.com/certbot/certbot/master/certbot/certbot/ssl-dhparams.pem > "$data_path/conf/ssl-dhparams.pem"
  echo
fi

echo "### Creating dummy certificate for $domains ..."
path="/etc/letsencrypt/live/$domains"
mkdir -p "$data_path/conf/live/$domains"

# Using environment variable for domain in the command
$DOCKER_COMPOSE run --rm --entrypoint "openssl req -x509 -nodes -newkey rsa:$rsa_key_size -days 1 -keyout '$path/privkey.pem' -out '$path/fullchain.pem' -subj '/CN=localhost'" certbot
echo

echo "### Starting nginx ..."
$DOCKER_COMPOSE up --force-recreate -d nginx
echo

echo "### Deleting dummy certificate for $domains ..."
$DOCKER_COMPOSE run --rm --entrypoint "rm -Rf /etc/letsencrypt/live/$domains && rm -Rf /etc/letsencrypt/archive/$domains && rm -Rf /etc/letsencrypt/renewal/$domains.conf" certbot
echo

echo "### Requesting Let's Encrypt certificate for $domains ..."
domain_args=""
for domain in "${domains[@]}"; do
  domain_args="$domain_args -d $domain"
done

case "$email" in
  "") email_arg="--register-unsafely-without-email" ;;
  *) email_arg="-m $email" ;;
esac

if [ $staging != "0" ]; then staging_arg="--staging"; fi

$DOCKER_COMPOSE run --rm --entrypoint "certbot certonly --webroot -w /var/www/certbot $staging_arg $email_arg $domain_args --rsa-key-size $rsa_key_size --agree-tos --force-renewal" certbot
echo

echo "### Reloading nginx ..."
$DOCKER_COMPOSE exec nginx nginx -s reload

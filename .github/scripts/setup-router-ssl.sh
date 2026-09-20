#!/bin/bash
set -e

# === Certbot Hooks (use ZCDNS HTTP API, no sqlite3 needed) ===
# Reads ADMIN_KEY from /opt/zcdns/.env or falls back to default

cat > /opt/zcdns/certbot-router-auth.sh << 'HOOK'
#!/bin/bash
# Read admin key from env file
ADMIN_KEY=$(grep '^ADMIN_KEY=' /opt/zcdns/.env 2>/dev/null | cut -d= -f2- | tr -d '"' || echo "@Kelvin123")
# Insert _acme-challenge TXT record via ZCDNS API
curl -sf -X POST http://127.0.0.1:8080/api/acme-challenge \
  -H "Content-Type: application/json" \
  -H "X-Admin-Key: ${ADMIN_KEY}" \
  -d "{\"subdomain\":\"router\",\"value\":\"${CERTBOT_VALIDATION}\"}" \
  && echo "[AUTH] TXT record added: ${CERTBOT_VALIDATION}" \
  || echo "[AUTH] WARN: failed to add TXT record"
sleep 10
HOOK

cat > /opt/zcdns/certbot-router-cleanup.sh << 'HOOK'
#!/bin/bash
# Read admin key from env file
ADMIN_KEY=$(grep '^ADMIN_KEY=' /opt/zcdns/.env 2>/dev/null | cut -d= -f2- | tr -d '"' || echo "@Kelvin123")
# Get list of acme challenge records and delete matching one
RECORD_ID=$(curl -sf http://127.0.0.1:8080/api/acme-challenge \
  -H "X-Admin-Key: ${ADMIN_KEY}" | \
  python3 -c "import sys,json; records=json.load(sys.stdin); \
match=[r for r in records if r.get('value')=='${CERTBOT_VALIDATION}']; \
print(match[0]['id'] if match else '')" 2>/dev/null || echo "")
if [ -n "${RECORD_ID}" ]; then
  curl -sf -X DELETE "http://127.0.0.1:8080/api/records/${RECORD_ID}" \
    -H "X-Admin-Key: ${ADMIN_KEY}" \
    && echo "[CLEANUP] TXT record removed" || true
fi
HOOK

chmod +x /opt/zcdns/certbot-router-auth.sh /opt/zcdns/certbot-router-cleanup.sh

# === Issue Wildcard Cert if not exists ===
if [ ! -d "/etc/letsencrypt/live/router.zcdns.id" ] && which certbot >/dev/null 2>&1; then
  certbot certonly \
    --manual \
    --preferred-challenges=dns \
    --manual-auth-hook /opt/zcdns/certbot-router-auth.sh \
    --manual-cleanup-hook /opt/zcdns/certbot-router-cleanup.sh \
    -d "*.router.zcdns.id" \
    -d "router.zcdns.id" \
    --agree-tos \
    --register-unsafely-without-email \
    --non-interactive || echo "[WARN] certbot failed - wildcard cert not issued yet"
fi

# === Setup Nginx if cert exists ===
if [ -d "/etc/letsencrypt/live/router.zcdns.id" ] && [ -d "/etc/nginx/sites-available" ]; then
  cat > /etc/nginx/sites-available/router.zcdns.id << 'NGINX'
server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name router.zcdns.id *.router.zcdns.id;

    ssl_certificate /etc/letsencrypt/live/router.zcdns.id/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/router.zcdns.id/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
    }
}
NGINX

  ln -sf /etc/nginx/sites-available/router.zcdns.id /etc/nginx/sites-enabled/router.zcdns.id
  nginx -t && systemctl reload nginx || echo "[WARN] nginx reload failed"
else
  echo "[INFO] Skipping nginx setup: cert not yet issued or nginx not installed"
fi

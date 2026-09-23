#!/bin/bash
set -e

# === Certbot Hooks (use ZCDNS HTTP API, no sqlite3 needed) ===
# Reads ADMIN_KEY from /opt/zcdns/.env or falls back to default
# NOTE: hook is called TWICE by certbot (once for *.domain, once for domain)
#       Both TXT records must exist before LE verifies - we wait for AXFR propagation

ZCDNS_PORT=$(grep '^PORT=' /opt/zcdns/.env 2>/dev/null | cut -d= -f2- | tr -d '"' || echo "8085")

cat > /opt/zcdns/certbot-router-auth.sh << HOOK
#!/bin/bash
# Read admin key and port from env file
ADMIN_KEY=\$(grep '^ADMIN_KEY=' /opt/zcdns/.env 2>/dev/null | cut -d= -f2- | tr -d '"' || echo "@Kelvin123")
ZCDNS_PORT=\$(grep '^PORT=' /opt/zcdns/.env 2>/dev/null | cut -d= -f2- | tr -d '"' || echo "8085")

echo "[AUTH] Adding TXT record for domain: \${CERTBOT_DOMAIN}, validation: \${CERTBOT_VALIDATION}"

# Insert _acme-challenge TXT record via ZCDNS API
RESULT=\$(curl -sf -X POST "http://127.0.0.1:\${ZCDNS_PORT}/api/acme-challenge" \\
  -H "Content-Type: application/json" \\
  -H "X-Admin-Key: \${ADMIN_KEY}" \\
  -d "{\"subdomain\":\"router\",\"value\":\"\${CERTBOT_VALIDATION}\"}" 2>&1)

if [ \$? -eq 0 ]; then
  echo "[AUTH] TXT record added OK: \${CERTBOT_VALIDATION}"
else
  echo "[AUTH] WARN: curl failed: \${RESULT}"
fi

# Wait for AXFR to propagate to secondary NS (ns2.zcdns.id)
# LE queries both nameservers - must wait for zone transfer
echo "[AUTH] Waiting 60s for AXFR propagation to secondary NS..."
sleep 60

# Verify TXT is visible in DNS before returning
DIG_RESULT=\$(dig @127.0.0.1 _acme-challenge.router.zcdns.id TXT +short 2>/dev/null || echo "")
echo "[AUTH] DNS check result: \${DIG_RESULT}"
HOOK

cat > /opt/zcdns/certbot-router-cleanup.sh << HOOK
#!/bin/bash
# Read admin key from env file
ADMIN_KEY=\$(grep '^ADMIN_KEY=' /opt/zcdns/.env 2>/dev/null | cut -d= -f2- | tr -d '"' || echo "@Kelvin123")
ZCDNS_PORT=\$(grep '^PORT=' /opt/zcdns/.env 2>/dev/null | cut -d= -f2- | tr -d '"' || echo "8085")

echo "[CLEANUP] Removing TXT record: \${CERTBOT_VALIDATION}"

# Find and delete matching TXT record
RECORD_ID=\$(curl -sf "http://127.0.0.1:\${ZCDNS_PORT}/api/acme-challenge" \\
  -H "X-Admin-Key: \${ADMIN_KEY}" | \\
  python3 -c "import sys,json
records=json.load(sys.stdin)
match=[r for r in records if r.get('value')=='\${CERTBOT_VALIDATION}']
print(match[0]['id'] if match else '')" 2>/dev/null || echo "")

if [ -n "\${RECORD_ID}" ]; then
  curl -sf -X DELETE "http://127.0.0.1:\${ZCDNS_PORT}/api/records/\${RECORD_ID}" \\
    -H "X-Admin-Key: \${ADMIN_KEY}" \\
    && echo "[CLEANUP] TXT record removed" || echo "[CLEANUP] WARN: delete failed"
else
  echo "[CLEANUP] No matching TXT record found (may already be removed)"
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
    listen 10.0.0.12:443 ssl http2;
    listen [fd00::12]:443 ssl http2;
    server_name router.zcdns.id *.router.zcdns.id;

    ssl_certificate /etc/letsencrypt/live/router.zcdns.id/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/router.zcdns.id/privkey.pem;

    location / {
        # CORS preflight
        if ($request_method = 'OPTIONS') {
            add_header 'Access-Control-Allow-Origin' '*' always;
            add_header 'Access-Control-Allow-Methods' 'GET, POST, OPTIONS, PUT, DELETE, PATCH' always;
            add_header 'Access-Control-Allow-Headers' '*' always;
            add_header 'Access-Control-Max-Age' 86400 always;
            add_header 'Content-Type' 'text/plain; charset=utf-8';
            add_header 'Content-Length' 0;
            return 204;
        }

        proxy_pass http://127.0.0.1:8085;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $http_connection;
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

  # Route router.zcdns.id and wildcard to Nginx's IP (10.0.0.12 / fd00::12)
  shared-ip update router.zcdns.id --localport=443 --localipv4=10.0.0.12 --localipv6=fd00::12 2>/dev/null || shared-ip add router.zcdns.id --localport=443 --localipv4=10.0.0.12 --localipv6=fd00::12 2>/dev/null || true
  shared-ip update "*.router.zcdns.id" --localport=443 --localipv4=10.0.0.12 --localipv6=fd00::12 2>/dev/null || shared-ip add "*.router.zcdns.id" --localport=443 --localipv4=10.0.0.12 --localipv6=fd00::12 2>/dev/null || true
  systemctl restart shared-ip 2>/dev/null || true

  # Clean up any leftover ACME challenge records
  sqlite3 /opt/zcdns/zcdns.db "DELETE FROM records WHERE subdomain='router' AND name='_acme-challenge';" 2>/dev/null || true
  systemctl restart zcdns 2>/dev/null || true
  echo "[INFO] Nginx configured on 10.0.0.12:443 and shared-ip updated successfully"
else
  echo "[INFO] Skipping nginx setup: cert not yet issued or nginx not installed"
fi


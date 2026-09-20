#!/bin/bash
set -e

# === Certbot Hooks ===
cat > /opt/zcdns/certbot-router-auth.sh << 'HOOK'
#!/bin/bash
sqlite3 /opt/zcdns/zcdns.db "INSERT OR IGNORE INTO records (id, subdomain, name, type, value, ttl) VALUES (lower(hex(randomblob(16))), 'router', '_acme-challenge', 'TXT', '${CERTBOT_VALIDATION}', 60);"
systemctl restart zcdns 2>/dev/null || true
sleep 5
HOOK

cat > /opt/zcdns/certbot-router-cleanup.sh << 'HOOK'
#!/bin/bash
sqlite3 /opt/zcdns/zcdns.db "DELETE FROM records WHERE subdomain='router' AND name='_acme-challenge' AND type='TXT' AND value='${CERTBOT_VALIDATION}';"
systemctl restart zcdns 2>/dev/null || true
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

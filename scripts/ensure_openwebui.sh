#!/usr/bin/env bash
set -e

OPENWEBUI_DIR="/tmp/open-webui"
REPO_URL="https://github.com/open-webui/open-webui.git"

if [ ! -d "$OPENWEBUI_DIR/.git" ]; then
    echo "==> [ensure_openwebui] /tmp/open-webui not found or incomplete. Cloning..."
    rm -rf "$OPENWEBUI_DIR"
    git clone --depth 1 "$REPO_URL" "$OPENWEBUI_DIR"
    echo "==> [ensure_openwebui] Successfully cloned open-webui to $OPENWEBUI_DIR"
else
    echo "==> [ensure_openwebui] $OPENWEBUI_DIR is present and ready."
fi

#!/usr/bin/env bash
set -e

TARGET_DIR="${1:-/tmp/open-webui}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
PATCH_DIR="$ROOT_DIR/patches/openwebui"

echo "==> [patch_openwebui] Applying OpenWebUI patches to: $TARGET_DIR"

if [ ! -d "$TARGET_DIR" ]; then
    echo "ERROR: Target directory $TARGET_DIR does not exist!"
    exit 1
fi

if [ -d "$PATCH_DIR" ]; then
    cp -rv "$PATCH_DIR"/* "$TARGET_DIR"/
    echo "==> [patch_openwebui] Successfully applied all custom patches from $PATCH_DIR!"
else
    echo "==> [patch_openwebui] No patches directory found at $PATCH_DIR"
fi

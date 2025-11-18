#!/usr/bin/env bash
set -euo pipefail

TARGET_DIR="pkg/profiles"

echo "Re-encrypting all SOPS files under: $TARGET_DIR"
echo

# Find all relevant file types
find "$TARGET_DIR" -type f -name "*.yml" | while IFS= read -r file; do
    echo "🔐 Re-encrypting $file"

    # decrypt → encrypt using new rules
    if sops -d -i "$file"; then
        sops -e -i "$file"
    else
        echo "❌ Failed to decrypt $file — skipping"
    fi
done

echo
echo "✅ Done re-encrypting all files."

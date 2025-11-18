#!/usr/bin/env bash
set -euo pipefail

echo "🔐 Checking pkg/profiles for unencrypted secrets..."

# Track if any unencrypted secret lines are found
FAILED=0

# Loop through all YAML files in pkg/profiles
for file in pkg/profiles/*.yml pkg/profiles/*.yaml; do
    # Skip if no files match
    [ -e "$file" ] || continue

    # Find lines containing key/secret/password but not ENC
    UNENCRYPTED_LINES=$(grep -E "(key|secret|password).*:" "$file" | grep -v "ENC" || true)

    if [ -n "$UNENCRYPTED_LINES" ]; then
        echo "❌ Unencrypted secrets found in $file:"
        echo "$UNENCRYPTED_LINES"
        FAILED=1
    fi
done

if [ "$FAILED" -eq 1 ]; then
    echo "Commit aborted: unencrypted secrets detected."
    exit 1
fi

echo "✔ All secrets in pkg/profiles are encrypted."

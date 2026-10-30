#!/usr/bin/env bash
# =============================================================================
# scripts/post-release.sh - post-release: upload to GitCode Release
#
# Called by the CI workflow after the GoReleaser release. Collects artifacts
# from dist/ and install scripts from dist-extra/, stages them into dist/gitcode/,
# then uploads via upload-gitcode-release.sh.
#
# Usage (auto-invoked by the CI workflow):
#   ./scripts/post-release.sh <version>
#
# Environment:
#   GITCODE_TOKEN  GitCode personal access token (skipped when empty)
# =============================================================================
set -euo pipefail

VERSION="${1:?usage: post-release.sh <version>}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DIST_DIR="${PROJECT_ROOT}/dist"
EXTRA_DIR="${PROJECT_ROOT}/dist-extra"
GITCODE_DIR="${DIST_DIR}/gitcode"

# ---- Skip when no token is set ----
if [[ -z "${GITCODE_TOKEN:-}" ]]; then
    echo "⚠️  GITCODE_TOKEN not set, skipping GitCode Release upload"
    exit 0
fi

# ---- Collect artifacts into dist/gitcode/ ----
mkdir -p "${GITCODE_DIR}"

# Archives + checksum
cp "${DIST_DIR}"/devbox_*.tar.gz "${GITCODE_DIR}/"
cp "${DIST_DIR}/checksums.txt" "${GITCODE_DIR}/"

# Baked install scripts
if [[ -f "${EXTRA_DIR}/install.sh" ]]; then
    cp "${EXTRA_DIR}/install.sh" "${GITCODE_DIR}/"
fi
if [[ -f "${EXTRA_DIR}/install.ps1" ]]; then
    cp "${EXTRA_DIR}/install.ps1" "${GITCODE_DIR}/"
fi

echo "GitCode upload directory contents:"
ls -la "${GITCODE_DIR}"/

# ---- Upload to GitCode Release ----
# upload-gitcode-release.sh re-bakes the install scripts to point at the GitCode URL.
# Release display name: 0.1.0-release → devbox-cli-0.1.0
DISPLAY_NAME="devbox-cli-$(echo "${VERSION}" | sed 's/-release$//')"
"${SCRIPT_DIR}/upload-gitcode-release.sh" \
    -t "${GITCODE_TOKEN}" \
    -o CloudDeveloperDepartment \
    -r devbox \
    -v "${VERSION}" \
    -d "${GITCODE_DIR}" \
    -n "${DISPLAY_NAME}" \
    -b "DevBox CLI ${VERSION} - synced from GitHub Release"

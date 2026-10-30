#!/usr/bin/env bash
# =============================================================================
# scripts/bake-install.sh - bake the install scripts
#
# Inject the version and download URL into install.sh / install.ps1, per channel:
#
#   dist-extra/install.sh / install.ps1      — GitHub channel (points to GitHub Release)
#
# GitCode needs no separate bake: post-release.sh re-bakes it to the GitCode URL on
# upload via upload-gitcode-release.sh.
#
# Usage:
#   ./scripts/bake-install.sh <version> <github_release_url>
#
# Example:
#   ./scripts/bake-install.sh 1.0.0-release \
#     "https://github.com/huaweicloud/devspace-devbox/releases/download/1.0.0-release"
#
# Output:
#   dist-extra/install.sh / install.ps1   — GitHub channel
# =============================================================================
set -euo pipefail

VERSION="${1:?usage: bake-install.sh <version> <github_release_url>}"
GITHUB_RELEASE_URL="${2:?usage: bake-install.sh <version> <github_release_url>}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
OUTPUT_DIR="${PROJECT_ROOT}/dist-extra"

mkdir -p "${OUTPUT_DIR}"

bake() {
    local src="$1" dst_dir="$2" artifact_url="$3"
    mkdir -p "${dst_dir}"
    sed -e "s|^DEFAULT_VERSION=.*|DEFAULT_VERSION=\"${VERSION}\"|" \
        -e "s|^DEFAULT_ARTIFACT_URL=.*|DEFAULT_ARTIFACT_URL=\"${artifact_url}\"|" \
        "${src}/install.sh" > "${dst_dir}/install.sh"
    chmod +x "${dst_dir}/install.sh"
    sed -e "s|DEFAULT_VERSION = \".*\"|DEFAULT_VERSION = \"${VERSION}\"|" \
        -e "s|DEFAULT_ARTIFACT_URL = \".*\"|DEFAULT_ARTIFACT_URL = \"${artifact_url}\"|" \
        "${src}/install.ps1" > "${dst_dir}/install.ps1"
    echo "=== baked ${dst_dir} ==="
    grep -E '^DEFAULT_ARTIFACT_URL=|^DEFAULT_VERSION=' "${dst_dir}/install.sh"
    grep -E 'DEFAULT_ARTIFACT_URL = "|DEFAULT_VERSION = "' "${dst_dir}/install.ps1" | head -2
}

# ---- GitHub channel ----
bake "${PROJECT_ROOT}" "${OUTPUT_DIR}" "${GITHUB_RELEASE_URL}"

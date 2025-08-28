#!/bin/bash
# Chrome Flatpak wrapper script for chromedp - minimal version

# Get project directory (assuming wrapper is in backend/scripts/)
PROJECT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
PROJECT_TMP_DIR="$PROJECT_DIR/tmp"

# Create project tmp directory if it doesn't exist
mkdir -p "$PROJECT_TMP_DIR"

# Set alternative runtime directory to avoid space issues (use standard /tmp for flatpak compatibility)
export XDG_RUNTIME_DIR="/tmp/webtestflow-flatpak-runtime-$$"
mkdir -p "$XDG_RUNTIME_DIR"

# Grant network access and disable sandbox for automation  
exec flatpak run --share=network --share=ipc --socket=x11 --socket=wayland --filesystem=home --device=dri com.google.Chrome \
    --no-first-run \
    --no-default-browser-check \
    --disable-dev-shm-usage \
    --no-sandbox \
    --disk-cache-dir="$PROJECT_TMP_DIR/chrome-cache-$$" \
    "$@"
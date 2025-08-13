#!/bin/bash

# Quick Docker/Podman Permission Fix Script

echo "🔧 Fixing Docker/Podman permissions..."

# Check if user has sudo without password
if sudo -n true 2>/dev/null; then
    echo "✅ User has sudo privileges"
    
    # Fix Docker socket permissions if it exists
    if [[ -S "/var/run/docker.sock" ]]; then
        echo "🔑 Fixing Docker socket permissions..."
        sudo chmod 666 /var/run/docker.sock
        echo "✅ Docker socket permissions fixed"
    fi
    
    # Suppress Podman warnings
    if command -v podman >/dev/null 2>&1; then
        if [[ ! -f "/etc/containers/nodocker" ]]; then
            echo "🔇 Suppressing Podman warnings..."
            sudo touch /etc/containers/nodocker
            echo "✅ Podman warnings suppressed"
        fi
    fi
    
    echo "✅ Permissions fixed! You can now run the startup script."
else
    echo "❌ User does not have sudo privileges"
    exit 1
fi
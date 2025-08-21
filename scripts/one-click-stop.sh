#!/bin/bash

# WebTestFlow One-Click Stop Script
# This script stops the complete development environment including database, backend, and frontend

set -e

echo "🛑 Stopping WebTestFlow Complete Development Environment..."

# Color definitions for better output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to stop a service by PID file
stop_service() {
    local service_name=$1
    local pid_file=$2
    
    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        if kill -0 "$pid" 2>/dev/null; then
            print_status "Stopping $service_name (PID: $pid)..."
            kill -TERM "$pid"
            
            # Wait for graceful shutdown
            local count=0
            while kill -0 "$pid" 2>/dev/null && [ $count -lt 10 ]; do
                sleep 1
                count=$((count + 1))
            done
            
            # Force kill if still running
            if kill -0 "$pid" 2>/dev/null; then
                print_warning "Force killing $service_name..."
                kill -KILL "$pid"
            fi
            
            print_success "$service_name stopped"
        else
            print_status "$service_name was not running"
        fi
        rm -f "$pid_file"
    else
        print_status "No PID file found for $service_name"
    fi
}

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to setup Docker/Podman environment
setup_container_env() {
    # Check if we're using Podman
    if command_exists podman; then
        # Set environment variable for Podman
        export DOCKER_HOST="unix:///run/user/$(id -u)/podman/podman.sock"
        
        # Check if we need sudo (simplified check)
        if ! podman info >/dev/null 2>&1 && sudo -n true 2>/dev/null; then
            export USE_SUDO_PODMAN=true
        fi
    fi
}

# Setup container environment
setup_container_env

# Stop backend service
print_status "Stopping backend service..."
stop_service "Backend" "logs/pid/backend.pid"

# Stop frontend service
print_status "Stopping frontend service..."
stop_service "Frontend" "logs/pid/frontend.pid"

# Stop OCR service
print_status "Stopping OCR service..."
stop_service "OCR" "logs/pid/ocr.pid"

# Function to run Docker Compose commands
run_docker_compose() {
    local compose_cmd=""
    local use_sudo=""
    
    # Check if we need to use sudo
    if [[ "${USE_SUDO_PODMAN:-}" == "true" ]]; then
        use_sudo="sudo -E"
    fi
    
    # Determine compose command
    if command_exists docker-compose; then
        compose_cmd="docker-compose"
    elif docker compose version &>/dev/null 2>/dev/null; then
        compose_cmd="docker compose"
    elif $use_sudo docker compose version &>/dev/null 2>/dev/null; then
        compose_cmd="docker compose"
    else
        print_error "Docker Compose is not available"
        return 1
    fi
    
    # Execute command
    if [[ -n "$use_sudo" ]]; then
        $use_sudo $compose_cmd "$@"
    else
        $compose_cmd "$@"
    fi
}

# Stop MySQL database container
print_status "Stopping MySQL database..."
if docker ps --format "table {{.Names}}" | grep -q "webtestflow-mysql"; then
    run_docker_compose down mysql
    print_success "MySQL container stopped"
else
    print_status "MySQL container was not running"
fi

# Stop OCR service container
print_status "Stopping OCR service container..."
if docker ps --format "table {{.Names}}" | grep -q "webtestflow-ocr"; then
    docker stop webtestflow-ocr >/dev/null 2>&1 && print_success "OCR container stopped" || true
    docker rm webtestflow-ocr >/dev/null 2>&1 && print_success "OCR container removed" || true
elif docker ps --format "table {{.Names}}" | grep -q "ocr-service"; then
    run_docker_compose down ocr-service
    print_success "OCR service stopped via Docker Compose"
else
    print_status "OCR container was not running"
fi

# Clean up any remaining processes
print_status "Cleaning up any remaining processes..."

# Kill backend processes
print_status "Stopping backend processes..."
# Kill webtestflow backend binary
pkill -f "webtestflow" 2>/dev/null && print_success "Stopped webtestflow backend" || true
# Kill main binary (alternative name)
pkill -f "./main" 2>/dev/null && print_success "Stopped main backend" || true
# Kill any remaining go processes for this project
pkill -f "go run cmd/main.go" 2>/dev/null && print_success "Stopped go run backend" || true
# Kill any processes using port 8080 (backend port)
if lsof -ti:8080 >/dev/null 2>&1; then
    lsof -ti:8080 | xargs kill -9 2>/dev/null && print_success "Killed processes on port 8080" || true
fi

# Kill frontend processes
print_status "Stopping frontend processes..."
# Kill any remaining npm start processes
pkill -f "npm start" 2>/dev/null && print_success "Stopped npm start processes" || true
# Kill any remaining react-scripts processes
pkill -f "react-scripts start" 2>/dev/null && print_success "Stopped react-scripts processes" || true
# Kill any node processes related to the frontend
pkill -f "node.*react-scripts" 2>/dev/null && print_success "Stopped node react-scripts processes" || true
# Kill any processes using port 3000 (frontend port)
if lsof -ti:3000 >/dev/null 2>&1; then
    lsof -ti:3000 | xargs kill -9 2>/dev/null && print_success "Killed processes on port 3000" || true
fi

# Kill OCR service processes
print_status "Stopping OCR service processes..."
# Kill any Python OCR processes
pkill -f "ocr_server.py" 2>/dev/null && print_success "Stopped OCR server processes" || true
# Kill any processes using port 8888 (OCR port)
if lsof -ti:8888 >/dev/null 2>&1; then
    lsof -ti:8888 | xargs kill -9 2>/dev/null && print_success "Killed processes on port 8888" || true
fi

# Kill any Chrome processes started by the test framework
print_status "Stopping Chrome test processes..."
pkill -f "chrome.*remote-debugging-port" 2>/dev/null && print_success "Stopped Chrome debugging processes" || true

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
print_success "✅ All WebTestFlow services have been stopped"
echo ""
echo -e "${BLUE}📝 Information:${NC}"
echo "   📁 Log files are preserved in the logs/ directory"
echo "   🗄️  Database data is preserved in Docker volumes"
echo ""
echo -e "${BLUE}🔄 To restart:${NC}"
echo "   ./scripts/one-click-start.sh  # Start everything"
echo "   ./scripts/start-dev.sh        # Start backend and frontend only"
echo ""
echo -e "${BLUE}🧹 To completely clean up (removes database data):${NC}"
echo "   docker-compose down -v       # Remove all containers and volumes"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
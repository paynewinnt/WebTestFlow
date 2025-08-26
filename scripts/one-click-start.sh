#!/bin/bash

# WebTestFlow One-Click Startup Script with Auto-Installation
# This script starts the complete development environment including database, backend, and frontend
# It automatically installs missing dependencies when possible
#
# Usage:
#   ./scripts/one-click-start.sh [OPTIONS]
#   
# Options:
#   -y, --yes       Auto-install dependencies without prompting
#   -h, --help      Show this help message
#   --skip-deps     Skip dependency installation (faster if deps are already installed)
#   --verbose       Enable verbose output for debugging
#
# Examples:
#   ./scripts/one-click-start.sh                    # Interactive mode (default)
#   ./scripts/one-click-start.sh -y                # Auto-install mode
#   ./scripts/one-click-start.sh --skip-deps       # Skip dependency checks
#
# Environment Variables:
#   AUTO_INSTALL=1                                  # Enable auto-install mode
#   SKIP_DEPS=1                                     # Skip dependency installation
#   VERBOSE=1                                       # Enable verbose output

set -e

# Default options
AUTO_INSTALL=false
SKIP_DEPS=false
VERBOSE=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -y|--yes)
            AUTO_INSTALL=true
            shift
            ;;
        -h|--help)
            echo "WebTestFlow One-Click Startup Script"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -y, --yes       Auto-install dependencies without prompting"
            echo "  -h, --help      Show this help message"
            echo "  --skip-deps     Skip dependency installation"
            echo "  --verbose       Enable verbose output for debugging"
            echo ""
            echo "Examples:"
            echo "  $0                    # Interactive mode (default)"
            echo "  $0 -y                # Auto-install mode"
            echo "  $0 --skip-deps       # Skip dependency checks"
            echo ""
            exit 0
            ;;
        --skip-deps)
            SKIP_DEPS=true
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use -h or --help for usage information"
            exit 1
            ;;
    esac
done

# Check environment variables
if [[ "${AUTO_INSTALL:-}" == "1" ]]; then
    AUTO_INSTALL=true
fi

if [[ "${SKIP_DEPS:-}" == "1" ]]; then
    SKIP_DEPS=true
fi

if [[ "${VERBOSE:-}" == "1" ]]; then
    VERBOSE=true
fi

# Enable verbose mode if requested
if $VERBOSE; then
    set -x
fi

echo "🚀 Starting WebTestFlow Complete Development Environment..."

# Color definitions for better output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
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

print_install() {
    echo -e "${PURPLE}[INSTALL]${NC} $1"
}

# Check if we're in the right directory
if [ ! -f "backend/go.mod" ]; then
    print_error "backend/go.mod not found. Please run this script from the project root directory."
    exit 1
fi

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to setup Docker/Podman environment
setup_container_env() {
    # Check if we're using Podman
    if command_exists podman; then
        print_status "Detected Podman container engine"
        
        # Enable user Podman socket if not already running
        if ! systemctl --user is-active --quiet podman.socket; then
            print_status "Enabling user Podman socket..."
            systemctl --user enable --now podman.socket
            sleep 2  # Give socket time to start
        fi
        
        # Set environment variable for Podman
        export DOCKER_HOST="unix:///run/user/$(id -u)/podman/podman.sock"
        print_success "Podman environment configured (DOCKER_HOST=$DOCKER_HOST)"
        
        # Test Podman access
        if ! podman info >/dev/null 2>&1; then
            print_error "Podman is not accessible. Checking alternatives..."
            
            # Option 1: Try with sudo if user has no-password sudo
            if sudo -n true 2>/dev/null; then
                print_status "Trying with elevated privileges..."
                if ask_permission "Use sudo for container operations" "sudo podman"; then
                    # Create a wrapper function for sudo podman
                    print_success "Will use sudo for container operations"
                    export USE_SUDO_PODMAN=true
                fi
            fi
            
            # Option 2: Fix Docker socket permissions
            if [[ -S "/var/run/docker.sock" ]]; then
                print_status "Found Docker socket, attempting to fix permissions..."
                if ask_permission "Fix Docker socket permissions" "sudo chmod 666 /var/run/docker.sock"; then
                    sudo chmod 666 /var/run/docker.sock
                    unset DOCKER_HOST  # Use default Docker socket
                    print_success "Docker socket permissions fixed"
                fi
            fi
        else
            print_success "Podman is accessible via user socket"
        fi
        
        # Create nodocker file to suppress warning if desired
        if [[ ! -f "/etc/containers/nodocker" ]]; then
            if ask_permission "Suppress Podman Docker emulation warnings" "sudo touch /etc/containers/nodocker"; then
                sudo touch /etc/containers/nodocker 2>/dev/null || true
                print_success "Podman warnings suppressed"
            fi
        fi
        
    elif command_exists docker; then
        print_status "Detected Docker container engine"
        
        # Check Docker daemon accessibility
        if ! docker info >/dev/null 2>&1; then
            print_warning "Docker daemon is not accessible"
            
            # Option 1: Fix socket permissions
            if [[ -S "/var/run/docker.sock" ]]; then
                print_status "Attempting to fix Docker socket permissions..."
                if ask_permission "Fix Docker socket permissions" "sudo chmod 666 /var/run/docker.sock"; then
                    sudo chmod 666 /var/run/docker.sock
                    print_success "Docker socket permissions fixed"
                fi
            fi
            
            # Option 2: Start Docker service
            if ! docker info >/dev/null 2>&1 && command_exists systemctl; then
                print_status "Attempting to start Docker service..."
                if sudo systemctl start docker 2>/dev/null; then
                    print_success "Docker service started"
                else
                    print_error "Failed to start Docker service"
                fi
            fi
            
            # Option 3: Add user to docker group
            if ! docker info >/dev/null 2>&1 && ! groups | grep -q docker; then
                print_warning "User is not in docker group"
                if ask_permission "Add user to docker group" "sudo usermod -aG docker \$USER"; then
                    sudo usermod -aG docker $USER
                    print_warning "Group change will take effect after logout/login"
                    print_status "Trying immediate group activation..."
                    if command_exists newgrp; then
                        print_status "Run: newgrp docker"
                    fi
                fi
            fi
            
            # Final check
            if ! docker info >/dev/null 2>&1; then
                print_error "Docker is still not accessible after fixes"
                return 1
            fi
        fi
        
        print_success "Docker environment is ready"
    else
        print_error "Neither Docker nor Podman found"
        return 1
    fi
}

# Function to detect OS
detect_os() {
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        if command_exists apt-get; then
            echo "ubuntu"
        elif command_exists yum; then
            echo "centos"
        elif command_exists dnf; then
            echo "fedora"
        elif command_exists pacman; then
            echo "arch"
        else
            echo "linux"
        fi
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        echo "macos"
    elif [[ "$OSTYPE" == "cygwin" ]] || [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "win32" ]]; then
        echo "windows"
    else
        echo "unknown"
    fi
}

# Function to ask user for permission
ask_permission() {
    local dependency=$1
    local install_command=$2
    
    echo ""
    print_warning "$dependency is not installed."
    
    if $AUTO_INSTALL; then
        print_install "Auto-installing $dependency..."
        echo -e "Install command: ${BLUE}$install_command${NC}"
        return 0
    fi
    
    echo -e "${YELLOW}Would you like to install it automatically?${NC}"
    echo -e "Install command: ${BLUE}$install_command${NC}"
    echo -n "Install $dependency? (y/N): "
    read -r response
    
    case "$response" in
        [yY]|[yY][eE][sS])
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

# Function to install Go
install_go() {
    local os=$1
    local go_version="1.21.5"
    
    case $os in
        "ubuntu"|"debian")
            local install_cmd="wget -q -O - https://git.io/vQhTU | bash -s -- --version $go_version"
            if ask_permission "Go" "$install_cmd"; then
                print_install "Installing Go $go_version..."
                wget -q -O - https://git.io/vQhTU | bash -s -- --version $go_version
                source ~/.bashrc
                return 0
            fi
            ;;
        "centos"|"fedora")
            local install_cmd="curl -s https://git.io/vQhTU | bash -s -- --version $go_version"
            if ask_permission "Go" "$install_cmd"; then
                print_install "Installing Go $go_version..."
                curl -s https://git.io/vQhTU | bash -s -- --version $go_version
                source ~/.bashrc
                return 0
            fi
            ;;
        "macos")
            if command_exists brew; then
                local install_cmd="brew install go"
                if ask_permission "Go" "$install_cmd"; then
                    print_install "Installing Go via Homebrew..."
                    brew install go
                    return 0
                fi
            else
                print_error "Homebrew not found. Please install Go manually from https://golang.org/dl/"
            fi
            ;;
        "arch")
            local install_cmd="sudo pacman -S go"
            if ask_permission "Go" "$install_cmd"; then
                print_install "Installing Go via pacman..."
                sudo pacman -S go
                return 0
            fi
            ;;
    esac
    return 1
}

# Function to install Node.js
install_node() {
    local os=$1
    
    case $os in
        "ubuntu"|"debian")
            local install_cmd="curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash - && sudo apt-get install -y nodejs"
            if ask_permission "Node.js" "$install_cmd"; then
                print_install "Installing Node.js 18.x..."
                curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
                sudo apt-get install -y nodejs
                return 0
            fi
            ;;
        "centos")
            local install_cmd="curl -fsSL https://rpm.nodesource.com/setup_18.x | sudo bash - && sudo yum install -y nodejs"
            if ask_permission "Node.js" "$install_cmd"; then
                print_install "Installing Node.js 18.x..."
                curl -fsSL https://rpm.nodesource.com/setup_18.x | sudo bash -
                sudo yum install -y nodejs
                return 0
            fi
            ;;
        "fedora")
            local install_cmd="curl -fsSL https://rpm.nodesource.com/setup_18.x | sudo bash - && sudo dnf install -y nodejs"
            if ask_permission "Node.js" "$install_cmd"; then
                print_install "Installing Node.js 18.x..."
                curl -fsSL https://rpm.nodesource.com/setup_18.x | sudo bash -
                sudo dnf install -y nodejs
                return 0
            fi
            ;;
        "macos")
            if command_exists brew; then
                local install_cmd="brew install node"
                if ask_permission "Node.js" "$install_cmd"; then
                    print_install "Installing Node.js via Homebrew..."
                    brew install node
                    return 0
                fi
            else
                print_error "Homebrew not found. Please install Node.js manually from https://nodejs.org/"
            fi
            ;;
        "arch")
            local install_cmd="sudo pacman -S nodejs npm"
            if ask_permission "Node.js" "$install_cmd"; then
                print_install "Installing Node.js via pacman..."
                sudo pacman -S nodejs npm
                return 0
            fi
            ;;
    esac
    return 1
}

# Function to install Docker
install_docker() {
    local os=$1
    
    case $os in
        "ubuntu"|"debian")
            local install_cmd="curl -fsSL https://get.docker.com | sh && sudo usermod -aG docker \$USER"
            if ask_permission "Docker" "$install_cmd"; then
                print_install "Installing Docker..."
                curl -fsSL https://get.docker.com | sh
                sudo usermod -aG docker $USER
                print_warning "Please log out and log back in for Docker permissions to take effect"
                return 0
            fi
            ;;
        "centos"|"fedora")
            local install_cmd="curl -fsSL https://get.docker.com | sh && sudo usermod -aG docker \$USER && sudo systemctl start docker && sudo systemctl enable docker"
            if ask_permission "Docker" "$install_cmd"; then
                print_install "Installing Docker..."
                curl -fsSL https://get.docker.com | sh
                sudo usermod -aG docker $USER
                sudo systemctl start docker
                sudo systemctl enable docker
                print_warning "Please log out and log back in for Docker permissions to take effect"
                return 0
            fi
            ;;
        "macos")
            print_error "Please install Docker Desktop for Mac from https://docs.docker.com/desktop/mac/install/"
            ;;
        "arch")
            local install_cmd="sudo pacman -S docker docker-compose && sudo systemctl start docker && sudo systemctl enable docker && sudo usermod -aG docker \$USER"
            if ask_permission "Docker" "$install_cmd"; then
                print_install "Installing Docker..."
                sudo pacman -S docker docker-compose
                sudo systemctl start docker
                sudo systemctl enable docker
                sudo usermod -aG docker $USER
                print_warning "Please log out and log back in for Docker permissions to take effect"
                return 0
            fi
            ;;
    esac
    return 1
}

# Function to install Docker Compose
install_docker_compose() {
    local os=$1
    
    if docker compose version &>/dev/null; then
        return 0  # Docker Compose plugin is already available
    fi
    
    case $os in
        "ubuntu"|"debian"|"centos"|"fedora"|"arch"|"linux")
            local install_cmd="sudo curl -L \"https://github.com/docker/compose/releases/latest/download/docker-compose-\$(uname -s)-\$(uname -m)\" -o /usr/local/bin/docker-compose && sudo chmod +x /usr/local/bin/docker-compose"
            if ask_permission "Docker Compose" "$install_cmd"; then
                print_install "Installing Docker Compose..."
                sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
                sudo chmod +x /usr/local/bin/docker-compose
                return 0
            fi
            ;;
        "macos")
            print_status "Docker Compose should be included with Docker Desktop for Mac"
            ;;
    esac
    return 1
}

# Detect operating system
OS=$(detect_os)
print_status "Detected OS: $OS"

# Check and install system dependencies
if $SKIP_DEPS; then
    print_status "Skipping dependency checks (--skip-deps enabled)"
else
    print_status "Checking and installing system dependencies..."
fi

MISSING_DEPS=()
FAILED_INSTALLS=()

if ! $SKIP_DEPS; then
    # Check Go
    if ! command_exists go; then
        print_warning "Go is not installed"
        if install_go "$OS"; then
            print_success "Go installed successfully"
            # Reload environment
            export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
        else
            MISSING_DEPS+=("Go")
            FAILED_INSTALLS+=("Go: https://golang.org/dl/")
        fi
    else
        print_success "Go is already installed ($(go version | awk '{print $3}'))"
    fi

    # Check Node.js
    if ! command_exists node; then
        print_warning "Node.js is not installed"
        if install_node "$OS"; then
            print_success "Node.js installed successfully"
        else
            MISSING_DEPS+=("Node.js")
            FAILED_INSTALLS+=("Node.js: https://nodejs.org/")
        fi
    else
        print_success "Node.js is already installed ($(node --version))"
    fi

    # Check npm (usually comes with Node.js)
    if ! command_exists npm; then
        if command_exists node; then
            print_warning "npm is missing but Node.js is installed - this is unusual"
        fi
        MISSING_DEPS+=("npm")
        FAILED_INSTALLS+=("npm: Usually comes with Node.js")
    else
        print_success "npm is already installed ($(npm --version))"
    fi

    # Check Docker
    if ! command_exists docker; then
        print_warning "Docker is not installed"
        if install_docker "$OS"; then
            print_success "Docker installed successfully"
            # Check if we need to start Docker service
            if ! docker info >/dev/null 2>&1; then
                print_warning "Docker service is not running. Attempting to start..."
                if command_exists systemctl; then
                    sudo systemctl start docker || true
                fi
            fi
        else
            MISSING_DEPS+=("Docker")
            FAILED_INSTALLS+=("Docker: https://docs.docker.com/get-docker/")
        fi
    else
        print_success "Docker is already installed ($(docker --version | cut -d' ' -f3 | tr -d ','))"
    fi

    # Check Docker Compose
    docker_compose_available=false
    if command_exists docker-compose; then
        docker_compose_available=true
        print_success "Docker Compose is already installed ($(docker-compose --version | cut -d' ' -f3 | tr -d ','))"
    elif docker compose version &>/dev/null; then
        docker_compose_available=true
        print_success "Docker Compose (plugin) is already available"
    else
        print_warning "Docker Compose is not installed"
        if install_docker_compose "$OS"; then
            print_success "Docker Compose installed successfully"
            docker_compose_available=true
        else
            MISSING_DEPS+=("Docker Compose")
            FAILED_INSTALLS+=("Docker Compose: https://docs.docker.com/compose/install/")
        fi
    fi

    # Final dependency check
    if [ ${#MISSING_DEPS[@]} -ne 0 ]; then
        echo ""
        print_error "❌ The following dependencies could not be installed automatically:"
        for dep in "${FAILED_INSTALLS[@]}"; do
            echo -e "   ${RED}•${NC} $dep"
        done
        echo ""
        print_error "Please install these dependencies manually and run the script again."
        exit 1
    fi

    print_success "✅ All system dependencies are available!"
else
    print_success "✅ Dependency checks skipped (assumed to be available)"
fi

# Setup container environment (Docker/Podman)
print_status "Configuring container environment..."
setup_container_env

# Create necessary directories
print_status "Creating necessary directories..."
mkdir -p uploads screenshots logs/pid

# Setup environment variables
if [ ! -f .env ]; then
    print_warning ".env file not found. Creating from defaults..."
    cat > .env << 'EOF'
# Database Configuration
DB_HOST=localhost
DB_PORT=3306
DB_USERNAME=webtestflow
DB_PASSWORD=123456
DB_NAME=webtestflow

# Server Configuration
SERVER_PORT=8080
SERVER_HOST=0.0.0.0
SERVER_MODE=debug

# JWT Configuration
JWT_SECRET=webtestflow-secret-key-change-in-production
JWT_EXPIRE_TIME=86400

# Chrome Configuration
CHROME_HEADLESS=false
CHROME_MAX_INSTANCES=10
CHROME_DEBUG_PORT=9222

# OCR Service Configuration
OCR_SERVICE_URL=http://localhost:8888
OCR_ENABLED=true
OCR_TIMEOUT=10

# ADB Configuration (for SMS verification codes)
ADB_ENABLED=true
ADB_TIMEOUT=60

# Captcha Configuration
CAPTCHA_DEFAULT_TIMEOUT=60
CAPTCHA_MAX_RETRIES=3

# Timeout Configuration
SERVER_READ_TIMEOUT=30
SERVER_WRITE_TIMEOUT=30
EOF
    print_success "Created .env file with default values"
else
    print_status ".env file already exists"
fi

# Function to check if local MySQL/MariaDB is running
check_local_mysql_running() {
    # Check if MySQL service is running
    if systemctl is-active --quiet mysqld 2>/dev/null; then
        return 0
    fi
    
    # Check if MariaDB service is running
    if systemctl is-active --quiet mariadb 2>/dev/null; then
        return 0
    fi
    
    # Check if MySQL is running on port 3306
    if netstat -ln 2>/dev/null | grep -q ":3306 "; then
        return 0
    fi
    
    # Alternative check using ss command if netstat is not available
    if ss -ln 2>/dev/null | grep -q ":3306 "; then
        return 0
    fi
    
    return 1
}

# Function to test MySQL connection
test_mysql_connection() {
    local host=${1:-localhost}
    local port=${2:-3306}
    local user=${3:-webtestflow}
    local password=${4:-123456}
    local database=${5:-webtestflow}
    
    # Try to connect to MySQL
    if command_exists mysql; then
        mysql -h"$host" -P"$port" -u"$user" -p"$password" -e "SELECT 1;" "$database" 2>/dev/null
        return $?
    fi
    
    return 1
}

# Function to setup local MySQL database and user
setup_local_mysql() {
    print_status "Setting up local MySQL database and user..."
    
    # Check if we can connect as root
    local root_password=""
    if mysql -uroot -e "SELECT 1;" 2>/dev/null; then
        root_password=""
    else
        print_status "MySQL root password required. Trying common passwords..."
        for pwd in "" "root" "123456" "password"; do
            if mysql -uroot -p"$pwd" -e "SELECT 1;" 2>/dev/null; then
                root_password="$pwd"
                break
            fi
        done
        
        if [[ -z "$root_password" ]]; then
            print_warning "Cannot connect to MySQL as root. Please ensure:"
            echo "1. MySQL root user is accessible"
            echo "2. Run: mysql -uroot -p"
            echo "3. Execute the following commands:"
            echo "   CREATE DATABASE IF NOT EXISTS webtestflow;"
            echo "   CREATE USER IF NOT EXISTS 'webtestflow'@'localhost' IDENTIFIED BY '123456';"
            echo "   GRANT ALL PRIVILEGES ON webtestflow.* TO 'webtestflow'@'localhost';"
            echo "   FLUSH PRIVILEGES;"
            return 1
        fi
    fi
    
    # Setup database and user
    local mysql_cmd="mysql -uroot"
    if [[ -n "$root_password" ]]; then
        mysql_cmd="mysql -uroot -p$root_password"
    fi
    
    $mysql_cmd -e "
        CREATE DATABASE IF NOT EXISTS webtestflow;
        CREATE USER IF NOT EXISTS 'webtestflow'@'localhost' IDENTIFIED BY '123456';
        GRANT ALL PRIVILEGES ON webtestflow.* TO 'webtestflow'@'localhost';
        FLUSH PRIVILEGES;
    " 2>/dev/null
    
    if [ $? -eq 0 ]; then
        print_success "Local MySQL database and user setup completed"
        return 0
    else
        print_error "Failed to setup local MySQL database"
        return 1
    fi
}

# Function to check if MySQL is running via Docker
check_mysql_running() {
    if docker ps --format "table {{.Names}}" | grep -q "webtestflow-mysql"; then
        return 0
    else
        return 1
    fi
}

# Function to try different MySQL images
try_mysql_images() {
    local images=(
        "mysql:8.0"
        "mysql:8.0-debian" 
        "mysql:latest"
        "mariadb:10.11"
        "mariadb:latest"
    )
    
    print_status "Trying different MySQL/MariaDB images due to network issues..."
    
    for image in "${images[@]}"; do
        print_status "Attempting to pull $image..."
        
        # Try to pull the image with timeout
        if timeout 60s docker pull "$image" 2>/dev/null; then
            print_success "Successfully pulled $image"
            
            # Update docker-compose.yml to use this image
            if [[ -f "docker-compose.yml" ]]; then
                print_status "Updating docker-compose.yml to use $image"
                sed -i "s|image: mysql:.*|image: $image|g" docker-compose.yml
                return 0
            fi
        else
            print_warning "Failed to pull $image, trying next..."
        fi
    done
    
    print_error "All MySQL image pulls failed due to network issues"
    return 1
}

# Function to start MySQL with fallback options
start_mysql_with_fallback() {
    local max_retries=3
    local retry_count=0
    
    while [ $retry_count -lt $max_retries ]; do
        retry_count=$((retry_count + 1))
        print_status "MySQL startup attempt $retry_count/$max_retries"
        
        # Try to start MySQL container
        if run_docker_compose up -d mysql; then
            print_success "MySQL container started successfully"
            return 0
        else
            print_warning "MySQL startup failed, attempt $retry_count/$max_retries"
            
            # On failure, try different images if it's a pull issue
            if [[ $retry_count -eq 1 ]]; then
                if try_mysql_images; then
                    print_status "Retrying with different MySQL image..."
                    continue
                fi
            fi
            
            # Clean up failed containers
            print_status "Cleaning up failed containers..."
            docker rm -f webtestflow-mysql 2>/dev/null || true
            
            if [ $retry_count -lt $max_retries ]; then
                print_status "Waiting 10 seconds before retry..."
                sleep 10
            fi
        fi
    done
    
    print_error "Failed to start MySQL after $max_retries attempts"
    
    # Offer manual alternatives
    echo ""
    print_warning "Alternative solutions:"
    echo "1. Use system MySQL: sudo systemctl start mysqld"
    echo "2. Try manual Docker command: docker run -d --name webtestflow-mysql -e MYSQL_ROOT_PASSWORD=123456 -p 3306:3306 mysql:8.0"
    echo "3. Use different registry: docker pull registry.cn-hangzhou.aliyuncs.com/library/mysql:8.0"
    echo ""
    
    return 1
}

# Function to run Docker Compose commands
run_docker_compose() {
    # Ensure Docker environment is set for Podman if needed
    if command_exists podman && ! command_exists docker && [[ -z "$DOCKER_HOST" ]]; then
        export DOCKER_HOST="unix:///run/user/$(id -u)/podman/podman.sock"
    fi
    
    local compose_cmd=""
    local use_sudo=""
    
    # Check if we need to use sudo
    if [[ "${USE_SUDO_PODMAN:-}" == "true" ]]; then
        use_sudo="sudo -E"  # Preserve environment variables
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
        print_status "Running: $use_sudo $compose_cmd $*"
        $use_sudo $compose_cmd "$@"
    else
        $compose_cmd "$@"
    fi
}

# Start MySQL database - check local first, then Docker
print_status "Starting MySQL database..."

# First, check if local MySQL/MariaDB is running
if check_local_mysql_running; then
    print_success "Local MySQL/MariaDB is already running"
    
    # Test if we can connect with our credentials
    if test_mysql_connection; then
        print_success "MySQL connection test successful"
    else
        print_warning "Local MySQL is running but webtestflow database/user not found"
        if ask_permission "Setup webtestflow database and user in local MySQL" "Setup now"; then
            if setup_local_mysql; then
                print_success "Local MySQL setup completed"
            else
                print_warning "Local MySQL setup failed, falling back to Docker MySQL"
                # Fall through to Docker setup
            fi
        else
            print_warning "Falling back to Docker MySQL"
            # Fall through to Docker setup
        fi
    fi
    
    # If local MySQL is working, skip Docker setup
    if test_mysql_connection; then
        echo ""
        print_success "✅ Using local MySQL database"
        MYSQL_TYPE="local"
    else
        print_warning "Local MySQL setup failed, trying Docker MySQL..."
        MYSQL_TYPE="docker"
    fi
else
    print_status "No local MySQL/MariaDB found, using Docker MySQL"
    MYSQL_TYPE="docker"
fi

# If we need Docker MySQL, start it
if [[ "$MYSQL_TYPE" == "docker" ]]; then
    if check_mysql_running; then
        print_success "MySQL container is already running"
    else
        print_status "Starting MySQL container..."
        
        # Use enhanced startup with fallbacks
        if start_mysql_with_fallback; then
            # Wait for MySQL to be ready
            print_status "Waiting for MySQL to be ready..."
            counter=0
            max_wait=60
            
            while [ $counter -lt $max_wait ]; do
                if docker exec webtestflow-mysql mysqladmin ping -h localhost --silent 2>/dev/null; then
                    print_success "MySQL is ready!"
                    break
                fi
                sleep 2
                counter=$((counter + 2))
                echo -n "."
            done
            
            if [ $counter -ge $max_wait ]; then
                print_error "MySQL failed to be ready within $max_wait seconds"
                print_status "But container may still be starting. Check logs: docker logs webtestflow-mysql"
                print_warning "Continuing with backend/frontend startup..."
            fi
        else
            print_error "Failed to start MySQL container"
            echo ""
            if ask_permission "Continue without MySQL (will cause backend to fail)" "Continue anyway"; then
                print_warning "Continuing without MySQL - backend will likely fail to connect"
            else
                print_error "Cannot continue without database. Please fix MySQL issue first."
                exit 1
            fi
        fi
    fi
fi

# Setup backend dependencies
print_status "Setting up backend dependencies..."
cd backend
if [ ! -f "go.sum" ] || [ "go.mod" -nt "go.sum" ]; then
    print_status "Installing Go dependencies..."
    go mod tidy
    go mod download
fi
print_success "Backend dependencies ready"

# Setup frontend dependencies
print_status "Setting up frontend dependencies..."
cd ../frontend
if [ ! -d "node_modules" ] || [ "package.json" -nt "node_modules" ]; then
    print_status "Installing Node.js dependencies..."
    npm install
fi
print_success "Frontend dependencies ready"

# Go back to root
cd ..

# Start OCR service (if enabled)
start_ocr_service() {
    print_status "Starting OCR service..."
    
    # Check if OCR service is already running
    if curl -s http://localhost:8888/health > /dev/null 2>&1; then
        print_success "OCR service is already running"
        return 0
    fi
    
    # Priority 1: Local Python (fastest startup)
    if command_exists python3 && [ -d "services/ocr" ]; then
        print_status "Starting OCR service with local Python..."
        cd services/ocr
        
        # Check if virtual environment exists
        if [ ! -d "venv" ]; then
            print_status "Creating Python virtual environment for OCR..."
            python3 -m venv venv
        fi
        
        # Activate virtual environment and install dependencies
        source venv/bin/activate
        if [ -f "requirements.txt" ]; then
            print_status "Installing OCR dependencies..."
            pip install -r requirements.txt > /dev/null 2>&1 || {
                print_warning "Some Python packages failed to install, OCR may not work properly"
            }
        fi
        
        # Start OCR server in background
        print_status "Starting OCR server (Python)..."
        nohup python3 ocr_server.py > ../../logs/ocr.log 2>&1 &
        OCR_PID=$!
        cd ../..
        
        # Save PID
        echo $OCR_PID > logs/pid/ocr.pid
        
        # Wait for service to start with retries
        print_status "Waiting for OCR service to be ready..."
        local retry_count=0
        local max_retries=10
        
        while [ $retry_count -lt $max_retries ]; do
            sleep 1
            if curl -s http://localhost:8888/health > /dev/null 2>&1; then
                print_success "OCR service started successfully with Python (PID: $OCR_PID)"
                return 0
            fi
            retry_count=$((retry_count + 1))
            echo -n "."
        done
        
        # Final check after retries
        if curl -s http://localhost:8888/health > /dev/null 2>&1; then
            print_success "OCR service started successfully with Python (PID: $OCR_PID)"
            return 0
        else
            print_warning "OCR service failed to start properly, check logs/ocr.log"
        fi
    fi
    
    # Priority 2: Docker Compose (if available and user prefers containers)
    if command_exists docker && [ -f "docker-compose.yml" ] && grep -q "ocr-service" docker-compose.yml; then
        print_status "Trying OCR service with Docker Compose..."
        if run_docker_compose up -d ocr-service 2>/dev/null; then
            print_success "OCR service started via Docker Compose"
            return 0
        else
            print_warning "Docker Compose OCR startup failed"
        fi
    fi
    
    # Priority 3: Standalone Docker (if image exists)
    if command_exists docker && docker images | grep -q "webtestflow-ocr"; then
        print_status "Trying OCR service with standalone Docker..."
        docker rm -f webtestflow-ocr 2>/dev/null || true
        if docker run -d \
            --name webtestflow-ocr \
            --restart unless-stopped \
            -p 8888:8888 \
            -e OCR_HOST=0.0.0.0 \
            -e OCR_PORT=8888 \
            webtestflow-ocr:latest > /dev/null 2>&1; then
            print_success "OCR service started via standalone Docker"
            return 0
        else
            print_warning "Standalone Docker OCR startup failed"
        fi
    fi
    
    print_warning "OCR service could not be started"
    print_status "图形验证码识别功能将不可用"
    print_status "💡 提示: 可以稍后使用 ./scripts/deploy-captcha.sh 单独部署OCR服务"
    return 1
}

# Start OCR service
start_ocr_service

# Start backend service
print_status "Starting backend service..."
cd backend
nohup go run cmd/main.go > ../logs/backend.log 2>&1 &
BACKEND_PID=$!
cd ..

# Start frontend service
print_status "Starting frontend service..."
cd frontend
nohup npm start > ../logs/frontend.log 2>&1 &
FRONTEND_PID=$!
cd ..

# Save PIDs for later cleanup
echo $BACKEND_PID > logs/pid/backend.pid
echo $FRONTEND_PID > logs/pid/frontend.pid

# Function to check OCR service status
check_ocr_service() {
    if curl -s http://localhost:8888/health > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Display startup information
echo ""
print_success "🎉 WebTestFlow Complete Environment Started!"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${BLUE}📊 Service Information:${NC}"
echo "   MySQL PID: Container (webtestflow-mysql)"
echo "   Backend PID: $BACKEND_PID"
echo "   Frontend PID: $FRONTEND_PID"
if [ -f "logs/pid/ocr.pid" ]; then
    OCR_PID=$(cat logs/pid/ocr.pid 2>/dev/null)
    echo "   OCR Service PID: $OCR_PID (Python)"
else
    echo "   OCR Service: Docker Container 或 未启动"
fi
echo ""
echo -e "${BLUE}🌐 Access URLs:${NC}"
echo "   🖥️  Frontend Application: http://localhost:3000"
echo "   🔌 Backend API: http://localhost:8080/api/v1"
echo "   ❤️  Health Check: http://localhost:8080/api/v1/health"
echo "   🔍 OCR Service: http://localhost:8888/health"
echo "   🗄️  MySQL Database: localhost:3306 (webtestflow/123456)"
echo ""
echo -e "${BLUE}📋 Useful Commands:${NC}"
echo "   📖 View Backend Logs: tail -f logs/backend.log"
echo "   📖 View Frontend Logs: tail -f logs/frontend.log"
echo "   📖 View OCR Logs: tail -f logs/ocr.log"
echo "   📖 View All Logs: tail -f logs/*.log"
echo "   🔍 Check MySQL: docker exec -it webtestflow-mysql mysql -u webtestflow -p123456 webtestflow
   🔍 Check Local MySQL: mysql -u webtestflow -p123456 webtestflow"
echo ""
echo -e "${BLUE}🛑 Stop Services:${NC}"
echo "   ./scripts/stop-dev.sh        # Stop backend and frontend only"
echo "   ./scripts/one-click-stop.sh  # Stop everything including database"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Wait a bit and check service status
sleep 5

print_status "Checking services status..."

# Check MySQL
if [[ "$MYSQL_TYPE" == "local" ]]; then
    if check_local_mysql_running && test_mysql_connection; then
        print_success "✅ MySQL service is running (Local MySQL/MariaDB)"
    else
        print_error "❌ Local MySQL service failed or connection test failed"
    fi
elif [[ "$MYSQL_TYPE" == "docker" ]]; then
    if check_mysql_running; then
        print_success "✅ MySQL service is running (Container: webtestflow-mysql)"
    else
        print_error "❌ MySQL container failed to start"
    fi
else
    print_error "❌ MySQL service status unknown"
fi

# Check OCR Service
if check_ocr_service; then
    if [ -f "logs/pid/ocr.pid" ]; then
        OCR_PID=$(cat logs/pid/ocr.pid 2>/dev/null)
        if kill -0 $OCR_PID 2>/dev/null; then
            print_success "✅ OCR service is running (Python PID: $OCR_PID)"
        else
            print_success "✅ OCR service is running (Docker Container)"
        fi
    else
        print_success "✅ OCR service is running (Docker Container)"
    fi
else
    print_warning "⚠️ OCR service is not running - 图形验证码功能不可用"
    if [ -f "logs/pid/ocr.pid" ]; then
        print_status "提示: 检查 logs/ocr.log 查看启动错误"
    fi
    print_status "可以使用 ./scripts/deploy-captcha.sh 单独部署OCR服务"
fi

# Check Backend
if kill -0 $BACKEND_PID 2>/dev/null; then
    print_success "✅ Backend service is running (PID: $BACKEND_PID)"
else
    print_error "❌ Backend service failed to start"
    print_error "Check logs: tail logs/backend.log"
fi

# Check Frontend
if kill -0 $FRONTEND_PID 2>/dev/null; then
    print_success "✅ Frontend service is running (PID: $FRONTEND_PID)"
else
    print_error "❌ Frontend service failed to start"
    print_error "Check logs: tail logs/frontend.log"
fi

echo ""
print_success "🚀 Setup complete! Your WebTestFlow environment is ready!"
print_status "💡 Tip: Run 'tail -f logs/*.log' to monitor all services at once"
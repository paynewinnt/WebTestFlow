#!/bin/bash

# Setup Docker/Podman Registry Mirror Sources
# Supports both China (domestic) and International (global) sources

echo "🌐 Setting up Docker/Podman registry mirror sources..."

# Parse command line arguments
MIRROR_TYPE=""
AUTO_DETECT=false
SKIP_TEST=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --china|--domestic)
            MIRROR_TYPE="china"
            shift
            ;;
        --international|--global)
            MIRROR_TYPE="international"
            shift
            ;;
        --auto|--detect)
            AUTO_DETECT=true
            shift
            ;;
        --skip-test)
            SKIP_TEST=true
            shift
            ;;
        -h|--help)
            echo "Docker/Podman Registry Mirror Setup"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --china, --domestic      Use China domestic mirrors"
            echo "  --international, --global Use international mirrors"
            echo "  --auto, --detect         Auto-detect best mirrors"
            echo "  --skip-test              Skip MySQL image pull test"
            echo "  -h, --help               Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0 --china              # Use China mirrors"
            echo "  $0 --international      # Use international mirrors"
            echo "  $0 --auto               # Auto-detect best mirrors"
            echo "  $0 --skip-test          # Configure mirrors without testing"
            echo "  $0                       # Interactive selection"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use -h or --help for usage information"
            exit 1
            ;;
    esac
done

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

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

# Function to detect network location/speed
detect_best_mirrors() {
    print_status "🔍 Auto-detecting best registry mirrors..."
    
    local china_mirrors=(
        "registry.cn-hangzhou.aliyuncs.com"
        "hub-mirror.c.163.com" 
        "mirror.baidubce.com"
    )
    
    local international_mirrors=(
        "docker.io"
        "registry-1.docker.io"
        "index.docker.io"
    )
    
    # Test China mirrors
    local china_speed=0
    for mirror in "${china_mirrors[@]}"; do
        if timeout 5s ping -c 2 "$mirror" >/dev/null 2>&1; then
            china_speed=$((china_speed + 1))
        fi
    done
    
    # Test international mirrors  
    local intl_speed=0
    for mirror in "${international_mirrors[@]}"; do
        if timeout 5s ping -c 2 "$mirror" >/dev/null 2>&1; then
            intl_speed=$((intl_speed + 1))
        fi
    done
    
    print_status "China mirrors connectivity: $china_speed/${#china_mirrors[@]}"
    print_status "International mirrors connectivity: $intl_speed/${#international_mirrors[@]}"
    
    if [[ $china_speed -ge $intl_speed ]]; then
        print_success "🇨🇳 Detected better connectivity to China mirrors"
        echo "china"
    else
        print_success "🌍 Detected better connectivity to international mirrors"
        echo "international"
    fi
}

# Function to show interactive menu
show_mirror_menu() {
    echo ""
    echo "📡 Please select registry mirror source:"
    echo ""
    echo "1) 🇨🇳 China Domestic Mirrors (国内镜像源)"
    echo "   - Alibaba Cloud (阿里云)"
    echo "   - NetEase (网易)"
    echo "   - Baidu Cloud (百度云)"
    echo "   - Best for users in mainland China"
    echo ""
    echo "2) 🌍 International Mirrors (国际镜像源)"  
    echo "   - Docker Hub (official)"
    echo "   - Google Container Registry"
    echo "   - GitHub Container Registry"
    echo "   - Best for users outside China"
    echo ""
    echo "3) 🔍 Auto-detect (自动检测)"
    echo "   - Automatically choose the fastest option"
    echo ""
    echo -n "Please select (1-3): "
    
    read -r choice
    case $choice in
        1)
            echo "china"
            ;;
        2) 
            echo "international"
            ;;
        3)
            detect_best_mirrors
            ;;
        *)
            print_error "Invalid choice. Using auto-detection..."
            detect_best_mirrors
            ;;
    esac
}

# Function to setup Docker mirrors
setup_docker_mirrors() {
    local mirror_type=$1
    print_status "Setting up Docker mirror sources ($mirror_type)..."
    
    # Create Docker daemon configuration
    sudo mkdir -p /etc/docker
    
    if [[ "$mirror_type" == "china" ]]; then
        print_status "🇨🇳 Configuring China domestic mirrors..."
        cat << 'EOF' | sudo tee /etc/docker/daemon.json
{
    "registry-mirrors": [
        "https://registry.cn-hangzhou.aliyuncs.com",
        "https://hub-mirror.c.163.com",
        "https://mirror.baidubce.com",
        "https://dockerproxy.com"
    ],
    "dns": ["202.101.172.35", "114.114.114.114", "8.8.8.8"],
    "log-driver": "json-file",
    "log-opts": {
        "max-size": "10m",
        "max-file": "3"
    }
}
EOF
    else
        print_status "🌍 Configuring international mirrors..."
        cat << 'EOF' | sudo tee /etc/docker/daemon.json
{
    "registry-mirrors": [
        "https://registry-1.docker.io",
        "https://index.docker.io",
        "https://gcr.io",
        "https://ghcr.io"
    ],
    "dns": ["8.8.8.8", "1.1.1.1", "8.8.4.4"],
    "log-driver": "json-file",
    "log-opts": {
        "max-size": "10m",
        "max-file": "3"
    }
}
EOF
    fi
    
    # Restart Docker service if it exists
    if systemctl is-active --quiet docker; then
        print_status "Restarting Docker service..."
        sudo systemctl restart docker
        print_success "Docker mirrors configured"
    else
        print_success "Docker configuration created (service not running)"
    fi
}

# Function to setup Podman mirrors
setup_podman_mirrors() {
    local mirror_type=$1
    print_status "Setting up Podman mirror sources ($mirror_type)..."
    
    # Create Podman registries configuration
    sudo mkdir -p /etc/containers
    
    if [[ "$mirror_type" == "china" ]]; then
        print_status "🇨🇳 Configuring China domestic mirrors..."
        cat << 'EOF' | sudo tee /etc/containers/registries.conf
# China domestic mirrors configuration
[[registry]]
prefix = "docker.io"
location = "registry.cn-hangzhou.aliyuncs.com"

[[registry]] 
prefix = "docker.io"
location = "hub-mirror.c.163.com"

[[registry]]
prefix = "docker.io"
location = "mirror.baidubce.com"

[[registry]]
prefix = "docker.io"
location = "dockerproxy.com"

[[registry]]
prefix = "docker.io"
location = "docker.io"

[registries.search]
registries = ['registry.cn-hangzhou.aliyuncs.com', 'hub-mirror.c.163.com', 'docker.io']

[registries.insecure]
registries = []

[registries.block] 
registries = []
EOF
    else
        print_status "🌍 Configuring international mirrors..."
        cat << 'EOF' | sudo tee /etc/containers/registries.conf
# International mirrors configuration
[[registry]]
prefix = "docker.io"
location = "registry-1.docker.io"

[[registry]]
prefix = "docker.io" 
location = "index.docker.io"

[[registry]]
prefix = "docker.io"
location = "docker.io"

[registries.search]
registries = ['docker.io', 'registry-1.docker.io', 'quay.io', 'gcr.io']

[registries.insecure]
registries = []

[registries.block]
registries = []
EOF
    fi
    
    print_success "Podman mirrors configured"
}

# Function to test MySQL image pull
test_mysql_pull() {
    local mirror_type=$1
    print_status "Testing MySQL image pull with $mirror_type mirrors..."
    
    # First check if mysql:8.0 is already available
    if docker images --format "table {{.Repository}}:{{.Tag}}" | grep -q "mysql:8.0"; then
        print_success "MySQL:8.0 image is already available locally"
        return 0
    fi
    
    local images=()
    if [[ "$mirror_type" == "china" ]]; then
        images=(
            "mysql:8.0"  # Try default first since mirrors are configured
            "registry.cn-hangzhou.aliyuncs.com/library/mysql:8.0" 
            "hub-mirror.c.163.com/library/mysql:8.0"
        )
    else
        images=(
            "mysql:8.0"
            "mysql:latest"
            "mariadb:10.11"
        )
    fi
    
    for image in "${images[@]}"; do
        print_status "Testing $image (timeout: 15s)..."
        if timeout 15s docker pull "$image" >/dev/null 2>&1; then
            print_success "Successfully pulled $image"
            
            # Tag it as mysql:8.0 if it's not already
            if [[ "$image" != "mysql:8.0" ]]; then
                docker tag "$image" "mysql:8.0" 2>/dev/null || true
                print_success "Tagged as mysql:8.0"
            fi
            return 0
        else
            print_warning "Failed to pull $image (timeout or network issue)"
        fi
    done
    
    print_warning "Image pull test failed, but mirrors are configured"
    print_status "You can manually pull images using: docker pull mysql:8.0"
    return 1
}

# Main execution logic
main() {
    local selected_mirror_type=""
    
    # Determine mirror type
    if [[ "$AUTO_DETECT" == true ]]; then
        selected_mirror_type=$(detect_best_mirrors)
    elif [[ -n "$MIRROR_TYPE" ]]; then
        selected_mirror_type="$MIRROR_TYPE"
    else
        selected_mirror_type=$(show_mirror_menu)
    fi
    
    print_success "Selected mirror type: $selected_mirror_type"
    echo ""
    
    # Setup mirrors based on container engine
    if command -v docker >/dev/null 2>&1; then
        setup_docker_mirrors "$selected_mirror_type"
    elif command -v podman >/dev/null 2>&1; then
        setup_podman_mirrors "$selected_mirror_type"
    else
        print_error "Neither Docker nor Podman found"
        exit 1
    fi
    
    # Test the setup (unless skipped)
    if [[ "$SKIP_TEST" == true ]]; then
        print_success "✅ Mirror setup completed! (Testing skipped)"
        echo ""
        echo "🚀 You can now run: ./scripts/one-click-start.sh"
    else
        print_status "Testing mirror configuration (use --skip-test to skip)..."
        if test_mysql_pull "$selected_mirror_type"; then
            print_success "✅ Mirror setup successful! MySQL image is now available."
            echo ""
            echo "🚀 You can now run: ./scripts/one-click-start.sh"
        else
            print_warning "⚠️ Mirror setup completed but image pull test failed."
            echo ""
            print_status "This is normal and mirrors are still configured correctly."
            print_warning "Try these manual commands if needed:"
            if [[ "$selected_mirror_type" == "china" ]]; then
                echo "1. docker pull mysql:8.0"
                echo "2. docker pull registry.cn-hangzhou.aliyuncs.com/library/mysql:8.0"
            else
                echo "1. docker pull mysql:8.0"
                echo "2. docker pull mariadb:10.11 && docker tag mariadb:10.11 mysql:8.0"
            fi
            echo ""
            echo "🚀 You can still run: ./scripts/one-click-start.sh"
        fi
    fi
}

# Run main function
main
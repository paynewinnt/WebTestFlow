#!/bin/bash

# AutoUI Platform Deployment Script

set -e

echo "🚀 Starting AutoUI Platform deployment..."

# Check if Docker and Docker Compose are installed
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed. Please install Docker first."
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    echo "❌ Docker Compose is not installed. Please install Docker Compose first."
    exit 1
fi

# Create environment file if it doesn't exist
if [ ! -f .env ]; then
    echo "📝 Creating .env file from template..."
    cp .env.example .env
    echo "⚠️  Please edit .env file to configure your environment variables."
fi

# Pull latest images
echo "📦 Pulling latest Docker images..."
docker-compose pull

# Build the application
echo "🔨 Building AutoUI Platform..."
docker-compose build

# Start the services
echo "🎯 Starting services..."
docker-compose up -d

# Wait for services to be healthy
echo "⏳ Waiting for services to be ready..."
sleep 30

# Check service status
echo "📊 Checking service status..."
docker-compose ps

# Show logs
echo "📋 Recent logs:"
docker-compose logs --tail=20

echo "✅ AutoUI Platform has been deployed successfully!"
echo ""
echo "🌐 Access the application:"
echo "   Frontend: http://localhost"
echo "   Backend API: http://localhost/api/v1"
echo "   Health Check: http://localhost/health"
echo ""
echo "📚 Useful commands:"
echo "   View logs: docker-compose logs -f"
echo "   Stop services: docker-compose down"
echo "   Restart services: docker-compose restart"
echo "   Update: docker-compose pull && docker-compose up -d"
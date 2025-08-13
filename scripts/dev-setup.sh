#!/bin/bash

# AutoUI Platform Development Setup Script

set -e

echo "🛠️  Setting up AutoUI Platform development environment..."

# Check system requirements
echo "🔍 Checking system requirements..."

# Check Go
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21+ first."
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
if [[ "$(printf '%s\n' "1.21" "$GO_VERSION" | sort -V | head -n1)" != "1.21" ]]; then
    echo "❌ Go version $GO_VERSION is too old. Please install Go 1.21+ first."
    exit 1
fi

# Check Node.js
if ! command -v node &> /dev/null; then
    echo "❌ Node.js is not installed. Please install Node.js 18+ first."
    exit 1
fi

NODE_VERSION=$(node --version | sed 's/v//')
if [[ "$(printf '%s\n' "18.0.0" "$NODE_VERSION" | sort -V | head -n1)" != "18.0.0" ]]; then
    echo "❌ Node.js version $NODE_VERSION is too old. Please install Node.js 18+ first."
    exit 1
fi

# Check MySQL
if ! command -v mysql &> /dev/null; then
    echo "⚠️  MySQL client is not installed. Please install MySQL 8.0+ or ensure it's accessible."
fi

echo "✅ System requirements check passed!"

# Setup backend
echo "🔧 Setting up backend..."
cd backend
go mod tidy
go mod download
echo "✅ Backend dependencies installed!"

# Setup frontend
echo "🎨 Setting up frontend..."
cd ../frontend
npm install
echo "✅ Frontend dependencies installed!"

# Go back to root
cd ..

# Create environment file

if [ ! -f .env ]; then
    echo "📝 Creating .env file..."
    cp .env.example .env
    echo "✅ Environment file created!"
else
    echo "ℹ️  .env file already exists"
fi

# Create necessary directories
echo "📁 Creating necessary directories..."
mkdir -p uploads screenshots logs
echo "✅ Directories created!"

echo ""
echo "🎉 Development environment setup complete!"
echo ""
echo "🚀 To start development:"
echo "   1. Start MySQL server and create database 'webtestflow'"
echo "   2. Update .env file with your database credentials"
echo "   3. Start backend: cd backend && go run cmd/main.go"
echo "   4. Start frontend: cd frontend && npm start"
echo ""
echo "🌐 Default URLs:"
echo "   Frontend: http://localhost:3000"
echo "   Backend API: http://localhost:8080/api/v1"
echo "   Health Check: http://localhost:8080/api/v1/health"
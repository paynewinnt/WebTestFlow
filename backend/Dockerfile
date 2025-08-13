# Multi-stage build for Go backend
FROM golang:1.21-alpine AS backend-builder

WORKDIR /app

# Install git and build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o main cmd/main.go

# Build stage for frontend
FROM node:18-alpine AS frontend-builder

WORKDIR /app

# Copy package files
COPY frontend/package*.json ./frontend/
WORKDIR /app/frontend
RUN npm ci --only=production

# Copy source code and build
COPY frontend/ .
RUN npm run build

# Final stage
FROM alpine:latest

# Install necessary packages
RUN apk add --no-cache \
    chromium \
    mysql-client \
    ca-certificates \
    tzdata \
    wget

# Set timezone
ENV TZ=Asia/Shanghai

# Create non-root user
RUN addgroup -g 1001 -S webtestflow && \
    adduser -S webtestflow -u 1001 -G webtestflow

# Create app directory
WORKDIR /app

# Copy backend binary
COPY --from=backend-builder /app/main .

# Copy frontend build
COPY --from=frontend-builder /app/frontend/build ./frontend/build

# Create directories for uploads and screenshots
RUN mkdir -p /app/uploads /app/screenshots /app/logs
RUN chown -R webtestflow:webtestflow /app

# Switch to non-root user
USER webtestflow

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/health || exit 1

# Start the application
CMD ["./main"]
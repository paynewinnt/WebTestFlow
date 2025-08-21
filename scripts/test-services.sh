#!/bin/bash

# WebTestFlow 服务快速测试脚本
# 检查所有服务是否正常运行

set -e

echo "🔍 WebTestFlow Services Health Check"
echo "=================================="

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 测试结果统计
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# 测试函数
test_service() {
    local service_name=$1
    local test_command=$2
    local expected_result=$3
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    echo -n "Testing $service_name... "
    
    if eval "$test_command" > /dev/null 2>&1; then
        if [[ -z "$expected_result" ]] || eval "$expected_result" > /dev/null 2>&1; then
            echo -e "${GREEN}✅ PASS${NC}"
            PASSED_TESTS=$((PASSED_TESTS + 1))
            return 0
        else
            echo -e "${RED}❌ FAIL (unexpected result)${NC}"
            FAILED_TESTS=$((FAILED_TESTS + 1))
            return 1
        fi
    else
        echo -e "${RED}❌ FAIL${NC}"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi
}

echo -e "${BLUE}Database Services:${NC}"
# 测试MySQL连接
test_service "MySQL (Docker)" "docker exec webtestflow-mysql mysqladmin ping -h localhost --silent"
test_service "MySQL (Local)" "mysql -h localhost -u webtestflow -p123456 -e 'SELECT 1;' webtestflow"

echo ""
echo -e "${BLUE}Core Services:${NC}"
# 测试后端API
test_service "Backend API" "curl -s http://localhost:8080/api/v1/health"
test_service "Backend Health" "curl -s http://localhost:8080/api/v1/health | grep -q 'healthy'"

# 测试前端应用
test_service "Frontend App" "curl -s http://localhost:3000"

echo ""
echo -e "${BLUE}OCR Services:${NC}"
# 测试OCR服务
test_service "OCR Service" "curl -s http://localhost:8888/health"
test_service "OCR Health" "curl -s http://localhost:8888/health | grep -q 'healthy'"

# 测试OCR识别接口
test_service "OCR Recognition" "curl -s -X POST http://localhost:8888/recognize -H 'Content-Type: application/json' -d '{\"image\": \"test\"}'"

echo ""
echo -e "${BLUE}ADB Services:${NC}"
# 测试ADB连接
test_service "ADB Tool" "command -v adb"
test_service "ADB Devices" "adb devices | grep -q device"

echo ""
echo -e "${BLUE}Container Services:${NC}"
# 测试Docker容器
test_service "MySQL Container" "docker ps | grep -q webtestflow-mysql"
test_service "OCR Container" "docker ps | grep -q 'webtestflow-ocr\|ocr-service'"

echo ""
echo -e "${BLUE}Port Availability:${NC}"
# 测试端口占用
test_service "Port 3000 (Frontend)" "lsof -i:3000 > /dev/null"
test_service "Port 8080 (Backend)" "lsof -i:8080 > /dev/null"
test_service "Port 8888 (OCR)" "lsof -i:8888 > /dev/null"
test_service "Port 3306 (MySQL)" "lsof -i:3306 > /dev/null"

echo ""
echo "=================================="
echo -e "${BLUE}Test Summary:${NC}"
echo "Total Tests: $TOTAL_TESTS"
echo -e "Passed: ${GREEN}$PASSED_TESTS${NC}"
echo -e "Failed: ${RED}$FAILED_TESTS${NC}"

if [[ $FAILED_TESTS -eq 0 ]]; then
    echo ""
    echo -e "${GREEN}🎉 All services are running normally!${NC}"
    echo ""
    echo -e "${BLUE}Service URLs:${NC}"
    echo "🖥️  Frontend: http://localhost:3000"
    echo "🔌 Backend: http://localhost:8080/api/v1"
    echo "🔍 OCR Service: http://localhost:8888/health"
    echo "🗄️  MySQL: localhost:3306"
    exit 0
else
    echo ""
    echo -e "${YELLOW}⚠️ Some services have issues. Common solutions:${NC}"
    echo ""
    echo "1. Restart all services:"
    echo "   ./scripts/one-click-stop.sh"
    echo "   ./scripts/one-click-start.sh"
    echo ""
    echo "2. Check logs:"
    echo "   tail -f logs/*.log"
    echo ""
    echo "3. Check Docker containers:"
    echo "   docker ps -a"
    echo "   docker logs webtestflow-mysql"
    echo "   docker logs webtestflow-ocr"
    echo ""
    echo "4. Check specific services:"
    echo "   ./scripts/test-captcha.sh ocr      # Test OCR only"
    echo "   ./scripts/test-captcha.sh adb      # Test ADB only"
    echo ""
    exit 1
fi
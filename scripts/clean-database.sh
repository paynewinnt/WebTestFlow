#!/bin/bash

# WebTestFlow Database Clean Mode Script
# 用于将数据库恢复到纯净模式，只保留基础配置数据
# Usage: ./clean-database.sh

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 数据库配置
DB_HOST="localhost"
DB_PORT="3306"
DB_USER="root"
DB_PASS="root"
DB_NAME="webtestflow"

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}WebTestFlow 数据库纯净模式恢复脚本${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""

# 确认执行
echo -e "${RED}⚠️  警告：此操作将清除以下数据：${NC}"
echo "  - 所有项目"
echo "  - 所有测试用例"
echo "  - 所有测试套件"
echo "  - 所有执行记录"
echo "  - 所有测试报告"
echo "  - 除admin外的所有用户"
echo "  - 除ID=2外的所有环境配置"
echo ""
echo -e "${GREEN}✅ 将保留以下数据：${NC}"
echo "  - admin用户"
echo "  - 所有设备配置"
echo "  - 环境配置（ID=2）"
echo ""

read -p "确定要执行数据清理吗？(yes/no): " confirm

if [ "$confirm" != "yes" ]; then
    echo -e "${YELLOW}操作已取消${NC}"
    exit 0
fi

echo ""
echo -e "${YELLOW}开始清理数据库...${NC}"

# 执行清理SQL
mysql -h $DB_HOST -P $DB_PORT -u $DB_USER -p$DB_PASS $DB_NAME << 'EOF' 2>/dev/null
-- 临时关闭外键检查
SET FOREIGN_KEY_CHECKS = 0;

-- 1. 清空执行记录相关表
TRUNCATE TABLE test_report_executions;
TRUNCATE TABLE test_reports;
TRUNCATE TABLE test_executions;
TRUNCATE TABLE screenshots;
TRUNCATE TABLE performance_metrics;

-- 2. 清空测试用例和测试套件相关表
TRUNCATE TABLE test_suite_cases;
TRUNCATE TABLE test_suites;
TRUNCATE TABLE test_cases;

-- 3. 清空项目表
TRUNCATE TABLE projects;

-- 4. 保留admin用户，删除其他用户
DELETE FROM users WHERE username != 'admin';

-- 5. 保留环境表ID=2的数据，删除其他
DELETE FROM environments WHERE id != 2;

-- 6. 设备表保留所有数据（不删除）

-- 重新启用外键检查
SET FOREIGN_KEY_CHECKS = 1;
EOF

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ 数据清理完成！${NC}"
    echo ""
    
    # 显示清理后的统计信息
    echo -e "${YELLOW}清理后的数据统计：${NC}"
    mysql -h $DB_HOST -P $DB_PORT -u $DB_USER -p$DB_PASS $DB_NAME -t << 'EOF' 2>/dev/null
SELECT 
    'Users' as '数据表', 
    COUNT(*) as '记录数' 
FROM users
UNION ALL
SELECT 'Environments', COUNT(*) FROM environments
UNION ALL
SELECT 'Devices', COUNT(*) FROM devices
UNION ALL
SELECT 'Projects', COUNT(*) FROM projects
UNION ALL
SELECT 'Test Cases', COUNT(*) FROM test_cases
UNION ALL
SELECT 'Test Suites', COUNT(*) FROM test_suites
UNION ALL
SELECT 'Test Executions', COUNT(*) FROM test_executions
UNION ALL
SELECT 'Test Reports', COUNT(*) FROM test_reports;
EOF
    
    echo ""
    echo -e "${GREEN}✅ 数据库已恢复到纯净模式！${NC}"
    echo ""
    echo "保留的数据："
    echo "  - Admin用户 (username: admin)"
    echo "  - 环境配置 (ID: 2, 安全宽带)"
    echo "  - 所有设备配置 (4个设备)"
else
    echo -e "${RED}❌ 数据清理失败！${NC}"
    echo "请检查数据库连接配置是否正确"
    exit 1
fi
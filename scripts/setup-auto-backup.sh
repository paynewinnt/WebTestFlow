#!/bin/bash

# 设置WebTestFlow数据库自动备份的脚本

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKUP_SCRIPT="$SCRIPT_DIR/backup-database.sh"

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

print_message() {
    echo -e "${2}${1}${NC}"
}

# 检查备份脚本是否存在
if [ ! -f "$BACKUP_SCRIPT" ]; then
    print_message "错误: 备份脚本不存在: $BACKUP_SCRIPT" "$RED"
    exit 1
fi

print_message "========================================" "$GREEN"
print_message "WebTestFlow 数据库自动备份设置" "$GREEN"
print_message "========================================" "$GREEN"

# 显示当前的cron任务
print_message "\n当前的cron任务:" "$YELLOW"
crontab -l 2>/dev/null | grep -v "^#" | grep backup-database.sh || echo "没有设置自动备份"

# 询问用户选择
echo ""
echo "请选择备份频率:"
echo "1) 每天凌晨2点备份"
echo "2) 每周一凌晨3点备份"
echo "3) 每月1日凌晨4点备份"
echo "4) 自定义cron表达式"
echo "5) 删除自动备份"
echo "0) 退出"

read -p "请输入选项 (0-5): " CHOICE

case $CHOICE in
    1)
        CRON_EXPR="0 2 * * *"
        CRON_DESC="每天凌晨2点"
        ;;
    2)
        CRON_EXPR="0 3 * * 1"
        CRON_DESC="每周一凌晨3点"
        ;;
    3)
        CRON_EXPR="0 4 1 * *"
        CRON_DESC="每月1日凌晨4点"
        ;;
    4)
        echo ""
        echo "Cron表达式格式: 分 时 日 月 星期"
        echo "例如: 0 2 * * * (每天凌晨2点)"
        read -p "请输入cron表达式: " CRON_EXPR
        CRON_DESC="自定义"
        ;;
    5)
        # 删除自动备份
        print_message "\n正在删除自动备份任务..." "$YELLOW"
        (crontab -l 2>/dev/null | grep -v "$BACKUP_SCRIPT") | crontab -
        print_message "✓ 自动备份任务已删除" "$GREEN"
        exit 0
        ;;
    0)
        print_message "退出设置" "$YELLOW"
        exit 0
        ;;
    *)
        print_message "无效的选项" "$RED"
        exit 1
        ;;
esac

# 添加cron任务
print_message "\n正在设置自动备份任务..." "$YELLOW"
print_message "备份频率: $CRON_DESC" "$YELLOW"

# 创建cron任务
CRON_JOB="$CRON_EXPR $BACKUP_SCRIPT >> $SCRIPT_DIR/../logs/backup.log 2>&1"

# 获取现有的cron任务（排除已存在的备份任务）
(crontab -l 2>/dev/null | grep -v "$BACKUP_SCRIPT"; echo "$CRON_JOB") | crontab -

if [ $? -eq 0 ]; then
    print_message "✓ 自动备份任务设置成功！" "$GREEN"
    
    # 创建日志目录
    LOG_DIR="$SCRIPT_DIR/../logs"
    if [ ! -d "$LOG_DIR" ]; then
        mkdir -p "$LOG_DIR"
        print_message "✓ 创建日志目录: $LOG_DIR" "$GREEN"
    fi
    
    print_message "\n当前的cron任务:" "$YELLOW"
    crontab -l | grep "$BACKUP_SCRIPT"
    
    print_message "\n备份日志将保存到: $LOG_DIR/backup.log" "$YELLOW"
    print_message "\n提示: 使用 'crontab -l' 查看所有定时任务" "$YELLOW"
    print_message "提示: 使用 'tail -f $LOG_DIR/backup.log' 查看备份日志" "$YELLOW"
else
    print_message "✗ 设置自动备份失败" "$RED"
    exit 1
fi
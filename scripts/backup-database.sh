#!/bin/bash

# WebTestFlow Database Backup Script
# 用于备份WebTestFlow MySQL数据库

# 配置部分
DB_HOST="localhost"
DB_PORT="3306"
DB_NAME="webtestflow"
DB_USER="root"
DB_PASSWORD="root"  # 建议使用环境变量或配置文件存储密码

# 备份配置
BACKUP_DIR="/home/ryan.rr.penn/Documents/Dev/GoProject/src/WebTestFlow/backups"
DATE=$(date +"%Y%m%d_%H%M%S")
BACKUP_FILE="webtestflow_backup_${DATE}.sql"
BACKUP_PATH="${BACKUP_DIR}/${BACKUP_FILE}"

# 保留备份天数（默认保留7天）
RETENTION_DAYS=7

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_message() {
    echo -e "${2}${1}${NC}"
}

# 检查命令是否存在
check_command() {
    if ! command -v $1 &> /dev/null; then
        print_message "错误: $1 命令未找到，请先安装MySQL客户端" "$RED"
        exit 1
    fi
}

# 创建备份目录
create_backup_dir() {
    if [ ! -d "$BACKUP_DIR" ]; then
        print_message "创建备份目录: $BACKUP_DIR" "$YELLOW"
        mkdir -p "$BACKUP_DIR"
        if [ $? -ne 0 ]; then
            print_message "错误: 无法创建备份目录" "$RED"
            exit 1
        fi
    fi
}

# 执行数据库备份
perform_backup() {
    print_message "========================================" "$GREEN"
    print_message "WebTestFlow 数据库备份开始" "$GREEN"
    print_message "========================================" "$GREEN"
    
    print_message "备份时间: $(date '+%Y-%m-%d %H:%M:%S')" "$YELLOW"
    print_message "数据库: $DB_NAME" "$YELLOW"
    print_message "备份文件: $BACKUP_PATH" "$YELLOW"
    
    # 使用mysqldump备份数据库
    # --single-transaction: 确保备份的一致性
    # --routines: 包含存储过程和函数
    # --triggers: 包含触发器
    # --events: 包含事件
    mysqldump -h "$DB_HOST" \
              -P "$DB_PORT" \
              -u "$DB_USER" \
              -p"$DB_PASSWORD" \
              --single-transaction \
              --routines \
              --triggers \
              --events \
              --databases "$DB_NAME" > "$BACKUP_PATH" 2>/dev/null
    
    if [ $? -eq 0 ]; then
        # 获取文件大小
        FILE_SIZE=$(du -h "$BACKUP_PATH" | cut -f1)
        print_message "✓ 备份成功完成！文件大小: $FILE_SIZE" "$GREEN"
        
        # 压缩备份文件
        print_message "正在压缩备份文件..." "$YELLOW"
        gzip "$BACKUP_PATH"
        if [ $? -eq 0 ]; then
            COMPRESSED_SIZE=$(du -h "${BACKUP_PATH}.gz" | cut -f1)
            print_message "✓ 压缩完成！压缩后大小: $COMPRESSED_SIZE" "$GREEN"
            BACKUP_PATH="${BACKUP_PATH}.gz"
        else
            print_message "警告: 压缩失败，保留原始SQL文件" "$YELLOW"
        fi
    else
        print_message "✗ 备份失败！" "$RED"
        print_message "请检查数据库连接参数和权限" "$RED"
        exit 1
    fi
}

# 清理旧备份
cleanup_old_backups() {
    print_message "清理超过 ${RETENTION_DAYS} 天的旧备份..." "$YELLOW"
    
    # 查找并删除旧备份文件
    find "$BACKUP_DIR" -name "webtestflow_backup_*.sql*" -type f -mtime +$RETENTION_DAYS -exec rm {} \; 2>/dev/null
    
    if [ $? -eq 0 ]; then
        print_message "✓ 旧备份清理完成" "$GREEN"
    else
        print_message "警告: 清理旧备份时出现问题" "$YELLOW"
    fi
    
    # 显示当前备份列表
    print_message "\n当前备份文件列表:" "$YELLOW"
    ls -lh "$BACKUP_DIR"/webtestflow_backup_*.sql* 2>/dev/null | tail -5
}

# 验证备份文件
verify_backup() {
    if [ -f "$BACKUP_PATH" ]; then
        print_message "\n备份验证:" "$YELLOW"
        
        # 检查文件是否为空
        if [ -s "$BACKUP_PATH" ]; then
            if [[ "$BACKUP_PATH" == *.gz ]]; then
                # 对于压缩文件，检查是否能正常解压
                gunzip -t "$BACKUP_PATH" 2>/dev/null
                if [ $? -eq 0 ]; then
                    print_message "✓ 备份文件验证通过" "$GREEN"
                else
                    print_message "✗ 备份文件可能已损坏" "$RED"
                    exit 1
                fi
            else
                # 对于SQL文件，检查是否包含关键内容
                if grep -q "CREATE DATABASE" "$BACKUP_PATH" 2>/dev/null; then
                    print_message "✓ 备份文件验证通过" "$GREEN"
                else
                    print_message "警告: 备份文件可能不完整" "$YELLOW"
                fi
            fi
        else
            print_message "✗ 备份文件为空" "$RED"
            exit 1
        fi
    else
        print_message "✗ 备份文件不存在" "$RED"
        exit 1
    fi
}

# 显示使用帮助
show_help() {
    echo "使用方法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -h, --help     显示帮助信息"
    echo "  -r, --restore  恢复备份（需要指定备份文件）"
    echo "  -l, --list     列出所有备份文件"
    echo ""
    echo "示例:"
    echo "  $0              执行数据库备份"
    echo "  $0 --list       列出所有备份"
    echo "  $0 --restore backup_file.sql.gz  恢复指定备份"
}

# 列出备份文件
list_backups() {
    print_message "========================================" "$GREEN"
    print_message "WebTestFlow 数据库备份文件列表" "$GREEN"
    print_message "========================================" "$GREEN"
    
    if [ -d "$BACKUP_DIR" ]; then
        echo ""
        ls -lh "$BACKUP_DIR"/webtestflow_backup_*.sql* 2>/dev/null
        echo ""
        
        # 统计备份数量和总大小
        BACKUP_COUNT=$(ls "$BACKUP_DIR"/webtestflow_backup_*.sql* 2>/dev/null | wc -l)
        if [ $BACKUP_COUNT -gt 0 ]; then
            TOTAL_SIZE=$(du -sh "$BACKUP_DIR" | cut -f1)
            print_message "总计: $BACKUP_COUNT 个备份文件，占用空间: $TOTAL_SIZE" "$YELLOW"
        else
            print_message "没有找到备份文件" "$YELLOW"
        fi
    else
        print_message "备份目录不存在" "$RED"
    fi
}

# 恢复备份
restore_backup() {
    local RESTORE_FILE=$1
    
    if [ -z "$RESTORE_FILE" ]; then
        print_message "错误: 请指定要恢复的备份文件" "$RED"
        echo "用法: $0 --restore <backup_file>"
        exit 1
    fi
    
    # 检查备份文件是否存在
    if [ ! -f "$RESTORE_FILE" ]; then
        # 尝试在备份目录中查找
        if [ -f "$BACKUP_DIR/$RESTORE_FILE" ]; then
            RESTORE_FILE="$BACKUP_DIR/$RESTORE_FILE"
        else
            print_message "错误: 备份文件不存在: $RESTORE_FILE" "$RED"
            exit 1
        fi
    fi
    
    print_message "========================================" "$GREEN"
    print_message "WebTestFlow 数据库恢复" "$GREEN"
    print_message "========================================" "$GREEN"
    
    print_message "警告: 恢复操作将覆盖现有数据库！" "$RED"
    read -p "确定要恢复吗？(yes/no): " CONFIRM
    
    if [ "$CONFIRM" != "yes" ]; then
        print_message "恢复操作已取消" "$YELLOW"
        exit 0
    fi
    
    # 先备份当前数据库
    print_message "正在备份当前数据库..." "$YELLOW"
    BACKUP_BEFORE_RESTORE="webtestflow_before_restore_${DATE}.sql"
    mysqldump -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASSWORD" \
              --single-transaction "$DB_NAME" > "$BACKUP_DIR/$BACKUP_BEFORE_RESTORE" 2>/dev/null
    
    if [ $? -eq 0 ]; then
        print_message "✓ 当前数据库已备份到: $BACKUP_DIR/$BACKUP_BEFORE_RESTORE" "$GREEN"
    fi
    
    # 执行恢复
    print_message "正在恢复数据库..." "$YELLOW"
    
    if [[ "$RESTORE_FILE" == *.gz ]]; then
        # 解压并恢复
        gunzip -c "$RESTORE_FILE" | mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASSWORD" 2>/dev/null
    else
        # 直接恢复
        mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASSWORD" < "$RESTORE_FILE" 2>/dev/null
    fi
    
    if [ $? -eq 0 ]; then
        print_message "✓ 数据库恢复成功！" "$GREEN"
    else
        print_message "✗ 数据库恢复失败！" "$RED"
        print_message "尝试从备份恢复: $BACKUP_DIR/$BACKUP_BEFORE_RESTORE" "$YELLOW"
        exit 1
    fi
}

# 主程序
main() {
    # 处理命令行参数
    case "$1" in
        -h|--help)
            show_help
            exit 0
            ;;
        -l|--list)
            list_backups
            exit 0
            ;;
        -r|--restore)
            check_command "mysql"
            restore_backup "$2"
            exit 0
            ;;
        "")
            # 执行备份
            check_command "mysqldump"
            create_backup_dir
            perform_backup
            verify_backup
            cleanup_old_backups
            
            print_message "\n========================================" "$GREEN"
            print_message "备份完成！" "$GREEN"
            print_message "备份文件: $BACKUP_PATH" "$GREEN"
            print_message "========================================" "$GREEN"
            ;;
        *)
            print_message "错误: 未知选项 '$1'" "$RED"
            show_help
            exit 1
            ;;
    esac
}

# 执行主程序
main "$@"
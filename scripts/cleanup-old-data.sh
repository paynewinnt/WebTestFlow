#!/bin/bash
# WebTestFlow 数据清理脚本
# 自动清理7天前的Chrome缓存、用户数据和截图文件

set -e

# 获取脚本所在目录的父目录（项目根目录）
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LOG_FILE="$PROJECT_ROOT/logs/cleanup.log"

# 创建日志目录
mkdir -p "$(dirname "$LOG_FILE")"

# 日志函数
log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') $1" | tee -a "$LOG_FILE"
}

log "========================================="
log "开始执行WebTestFlow数据清理任务"
log "项目根目录: $PROJECT_ROOT"

# 清理计数器
CLEANED_CHROME_CACHE=0
CLEANED_USER_DATA=0
CLEANED_SCREENSHOTS=0
CLEANED_RUNTIME=0

# 1. 清理Chrome缓存目录 (tmp/chrome-cache-*)
log "正在清理Chrome缓存目录..."
if [ -d "$PROJECT_ROOT/tmp" ]; then
    # 查找7天前的Chrome缓存目录
    find "$PROJECT_ROOT/tmp" -type d -name "chrome-cache-*" -mtime +7 2>/dev/null | while read -r cache_dir; do
        if [ -d "$cache_dir" ]; then
            log "删除Chrome缓存目录: $cache_dir"
            rm -rf "$cache_dir"
            CLEANED_CHROME_CACHE=$((CLEANED_CHROME_CACHE + 1))
        fi
    done
    
    # 查找7天前的Chrome用户数据目录 (chrome-data-*, chrome-test*)
    find "$PROJECT_ROOT/tmp" -type d \( -name "chrome-data-*" -o -name "chrome-test*" \) -mtime +7 2>/dev/null | while read -r data_dir; do
        if [ -d "$data_dir" ]; then
            log "删除Chrome用户数据目录: $data_dir"
            rm -rf "$data_dir"
            CLEANED_USER_DATA=$((CLEANED_USER_DATA + 1))
        fi
    done
    
    # 清理flatpak运行时目录
    find "$PROJECT_ROOT/tmp" -type d -name "flatpak-runtime-*" -mtime +7 2>/dev/null | while read -r runtime_dir; do
        if [ -d "$runtime_dir" ]; then
            log "删除Flatpak运行时目录: $runtime_dir"
            rm -rf "$runtime_dir"
            CLEANED_RUNTIME=$((CLEANED_RUNTIME + 1))
        fi
    done
fi

# 2. 清理系统临时目录中的Chrome数据
log "正在清理系统临时目录中的Chrome数据..."
# 清理/tmp中的WebTestFlow相关目录
find /tmp -maxdepth 1 -type d \( -name "chrome-data-*" -o -name "webtestflow-*" -o -name "chrome-cache-*" \) -mtime +7 2>/dev/null | while read -r temp_dir; do
    if [ -d "$temp_dir" ]; then
        log "删除临时Chrome数据目录: $temp_dir"
        rm -rf "$temp_dir"
        CLEANED_USER_DATA=$((CLEANED_USER_DATA + 1))
    fi
done

# 3. 清理截图数据
log "正在清理截图数据..."
if [ -d "$PROJECT_ROOT/screenshots" ]; then
    # 清理7天前的截图文件
    find "$PROJECT_ROOT/screenshots" -type f \( -name "*.png" -o -name "*.jpg" -o -name "*.jpeg" \) -mtime +7 2>/dev/null | while read -r screenshot_file; do
        if [ -f "$screenshot_file" ]; then
            log "删除截图文件: $screenshot_file"
            rm -f "$screenshot_file"
            CLEANED_SCREENSHOTS=$((CLEANED_SCREENSHOTS + 1))
        fi
    done
    
    # 清理7天前的空目录
    find "$PROJECT_ROOT/screenshots" -type d -empty -mtime +7 2>/dev/null | while read -r empty_dir; do
        if [ -d "$empty_dir" ] && [ "$empty_dir" != "$PROJECT_ROOT/screenshots" ]; then
            log "删除空的截图目录: $empty_dir"
            rmdir "$empty_dir" 2>/dev/null || true
        fi
    done
fi

# 4. 清理上传文件（如果有的话）
log "正在清理上传文件..."
if [ -d "$PROJECT_ROOT/uploads" ]; then
    find "$PROJECT_ROOT/uploads" -type f -mtime +7 2>/dev/null | while read -r upload_file; do
        if [ -f "$upload_file" ]; then
            log "删除上传文件: $upload_file"
            rm -f "$upload_file"
        fi
    done
fi

# 5. 清理日志文件（保留最近30天的日志）
log "正在清理旧日志文件..."
if [ -d "$PROJECT_ROOT/logs" ]; then
    find "$PROJECT_ROOT/logs" -name "*.log" -mtime +30 2>/dev/null | while read -r log_file; do
        if [ -f "$log_file" ] && [ "$log_file" != "$LOG_FILE" ]; then
            log "删除旧日志文件: $log_file"
            rm -f "$log_file"
        fi
    done
fi

# 显示清理统计
log "清理完成统计:"
log "- Chrome缓存目录: 已处理"
log "- Chrome用户数据目录: 已处理" 
log "- 截图文件: 已处理"
log "- Flatpak运行时目录: 已处理"

# 显示当前磁盘使用情况
log "当前磁盘使用情况:"
if [ -d "$PROJECT_ROOT/tmp" ]; then
    TMP_SIZE=$(du -sh "$PROJECT_ROOT/tmp" 2>/dev/null | cut -f1 || echo "0")
    log "- tmp目录大小: $TMP_SIZE"
fi

if [ -d "$PROJECT_ROOT/screenshots" ]; then
    SCREENSHOTS_SIZE=$(du -sh "$PROJECT_ROOT/screenshots" 2>/dev/null | cut -f1 || echo "0")
    log "- 截图目录大小: $SCREENSHOTS_SIZE"
fi

if [ -d "$PROJECT_ROOT/logs" ]; then
    LOGS_SIZE=$(du -sh "$PROJECT_ROOT/logs" 2>/dev/null | cut -f1 || echo "0")
    log "- 日志目录大小: $LOGS_SIZE"
fi

log "WebTestFlow数据清理任务执行完成"
log "========================================="

# 如果有参数 --verbose，显示详细信息
if [ "$1" = "--verbose" ]; then
    echo "清理脚本执行完成，详细日志请查看: $LOG_FILE"
    echo "最近的清理记录:"
    tail -20 "$LOG_FILE"
fi
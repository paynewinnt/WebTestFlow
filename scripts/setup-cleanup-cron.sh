#!/bin/bash
# WebTestFlow 定时清理任务设置脚本

set -e

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CLEANUP_SCRIPT="$PROJECT_ROOT/scripts/cleanup-old-data.sh"
CRON_JOB="0 2 * * * $CLEANUP_SCRIPT > /dev/null 2>&1"

echo "WebTestFlow 定时清理任务设置"
echo "=============================="
echo "项目根目录: $PROJECT_ROOT"
echo "清理脚本路径: $CLEANUP_SCRIPT"
echo "定时任务: 每天凌晨2点执行清理"

# 检查清理脚本是否存在
if [ ! -f "$CLEANUP_SCRIPT" ]; then
    echo "错误: 清理脚本不存在: $CLEANUP_SCRIPT"
    exit 1
fi

# 检查脚本是否有执行权限
if [ ! -x "$CLEANUP_SCRIPT" ]; then
    echo "警告: 清理脚本没有执行权限，正在添加权限..."
    chmod +x "$CLEANUP_SCRIPT"
fi

# 检查当前用户的crontab中是否已经存在相同的任务
if crontab -l 2>/dev/null | grep -q "$CLEANUP_SCRIPT"; then
    echo "定时任务已存在，正在更新..."
    # 删除现有的任务
    (crontab -l 2>/dev/null | grep -v "$CLEANUP_SCRIPT") | crontab -
fi

# 添加新的定时任务
echo "正在添加定时清理任务..."
(crontab -l 2>/dev/null; echo "$CRON_JOB") | crontab -

echo "定时任务设置完成！"
echo ""
echo "当前的crontab配置:"
echo "-------------------"
crontab -l | grep "$CLEANUP_SCRIPT" || echo "未找到相关任务"

echo ""
echo "使用说明:"
echo "- 任务将在每天凌晨2:00自动执行"
echo "- 手动执行清理: $CLEANUP_SCRIPT"
echo "- 查看清理日志: tail -f $PROJECT_ROOT/logs/cleanup.log"
echo "- 移除定时任务: crontab -e 然后删除相关行"
echo ""
echo "测试清理脚本 (可选):"
echo "$CLEANUP_SCRIPT --verbose"
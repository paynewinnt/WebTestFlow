#!/bin/bash
# WebTestFlow 立即清理脚本 - 用于手动清理数据

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CLEANUP_SCRIPT="$PROJECT_ROOT/scripts/cleanup-old-data.sh"

echo "🧹 WebTestFlow 立即清理工具"
echo "============================"

# 检查清理脚本是否存在
if [ ! -f "$CLEANUP_SCRIPT" ]; then
    echo "❌ 错误: 清理脚本不存在: $CLEANUP_SCRIPT"
    exit 1
fi

# 显示清理前的状态
echo "📊 清理前状态:"
if [ -d "$PROJECT_ROOT/tmp" ]; then
    TMP_SIZE=$(du -sh "$PROJECT_ROOT/tmp" 2>/dev/null | cut -f1 || echo "0")
    echo "   - tmp目录: $TMP_SIZE"
fi

if [ -d "$PROJECT_ROOT/screenshots" ]; then
    SCREENSHOTS_SIZE=$(du -sh "$PROJECT_ROOT/screenshots" 2>/dev/null | cut -f1 || echo "0")
    echo "   - 截图目录: $SCREENSHOTS_SIZE"
fi

echo ""
echo "🚀 开始执行清理..."

# 执行清理脚本并显示详细输出
"$CLEANUP_SCRIPT" --verbose

echo ""
echo "📊 清理后状态:"
if [ -d "$PROJECT_ROOT/tmp" ]; then
    TMP_SIZE=$(du -sh "$PROJECT_ROOT/tmp" 2>/dev/null | cut -f1 || echo "0")
    echo "   - tmp目录: $TMP_SIZE"
fi

if [ -d "$PROJECT_ROOT/screenshots" ]; then
    SCREENSHOTS_SIZE=$(du -sh "$PROJECT_ROOT/screenshots" 2>/dev/null | cut -f1 || echo "0")
    echo "   - 截图目录: $SCREENSHOTS_SIZE"
fi

echo ""
echo "✅ 清理完成！"
echo "📋 查看完整日志: tail -f $PROJECT_ROOT/logs/cleanup.log"
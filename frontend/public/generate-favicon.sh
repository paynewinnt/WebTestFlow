#!/bin/bash

# 生成不同尺寸的PNG文件（如果系统安装了ImageMagick）
if command -v convert &> /dev/null; then
    echo "生成favicon.ico..."
    
    # 从SVG生成不同尺寸的PNG
    convert -background transparent favicon.svg -resize 16x16 favicon-16.png
    convert -background transparent favicon.svg -resize 32x32 favicon-32.png
    convert -background transparent favicon.svg -resize 48x48 favicon-48.png
    
    # 合并成ICO文件
    convert favicon-16.png favicon-32.png favicon-48.png favicon.ico
    
    # 清理临时文件
    rm favicon-16.png favicon-32.png favicon-48.png
    
    echo "favicon.ico 生成完成！"
else
    echo "需要安装ImageMagick来生成ICO文件"
    echo "安装命令："
    echo "  Ubuntu/Debian: sudo apt-get install imagemagick"
    echo "  macOS: brew install imagemagick"
    echo "  CentOS/RHEL: sudo yum install ImageMagick"
fi
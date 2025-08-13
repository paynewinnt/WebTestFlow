#!/bin/bash

# Chrome安装脚本 for Rocky Linux 9

set -e

echo "🌟 安装Google Chrome浏览器..."

# 检查是否有sudo权限
if ! sudo -n true 2>/dev/null; then
    echo "❌ 需要sudo权限来安装Chrome"
    echo "请运行: sudo $0"
    exit 1
fi

# 添加Google Chrome仓库
echo "📦 添加Google Chrome仓库..."
sudo tee /etc/yum.repos.d/google-chrome.repo > /dev/null <<EOF
[google-chrome]
name=google-chrome
baseurl=http://dl.google.com/linux/chrome/rpm/stable/x86_64
enabled=1
gpgcheck=1
gpgkey=https://dl.google.com/linux/linux_signing_key.pub
EOF

# 导入GPG密钥
echo "🔑 导入GPG密钥..."
sudo rpm --import https://dl.google.com/linux/linux_signing_key.pub

# 安装Chrome
echo "⬇️ 安装Google Chrome..."
sudo dnf install -y google-chrome-stable

# 验证安装
if which google-chrome-stable > /dev/null 2>&1; then
    echo "✅ Google Chrome安装成功！"
    google-chrome-stable --version
    
    # 创建软链接
    if ! which google-chrome > /dev/null 2>&1; then
        sudo ln -sf /usr/bin/google-chrome-stable /usr/bin/google-chrome
        echo "🔗 创建google-chrome软链接"
    fi
    
    echo ""
    echo "🎉 Chrome安装完成！"
    echo "📝 现在可以使用AutoUI Platform的录制功能了"
    echo ""
    echo "💡 提示："
    echo "   - Chrome将以headless模式运行(无界面)"
    echo "   - 录制时会自动打开Chrome进行操作"
    echo "   - 支持所有现代Web标准和JavaScript"
    
else
    echo "❌ Chrome安装失败"
    exit 1
fi
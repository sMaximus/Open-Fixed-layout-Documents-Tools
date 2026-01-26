#!/bin/bash
echo "========================================"
echo "  OFD Viewer JS 打包压缩脚本"
echo "========================================"
echo

# 检查 node_modules 是否存在
if [ ! -d "node_modules" ]; then
    echo "正在安装依赖..."
    npm install
    if [ $? -ne 0 ]; then
        echo "依赖安装失败!"
        exit 1
    fi
    echo
fi

echo "开始打包压缩 JS 文件..."
node build-js.js

if [ $? -ne 0 ]; then
    echo "打包失败!"
    exit 1
fi

echo
echo "打包完成! 输出目录: dist/"

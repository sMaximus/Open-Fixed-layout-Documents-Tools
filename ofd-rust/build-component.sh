#!/bin/bash
set -e

echo "=== 构建 OFD Viewer 组件 ==="

# 1. 构建 WASM
echo "[1/3] 构建 WASM..."
wasm-pack build --target web --out-dir target/wasm-pack-pkg
node scripts/sync-wasm-artifact.mjs

# 2. 安装 JS 依赖
echo "[2/3] 安装依赖..."
npm install

# 3. 打包 JS 组件
echo "[3/3] 打包 JS 组件..."
npx rollup -c rollup.config.mjs

echo ""
echo "=== 构建完成 ==="
echo "输出文件:"
ls -lh dist/ofd-viewer.*
echo ""
echo "使用方式见 dist/example.html"

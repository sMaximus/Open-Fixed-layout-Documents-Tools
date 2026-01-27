#!/bin/bash
# 构建 Rust WASM

# 安装 wasm-pack（如果未安装）
if ! command -v wasm-pack &> /dev/null; then
    echo "Installing wasm-pack..."
    cargo install wasm-pack
fi

# 构建 WASM
wasm-pack build --target web --out-dir pkg

echo "Build complete! Output in pkg/"

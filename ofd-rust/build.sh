#!/bin/bash
# 构建 Rust WASM

# 安装 wasm-pack（如果未安装）
if ! command -v wasm-pack &> /dev/null; then
    echo "Installing wasm-pack..."
    cargo install wasm-pack
fi

# 构建 WASM
wasm-pack build --target web --out-dir target/wasm-pack-pkg
node scripts/sync-wasm-artifact.mjs

echo "Build complete! Updated pkg/ofd_rust_bg.wasm without overwriting pkg/ofd_rust.js"

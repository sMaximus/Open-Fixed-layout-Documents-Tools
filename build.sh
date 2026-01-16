#!/bin/bash
echo "编译 OFD WASM..."
GOOS=js GOARCH=wasm go build -o web/ofd.wasm main.go

echo "复制 wasm_exec.js..."
cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" web/wasm_exec.js

echo "构建完成!"
echo "请使用 HTTP 服务器打开 web/index.html"
echo "例如: cd web && python -m http.server 8080"

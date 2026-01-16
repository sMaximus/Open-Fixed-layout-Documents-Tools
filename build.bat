@echo off
echo 编译 OFD WASM...

if not exist "web" mkdir web

set GOOS=js
set GOARCH=wasm
go build -o web\ofd.wasm main.go

if %ERRORLEVEL% NEQ 0 (
    echo 编译失败!
    exit /b 1
)

echo 复制 wasm_exec.js...
for /f "delims=" %%i in ('go env GOROOT') do set GOROOT=%%i
copy "%GOROOT%\misc\wasm\wasm_exec.js" web\wasm_exec.js

echo.
echo 构建完成!
echo 请运行: cd web ^&^& python -m http.server 8080

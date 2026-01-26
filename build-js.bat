@echo off
echo ========================================
echo   OFD Viewer JS 打包压缩脚本
echo ========================================
echo.

REM 检查 node_modules 是否存在
if not exist "node_modules" (
    echo 正在安装依赖...
    npm install
    if %ERRORLEVEL% NEQ 0 (
        echo 依赖安装失败!
        exit /b 1
    )
    echo.
)

echo 开始打包压缩 JS 文件...
node build-js.js

if %ERRORLEVEL% NEQ 0 (
    echo 打包失败!
    exit /b 1
)

echo.
echo 打包完成! 输出目录: dist\

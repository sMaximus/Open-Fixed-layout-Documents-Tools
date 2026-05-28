@echo off
echo === 构建 OFD Viewer 组件 ===

echo [1/3] 构建 WASM...
wasm-pack build --target web --out-dir target/wasm-pack-pkg
if errorlevel 1 goto :error
node scripts\sync-wasm-artifact.mjs
if errorlevel 1 goto :error

echo [2/3] 安装依赖...
call npm install
if errorlevel 1 goto :error

echo [3/3] 打包 JS 组件...
call npx rollup -c rollup.config.mjs
if errorlevel 1 goto :error

echo.
echo === 构建完成 ===
dir /b dist\ofd-viewer.*
echo.
echo 使用方式见 dist\example.html
goto :eof

:error
echo 构建失败!
exit /b 1

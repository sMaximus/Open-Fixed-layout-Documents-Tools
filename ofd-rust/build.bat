@echo off
echo Building Rust WASM...

wasm-pack build --target web --out-dir pkg

if %ERRORLEVEL% equ 0 (
    echo Build complete! Output in pkg/
) else (
    echo Build failed!
    exit /b 1
)

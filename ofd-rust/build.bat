@echo off
setlocal
echo Building Rust WASM...

wasm-pack build --target web --out-dir target/wasm-pack-pkg

if not %ERRORLEVEL% equ 0 (
    echo Build failed!
    exit /b 1
)

node scripts\sync-wasm-artifact.mjs

if not %ERRORLEVEL% equ 0 (
    echo Failed to sync WASM artifact!
    exit /b 1
)

echo Build complete! Updated pkg/ofd_rust_bg.wasm without overwriting pkg/ofd_rust.js

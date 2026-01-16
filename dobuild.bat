@echo off
set GOOS=js
set GOARCH=wasm
go build -o web/ofd.wasm .
echo Done

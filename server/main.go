package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	port := flag.String("port", "8080", "服务端口")
	flag.Parse()

	// 获取当前目录
	dir, _ := os.Getwd()
	webDir := filepath.Join(dir, "web")

	// 检查 web 目录是否存在
	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		// 尝试上级目录
		webDir = filepath.Join(filepath.Dir(dir), "web")
	}

	fmt.Printf("服务目录: %s\n", webDir)
	fmt.Printf("服务器启动: http://localhost:%s\n", *port)
	fmt.Println("按 Ctrl+C 停止服务器")

	// 设置 WASM MIME 类型
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		filePath := filepath.Join(webDir, path)

		// 设置正确的 Content-Type
		if filepath.Ext(path) == ".wasm" {
			w.Header().Set("Content-Type", "application/wasm")
		}

		http.ServeFile(w, r, filePath)
	})

	log.Fatal(http.ListenAndServe(":"+*port, nil))
}

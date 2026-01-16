//go:build js && wasm

package main

import (
	"encoding/json"
	"ofd-wasm/ofd"
	"syscall/js"
)

var parser *ofd.Parser

// parseOFD 解析OFD文件
func parseOFD(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return createErrorResult("需要提供文件数据")
	}

	// 从 Uint8Array 获取数据
	uint8Array := args[0]
	length := uint8Array.Get("length").Int()
	data := make([]byte, length)
	js.CopyBytesToGo(data, uint8Array)

	// 创建解析器
	var err error
	parser, err = ofd.NewParser(data)
	if err != nil {
		return createErrorResult("创建解析器失败: " + err.Error())
	}

	// 解析文件
	result, err := parser.Parse()
	if err != nil {
		return createErrorResult("解析失败: " + err.Error())
	}

	// 转换为JSON
	jsonData, err := json.Marshal(result)
	if err != nil {
		return createErrorResult("JSON序列化失败: " + err.Error())
	}

	return string(jsonData)
}

// getDocInfo 获取文档信息
func getDocInfo(this js.Value, args []js.Value) interface{} {
	if parser == nil {
		return createErrorResult("请先解析OFD文件")
	}

	info := parser.GetDocInfo()
	if info == nil {
		return createErrorResult("无法获取文档信息")
	}

	jsonData, _ := json.Marshal(info)
	return string(jsonData)
}


// getFiles 获取文件列表
func getFiles(this js.Value, args []js.Value) interface{} {
	if parser == nil {
		return createErrorResult("请先解析OFD文件")
	}

	files := parser.GetFiles()
	jsonData, _ := json.Marshal(files)
	return string(jsonData)
}

// getPageCount 获取页数
func getPageCount(this js.Value, args []js.Value) interface{} {
	if parser == nil {
		return 0
	}
	return parser.GetPageCount()
}

// getFileContent 获取文件内容
func getFileContent(this js.Value, args []js.Value) interface{} {
	if parser == nil {
		return createErrorResult("请先解析OFD文件")
	}

	if len(args) < 1 {
		return createErrorResult("需要提供文件名")
	}

	fileName := args[0].String()
	content, err := parser.GetFileContent(fileName)
	if err != nil {
		return createErrorResult(err.Error())
	}

	// 返回 Uint8Array
	uint8Array := js.Global().Get("Uint8Array").New(len(content))
	js.CopyBytesToJS(uint8Array, content)
	return uint8Array
}

// getPagePath 获取页面路径
func getPagePath(this js.Value, args []js.Value) interface{} {
	if parser == nil {
		return ""
	}

	if len(args) < 1 {
		return ""
	}

	index := args[0].Int()
	return parser.GetPagePath(index)
}

// renderPage 渲染页面为HTML
func renderPage(this js.Value, args []js.Value) interface{} {
	if parser == nil {
		return createErrorResult("请先解析OFD文件")
	}

	pageIndex := 0
	if len(args) > 0 {
		pageIndex = args[0].Int()
	}

	result := parser.RenderPage(pageIndex)
	jsonData, _ := json.Marshal(result)
	return string(jsonData)
}

// renderAllPages 渲染所有页面
func renderAllPages(this js.Value, args []js.Value) interface{} {
	if parser == nil {
		return createErrorResult("请先解析OFD文件")
	}

	pageCount := parser.GetPageCount()
	results := make([]*ofd.PageRenderResult, pageCount)

	for i := 0; i < pageCount; i++ {
		results[i] = parser.RenderPage(i)
	}

	jsonData, _ := json.Marshal(results)
	return string(jsonData)
}

// getDebugInfo 获取调试信息
func getDebugInfo(this js.Value, args []js.Value) interface{} {
	if parser == nil {
		return createErrorResult("请先解析OFD文件")
	}

	info := parser.GetDebugInfo()
	jsonData, _ := json.Marshal(info)
	return string(jsonData)
}

// getFonts 获取字体信息
func getFonts(this js.Value, args []js.Value) interface{} {
	if parser == nil {
		return createErrorResult("请先解析OFD文件")
	}

	fonts := parser.GetFonts()
	jsonData, _ := json.Marshal(fonts)
	return string(jsonData)
}

// dumpAllFiles 导出所有文件内容（用于调试）
func dumpAllFiles(this js.Value, args []js.Value) interface{} {
	if parser == nil {
		return createErrorResult("请先解析OFD文件")
	}

	files := parser.GetFiles()
	result := make(map[string]interface{})

	for _, fileName := range files {
		content, err := parser.GetFileContent(fileName)
		if err != nil {
			result[fileName] = map[string]string{"error": err.Error()}
			continue
		}

		// 判断是否是文本文件（XML等）
		isText := false
		lowerName := fileName
		for _, ext := range []string{".xml", ".xsd", ".txt", ".json"} {
			if len(lowerName) > len(ext) && lowerName[len(lowerName)-len(ext):] == ext {
				isText = true
				break
			}
		}

		if isText {
			result[fileName] = map[string]interface{}{
				"type":    "text",
				"size":    len(content),
				"content": string(content),
			}
		} else {
			result[fileName] = map[string]interface{}{
				"type": "binary",
				"size": len(content),
			}
		}
	}

	jsonData, _ := json.Marshal(result)
	return string(jsonData)
}

func createErrorResult(msg string) string {
	result := map[string]string{"error": msg}
	jsonData, _ := json.Marshal(result)
	return string(jsonData)
}

func main() {
	c := make(chan struct{}, 0)

	// 注册全局函数
	js.Global().Set("ofdParseFile", js.FuncOf(parseOFD))
	js.Global().Set("ofdGetDocInfo", js.FuncOf(getDocInfo))
	js.Global().Set("ofdGetFiles", js.FuncOf(getFiles))
	js.Global().Set("ofdGetPageCount", js.FuncOf(getPageCount))
	js.Global().Set("ofdGetFileContent", js.FuncOf(getFileContent))
	js.Global().Set("ofdGetPagePath", js.FuncOf(getPagePath))
	js.Global().Set("ofdRenderPage", js.FuncOf(renderPage))
	js.Global().Set("ofdRenderAllPages", js.FuncOf(renderAllPages))
	js.Global().Set("ofdGetDebugInfo", js.FuncOf(getDebugInfo))
	js.Global().Set("ofdGetFonts", js.FuncOf(getFonts))
	js.Global().Set("ofdDumpAllFiles", js.FuncOf(dumpAllFiles))

	println("OFD WASM 解析器已加载")
	<-c
}

# OFD WebAssembly 解析器

使用 Go 编写的 OFD 文件解析器，编译为 WebAssembly 在浏览器中运行。

## 项目结构

```
ofd-wasm/
├── go.mod              # Go 模块定义
├── main.go             # WASM 入口，导出 JS 函数
├── dobuild.bat         # Windows 构建脚本
├── README.md           # 项目说明
├── ofd/
│   ├── types.go        # OFD 基础类型定义
│   ├── parser.go       # OFD 文件解析逻辑
│   ├── page.go         # 页面结构定义
│   └── render.go       # 页面渲染逻辑
├── server/
│   └── main.go         # HTTP 服务器
└── web/
    ├── index.html      # 演示页面
    ├── app.js          # 前端交互逻辑
    ├── wasm_exec.js    # Go WASM 运行时
    └── ofd.wasm        # 编译后的 WASM 文件
```

## 功能特性

- ✅ OFD 文件解析（ZIP 格式）
- ✅ 文档信息提取（标题、作者、日期等）
- ✅ 页面列表和导航
- ✅ Canvas 渲染（路径、图形）
- ✅ 文本层渲染（可选择、复制）
- ⚠️ 图片渲染（部分支持）
- ⚠️ 字体和排版（需要优化）

## 构建和运行

### 1. 编译 WASM

```bash
# Windows
dobuild.bat

# Linux/Mac
GOOS=js GOARCH=wasm go build -o web/ofd.wasm .
```

### 2. 启动服务器

```bash
go run server/main.go
```

### 3. 访问

打开浏览器访问 http://localhost:8080

## API 说明

WASM 加载后，以下函数可在全局使用：

| 函数                       | 参数     | 返回值      | 说明          |
| -------------------------- | -------- | ----------- | ------------- |
| `ofdParseFile(Uint8Array)` | 文件数据 | JSON 字符串 | 解析 OFD 文件 |
| `ofdGetDocInfo()`          | -        | JSON 字符串 | 获取文档信息  |
| `ofdGetFiles()`            | -        | JSON 字符串 | 获取文件列表  |
| `ofdGetPageCount()`        | -        | 数字        | 获取页数      |
| `ofdGetFileContent(name)`  | 文件名   | Uint8Array  | 获取文件内容  |
| `ofdGetPagePath(index)`    | 页面索引 | 字符串      | 获取页面路径  |
| `ofdRenderPage(index)`     | 页面索引 | JSON 字符串 | 渲染单个页面  |
| `ofdRenderAllPages()`      | -        | JSON 字符串 | 渲染所有页面  |

## 渲染架构

采用 **Canvas + HTML Text Layer** 双层渲染：

1. **Canvas 层**：渲染图形、路径、图片（背景）
2. **Text 层**：HTML span 元素，文字可选择复制

### 数据流

```
OFD 文件 (ZIP)
    ↓
Go 解析器 (parser.go)
    ↓
页面数据 (Page struct)
    ↓
渲染器 (render.go)
    ↓
JSON 数据 { canvasData, textLayer }
    ↓
JavaScript (app.js)
    ↓
Canvas 绘制 + DOM 渲染
```

## 已知问题

1. **图片渲染不完整**

   - 印章等圆形图片未显示
   - 需要检查 ResourceID 匹配和图片路径解析

2. **文字排版偏差**

   - 字体映射不准确
   - 字间距计算需要优化
   - 需要支持 DeltaX/DeltaY 属性

3. **坐标系统**

   - OFD 使用 mm 单位，需要精确转换
   - 路径坐标可能需要相对/绝对转换

4. **字体支持**
   - 缺少字体文件加载
   - 需要字体替换机制

## 优化方向

### 短期优化

1. 修复图片资源 ID 匹配逻辑
2. 实现 TextCode 的 DeltaX 字间距
3. 优化字体大小和位置计算

### 中期优化

1. 支持更多路径命令（Arc、椭圆等）
2. 实现 CGTransform 字形变换
3. 添加颜色空间转换

### 长期优化

1. 字体文件嵌入和加载
2. 完整的 OFD 规范支持
3. 性能优化（大文件、多页面）
4. 打印和导出功能

## 技术栈

- **后端**: Go 1.21+
- **前端**: Vanilla JavaScript
- **渲染**: Canvas API + DOM
- **格式**: OFD (Open Fixed-layout Document)

## 参考资料

- [OFD 标准规范](http://www.ofdspec.org/)
- [Go WebAssembly](https://github.com/golang/go/wiki/WebAssembly)
- [Canvas API](https://developer.mozilla.org/en-US/docs/Web/API/Canvas_API)

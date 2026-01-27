# OFD Rust WASM

用 Rust 实现的 OFD (Open Fixed-layout Document) 解析器，编译为 WebAssembly。

## 功能

- 解析 OFD 文档结构
- 提取页面内容（文本、路径、图片）
- 支持渐变填充
- 支持 CTM 变换矩阵
- 字体信息提取

## 构建

需要安装 Rust 和 wasm-pack：

```bash
# 安装 wasm-pack
cargo install wasm-pack

# 构建
wasm-pack build --target web --out-dir pkg

# 或使用脚本
./build.sh  # Linux/Mac
build.bat   # Windows
```

## 使用

```javascript
import init, { OFDParser } from "./pkg/ofd_rust.js";

await init();

// 加载 OFD 文件
const data = new Uint8Array(arrayBuffer);
const parser = new OFDParser(data);

// 解析
const result = parser.parse();
console.log("页数:", result.pageCount);

// 渲染页面
const pageResult = parser.render_page(0);
console.log("宽度:", pageResult.width);
console.log("高度:", pageResult.height);

// 获取字体
const fonts = parser.get_fonts();
```

## API

### OFDParser

- `new OFDParser(data: Uint8Array)` - 创建解析器
- `parse()` - 解析文档，返回 `{ files, pageCount, error }`
- `get_page_count()` - 获取页数
- `get_page_size(index)` - 获取页面尺寸
- `render_page(index)` - 渲染页面
- `get_fonts()` - 获取字体信息
- `get_files()` - 获取文件列表
- `get_doc_info()` - 获取文档信息

## 许可证

MIT

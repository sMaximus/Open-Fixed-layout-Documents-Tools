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

### 合并版入口（单 JS + 单 wasm）

```javascript
import { OFDViewer } from "ofd-viewer/bundle";

const viewer = new OFDViewer({
  container: document.getElementById("viewer"),
});

await viewer.init();
await viewer.loadFile(file);
```

打包产物位于 `dist/`：

- `dist/ofd-viewer.bundle.esm.js`
- `dist/ofd-viewer.bundle.umd.js`
- `dist/ofd-viewer.wasm`

浏览器直接使用 UMD 时，只需要同目录的一个 JS 和一个 wasm：

```html
<script src="./dist/ofd-viewer.bundle.umd.js"></script>
<script>
  (async () => {
    const viewer = new OFDViewerLib.OFDViewer({
      container: document.getElementById("viewer"),
    });
    await viewer.init();
  })();
</script>
```

### 底层 WASM 入口（手动 wiring）

```javascript
import init, { OFDParser } from "./pkg/ofd_rust.js";

await init();

// 加载 OFD 文件
const data = new Uint8Array(arrayBuffer);
const parser = new OFDParser(data);

// 解析
const result = parser.parse();
const pageCount = result.pageCount;

// 渲染页面
const pageResult = parser.render_page(0);
const pageWidth = pageResult.width;
const pageHeight = pageResult.height;

// 获取字体
const fonts = parser.get_fonts();
```

## API

### Bundle OFDViewer

- `new OFDViewer(options)` - 创建查看器
- `await viewer.init()` - 自动初始化同目录 `ofd-viewer.wasm`
- `await viewer.init(wasmUrl)` - 指定自定义 wasm 地址
- `await viewer.init(initWasm, OFDParser, wasmUrl?)` - 兼容原始手动初始化方式

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

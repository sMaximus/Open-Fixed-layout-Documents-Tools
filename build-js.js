/**
 * JS 打包压缩混淆脚本
 * 使用 Terser 进行压缩和混淆
 * wasm_exec.js 会被合并到每个输出文件中
 */

const fs = require("fs");
const path = require("path");
const { minify } = require("terser");

// 配置
const config = {
  // wasm_exec.js 会被合并到每个输出文件中
  wasmExec: "web/wasm_exec.js",
  // 输入文件（会与 wasm_exec.js 合并）
  input: [
    { src: "web/ofd-viewer.js", out: "ofd-viewer.min.js" },
    { src: "web/app.js", out: "app.min.js" },
  ],
  // 输出目录
  outputDir: "dist",
  // Terser 压缩选项
  terserOptions: {
    compress: {
      drop_console: false, // 保留 console（生产环境可设为 true）
      drop_debugger: true, // 移除 debugger
      dead_code: true, // 移除无用代码
      unused: true, // 移除未使用的变量
      passes: 2, // 压缩遍数
    },
    mangle: {
      toplevel: false, // 不混淆顶级变量（保留导出的类名）
      properties: false, // 不混淆属性名
    },
    format: {
      comments: false, // 移除注释
    },
    sourceMap: false, // 不生成 source map
  },
};

async function build() {
  console.log("🚀 开始打包 JS 文件...\n");

  // 确保输出目录存在
  if (!fs.existsSync(config.outputDir)) {
    fs.mkdirSync(config.outputDir, { recursive: true });
  }

  // 读取 wasm_exec.js
  let wasmExecCode = "";
  if (fs.existsSync(config.wasmExec)) {
    wasmExecCode = fs.readFileSync(config.wasmExec, "utf8");
    console.log(
      `📦 wasm_exec.js: ${formatSize(Buffer.byteLength(wasmExecCode, "utf8"))}`,
    );
    console.log("   将合并到所有输出文件中\n");
  } else {
    console.warn("⚠️  wasm_exec.js 不存在，跳过合并\n");
  }

  let totalOriginal = 0;
  let totalMinified = 0;

  for (const item of config.input) {
    const outputFile = path.join(config.outputDir, item.out);

    try {
      // 读取源文件
      const srcCode = fs.readFileSync(item.src, "utf8");

      // 合并 wasm_exec.js + 源文件
      const combinedCode = wasmExecCode + "\n" + srcCode;
      const originalSize = Buffer.byteLength(combinedCode, "utf8");
      totalOriginal += originalSize;

      console.log(`📦 处理: ${item.src} (含 wasm_exec.js)`);
      console.log(`   合并后大小: ${formatSize(originalSize)}`);

      // 压缩混淆
      const result = await minify(combinedCode, config.terserOptions);

      if (result.error) {
        throw result.error;
      }

      const minifiedSize = Buffer.byteLength(result.code, "utf8");
      totalMinified += minifiedSize;

      // 写入输出文件
      fs.writeFileSync(outputFile, result.code);

      const ratio = ((1 - minifiedSize / originalSize) * 100).toFixed(1);
      console.log(`   压缩后: ${formatSize(minifiedSize)} (减少 ${ratio}%)`);
      console.log(`   输出: ${outputFile}\n`);
    } catch (err) {
      console.error(`❌ 处理 ${item.src} 失败:`, err.message);
      process.exit(1);
    }
  }

  // 复制其他必要文件到 dist（不再需要复制 wasm_exec.js）
  const filesToCopy = ["web/ofd.wasm", "web/index.html"];

  console.log("📋 复制其他文件...");
  for (const file of filesToCopy) {
    if (fs.existsSync(file)) {
      const destFile = path.join(config.outputDir, path.basename(file));
      fs.copyFileSync(file, destFile);
      console.log(`   ${file} -> ${destFile}`);
    }
  }

  // 总结
  console.log("\n✅ 打包完成!");
  console.log(`   总原始大小: ${formatSize(totalOriginal)}`);
  console.log(`   总压缩后: ${formatSize(totalMinified)}`);
  console.log(
    `   总压缩率: ${((1 - totalMinified / totalOriginal) * 100).toFixed(1)}%`,
  );
}

function formatSize(bytes) {
  if (bytes < 1024) return bytes + " B";
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(2) + " KB";
  return (bytes / 1024 / 1024).toFixed(2) + " MB";
}

build().catch((err) => {
  console.error("构建失败:", err);
  process.exit(1);
});

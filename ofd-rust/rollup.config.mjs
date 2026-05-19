import fs from "node:fs";
import terser from "@rollup/plugin-terser";

/** 将 CSS 文件内联为 JS 字符串导出 */
function cssInline() {
  return {
    name: "css-inline",
    transform(code, id) {
      if (!id.endsWith(".css")) return null;
      return { code: `export default ${JSON.stringify(code)};`, map: null };
    },
  };
}

function emitWasmAsset(sourcePath, fileName) {
  return {
    name: "emit-wasm-asset",
    generateBundle() {
      this.emitFile({
        type: "asset",
        fileName,
        source: fs.readFileSync(sourcePath),
      });
    },
  };
}

const coreInput = "src-js/ofd-viewer.js";
const bundleInput = "src-js/ofd-viewer-bundle.js";

function corePlugins({ minify = false } = {}) {
  return minify ? [cssInline(), terser()] : [cssInline()];
}

function bundlePlugins({ minify = false } = {}) {
  const plugins = [cssInline(), emitWasmAsset("pkg/ofd_rust_bg.wasm", "ofd-viewer.wasm")];
  if (minify) plugins.push(terser());
  return plugins;
}

export default [
  {
    input: coreInput,
    output: { file: "dist/ofd-viewer.esm.js", format: "es", sourcemap: true },
    plugins: corePlugins(),
  },
  {
    input: coreInput,
    output: {
      file: "dist/ofd-viewer.esm.min.js",
      format: "es",
      sourcemap: true,
    },
    plugins: corePlugins({ minify: true }),
  },
  {
    input: coreInput,
    output: {
      file: "dist/ofd-viewer.umd.js",
      format: "umd",
      name: "OFDViewerLib",
      sourcemap: true,
      exports: "named",
    },
    plugins: corePlugins(),
  },
  {
    input: coreInput,
    output: {
      file: "dist/ofd-viewer.umd.min.js",
      format: "umd",
      name: "OFDViewerLib",
      sourcemap: true,
      exports: "named",
    },
    plugins: corePlugins({ minify: true }),
  },
  {
    input: bundleInput,
    output: {
      file: "dist/ofd-viewer.bundle.esm.js",
      format: "es",
      sourcemap: true,
    },
    plugins: bundlePlugins(),
  },
  {
    input: bundleInput,
    output: {
      file: "dist/ofd-viewer.bundle.esm.min.js",
      format: "es",
      sourcemap: true,
    },
    plugins: bundlePlugins({ minify: true }),
  },
  {
    input: bundleInput,
    output: {
      file: "dist/ofd-viewer.bundle.umd.js",
      format: "umd",
      name: "OFDViewerLib",
      sourcemap: true,
      exports: "named",
    },
    plugins: bundlePlugins(),
  },
  {
    input: bundleInput,
    output: {
      file: "dist/ofd-viewer.bundle.umd.min.js",
      format: "umd",
      name: "OFDViewerLib",
      sourcemap: true,
      exports: "named",
    },
    plugins: bundlePlugins({ minify: true }),
  },
];

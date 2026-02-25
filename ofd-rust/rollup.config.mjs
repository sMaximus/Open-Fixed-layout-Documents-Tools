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

const input = "src-js/ofd-viewer.js";
const plugins = [cssInline()];
const pluginsMin = [cssInline(), terser()];

export default [
  {
    input,
    output: { file: "dist/ofd-viewer.esm.js", format: "es", sourcemap: true },
    plugins,
  },
  {
    input,
    output: {
      file: "dist/ofd-viewer.esm.min.js",
      format: "es",
      sourcemap: true,
    },
    plugins: pluginsMin,
  },
  {
    input,
    output: {
      file: "dist/ofd-viewer.umd.js",
      format: "umd",
      name: "OFDViewerLib",
      sourcemap: true,
      exports: "named",
    },
    plugins,
  },
  {
    input,
    output: {
      file: "dist/ofd-viewer.umd.min.js",
      format: "umd",
      name: "OFDViewerLib",
      sourcemap: true,
      exports: "named",
    },
    plugins: pluginsMin,
  },
];

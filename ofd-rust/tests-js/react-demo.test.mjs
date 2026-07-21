import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, "..");
const demoPath = path.join(repoRoot, "demo-react.html");

assert.ok(fs.existsSync(demoPath), "expected demo-react.html to exist");

const source = fs.readFileSync(demoPath, "utf8");

for (const [label, pattern] of [
  ["React root", /id="root"/],
  ["React UMD script", /react(?:\.production\.min)?\.js/],
  ["ReactDOM UMD script", /react-dom(?:\.production\.min)?\.js/],
  ["viewer UMD bundle", /\.\/dist\/ofd-viewer\.umd\.js/],
  ["WASM module import", /from\s+["']\.\/pkg\/ofd_rust\.js["']/],
  ["OFDViewer construction", /new\s+OFDViewerLib\.OFDViewer/],
  ["React lifecycle", /React\.useEffect/],
  ["file loading", /\.loadFile\(file\)/],
  ["viewer cleanup", /\.destroy\(\)/],
]) {
  assert.match(source, pattern, `expected demo-react.html to include ${label}`);
}

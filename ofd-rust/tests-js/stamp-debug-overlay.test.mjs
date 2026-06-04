import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, "..");

function read(relativePath) {
  return fs.readFileSync(path.join(repoRoot, relativePath), "utf8");
}

const componentSource = read("src-js/ofd-viewer.js");
const componentCss = read("src-js/ofd-viewer.css");
const pkgSource = read("pkg/ofd_rust.js");

for (const [label, source] of [
  ["src-js/ofd-viewer.js", componentSource],
  ["pkg/ofd_rust.js", pkgSource],
]) {
  assert.match(
    source,
    /stampDebug/,
    `${label} should consume stampDebug from render_page_svg output`,
  );
  assert.match(
    source,
    /ofd-stamp-debug-layer/,
    `${label} should render a page-visible stamp debug layer`,
  );
  assert.match(
    source,
    /定位符/,
    `${label} should label the printed locator information in Chinese`,
  );
}

assert.match(
  componentCss,
  /\.ofd-stamp-debug-layer/,
  "component CSS should style the stamp debug layer",
);

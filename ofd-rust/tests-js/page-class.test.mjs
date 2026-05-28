import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, "..");

const viewerSource = fs.readFileSync(
  path.join(repoRoot, "src-js", "ofd-viewer.js"),
  "utf8",
);

assert.match(
  viewerSource,
  /className\s*=\s*`ofd-page ofd-page-\$\{index\}`/,
  "expected _createPage to add a page-specific class name based on the page index",
);

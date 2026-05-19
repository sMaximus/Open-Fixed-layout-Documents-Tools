import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, "..");

const bundleEsmPath = path.join(repoRoot, "dist", "ofd-viewer.bundle.esm.js");
const bundleUmdPath = path.join(repoRoot, "dist", "ofd-viewer.bundle.umd.js");
const bundleWasmPath = path.join(repoRoot, "dist", "ofd-viewer.wasm");

function readJson(relativePath) {
  return JSON.parse(
    fs.readFileSync(path.join(repoRoot, relativePath), "utf8"),
  );
}

function stripComments(source) {
  return source
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/(^|\s)\/\/.*$/gm, "$1");
}

function runCheck(name, fn) {
  try {
    fn();
    console.log(`PASS ${name}`);
  } catch (error) {
    console.error(`FAIL ${name}`);
    throw error;
  }
}

runCheck("bundled viewer build emits JS and wasm artifacts in dist", () => {
  assert.ok(
    fs.existsSync(bundleEsmPath),
    "expected dist/ofd-viewer.bundle.esm.js to exist",
  );
  assert.ok(
    fs.existsSync(bundleUmdPath),
    "expected dist/ofd-viewer.bundle.umd.js to exist",
  );
  assert.ok(
    fs.existsSync(bundleWasmPath),
    "expected dist/ofd-viewer.wasm to exist",
  );
});

runCheck("bundled entry is exported from package.json", () => {
  const pkg = readJson("package.json");

  assert.deepEqual(pkg.exports["./bundle"], {
    import: "./dist/ofd-viewer.bundle.esm.js",
    require: "./dist/ofd-viewer.bundle.umd.js",
  });
});

runCheck(
  "bundled JS resolves its wasm from dist instead of pkg glue imports",
  () => {
    const bundleText = fs.readFileSync(bundleEsmPath, "utf8");
    const executableText = stripComments(bundleText);

    assert.doesNotMatch(
      executableText,
      /from\s+["'](?:\.\.\/|\.\.\\|\.\/|\.\\)?pkg\/ofd_rust\.js["']/,
      "bundle should not import pkg/ofd_rust.js at runtime",
    );
    assert.match(
      bundleText,
      /ofd-viewer\.wasm/,
      "bundle should refer to the copied dist wasm asset",
    );
    assert.match(
      executableText,
      /function initWasm\(moduleOrPath = DEFAULT_WASM_URL\)/,
      "bundle should export an initWasm wrapper that defaults to the dist wasm",
    );
  },
);

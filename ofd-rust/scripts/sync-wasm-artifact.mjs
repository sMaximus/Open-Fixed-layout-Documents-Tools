import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const repoRoot = path.resolve(path.dirname(__filename), "..");
const source = path.join(
  repoRoot,
  "target",
  "wasm-pack-pkg",
  "ofd_rust_bg.wasm",
);
const destination = path.join(repoRoot, "pkg", "ofd_rust_bg.wasm");

fs.copyFileSync(source, destination);

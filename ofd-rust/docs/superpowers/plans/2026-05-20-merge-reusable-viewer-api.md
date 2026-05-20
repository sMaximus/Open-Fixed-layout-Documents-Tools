# Merge Reusable Viewer API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move reusable OFD viewer behavior from `index.js` into exported APIs in `pkg/ofd_rust.js`, leaving `index.js` as a thin caller.

**Architecture:** Keep wasm-bindgen exports intact and add a viewer factory at the bottom of `pkg/ofd_rust.js`. `createOFDViewer(options)` owns all viewer state and returns reusable methods; `initOFDViewer(options)` initializes WASM, creates a viewer, binds the existing DOM, and exposes compatibility methods on `window`.

**Tech Stack:** JavaScript ES modules, wasm-bindgen web target, browser DOM APIs, existing OFD parser WASM exports.

---

## File Structure

- Modify `D:/GO/Open-Fixed-layout-Documents-Tools/ofd-rust/pkg/ofd_rust.js`
  - Keep generated `OFDParser`, `init`, `initSync`, and default export unchanged.
  - Add `createOFDViewer(options = {})` and `initOFDViewer(options = {})` after the existing export line.
  - Move reusable stateful viewer functions from `index.js` into the factory closure.
- Modify `D:/GO/Open-Fixed-layout-Documents-Tools/ofd-rust/index.js`
  - Replace the full viewer implementation with a small import/call entrypoint.
- Optionally modify `D:/GO/Open-Fixed-layout-Documents-Tools/ofd-rust/pkg/ofd_rust.d.ts`
  - Add declarations for the two new exported viewer helper functions if the file exists and already declares generated exports.

## Task 1: Add exported viewer factory to `pkg/ofd_rust.js`

**Files:**
- Modify: `D:/GO/Open-Fixed-layout-Documents-Tools/ofd-rust/pkg/ofd_rust.js`

- [ ] **Step 1: Add the factory shell after the existing export line**

Append this code immediately after:

```js
export { initSync, __wbg_init as default };
```

Add:

```js
export function createOFDViewer(options = {}) {
    const doc = options.document || globalThis.document;
    const win = options.window || globalThis.window;
    if (!doc || !win) {
        throw new Error("createOFDViewer requires a browser-like document and window.");
    }

    let parser = null;
    let currentPageCount = 0;
    const BASE_SCALE = options.baseScale ?? 3.78;
    let currentZoom = 1.0;
    const ZOOM_STEP = options.zoomStep ?? 0.25;
    const ZOOM_MIN = options.zoomMin ?? 0.25;
    const ZOOM_MAX = options.zoomMax ?? 5.0;
    const loadedFonts = new Map();
    let allPagesData = [];
    const renderedPages = new Set();
    const INITIAL_PAGES = options.initialPages ?? 3;
    const PRELOAD_THRESHOLD = options.preloadThreshold ?? 200;
    const MOCK_SEAL_BASE64 =
        "PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIxMjAiIGhlaWdodD0iMTIwIiB2aWV3Qm94PSIwIDAgMTIwIDEyMCI+PGNpcmNsZSBjeD0iNjAiIGN5PSI2MCIgcj0iNTIiIGZpbGw9Im5vbmUiIHN0cm9rZT0iI2Q4MDAwMCIgc3Ryb2tlLXdpZHRoPSI2Ii8+PGNpcmNsZSBjeD0iNjAiIGN5PSI2MCIgcj0iMzIiIGZpbGw9Im5vbmUiIHN0cm9rZT0iI2Q4MDAwMCIgc3Ryb2tlPSIjZDgwMDAwIiBzdHJva2Utd2lkdGg9IjMiIHN0cm9rZS1kYXNoYXJyYXk9IjQgNCIvPjwvc3ZnPg==";
    let sealPlacementActive = false;
    let sealCursorEl = null;
    let sealDataUrl = "";
    let previousBodyCursor = "";

    function getElement(id) {
        return doc.getElementById(id);
    }

    function getScale() {
        return BASE_SCALE * currentZoom;
    }

    function updateZoomDisplay() {
        const el = getElement("zoomLevel");
        if (el) el.textContent = Math.round(currentZoom * 100) + "%";
    }

    function updateStatus(msg) {
        const status = getElement("status");
        if (status) status.textContent = msg;
    }

    return {
        parseAndRender,
        showXmlModal,
        hideXmlModal,
        zoomIn,
        zoomOut,
        resetZoom,
        setZoom,
        startSealPlacement,
        startSealPlacementWithMock,
        printOFD,
        stopSealPlacement,
        destroy,
    };
}
```

- [ ] **Step 2: Move the remaining helper functions into the factory closure**

Copy the function bodies from `index.js` into `createOFDViewer` before the `return` statement. Preserve these exact function names because the returned object references them:

```js
applyZoomToPages
setZoom
zoomIn
zoomOut
resetZoom
normalizeSealDataUrl
stopSealPlacement
startSealPlacement
handleSealMouseMove
getPageDataByIndex
handleSealPlacementClick
startSealPlacementWithMock
infoItem
displayDocInfo
displayPageList
scrollToPage
loadOFDFonts
parseAndRender
renderAllPages
createPageContainer
renderPageContent
setupScrollListener
loadAndRenderPage
getFileIcon
buildFileTree
countTreeFiles
renderTreeNode
showXmlModal
hideXmlModal
formatXml
escHtml
highlightXml
printOFD
```

When moving the code, make these mechanical substitutions:

```js
document.  -> doc.
window.    -> win.
document.getElementById("x") -> getElement("x")
```

Do not change the existing HTML ids/classes or user-facing messages.

- [ ] **Step 3: Add `destroy()` inside the factory**

Add this function before the returned object:

```js
function destroy() {
    stopSealPlacement();
    doc.removeEventListener("keydown", handleKeyDown);
    doc.body.removeEventListener("dragover", handleDragOver);
    doc.body.removeEventListener("drop", handleDrop);
}
```

This compiles after Task 2 adds `handleKeyDown`, `handleDragOver`, and `handleDrop` in the binding task.

- [ ] **Step 4: Run a syntax check**

Run:

```bash
node --check pkg/ofd_rust.js
```

Expected: no output and exit code 0. If it fails, fix the reported syntax error before continuing.

## Task 2: Add DOM binding initializer to `pkg/ofd_rust.js`

**Files:**
- Modify: `D:/GO/Open-Fixed-layout-Documents-Tools/ofd-rust/pkg/ofd_rust.js`

- [ ] **Step 1: Add event handler functions inside `createOFDViewer`**

Before `destroy()`, add:

```js
function handleKeyDown(e) {
    if (e.key === "Escape") {
        hideXmlModal();
        stopSealPlacement();
    }
    if ((e.ctrlKey || e.metaKey) && (e.key === "+" || e.key === "=")) {
        e.preventDefault();
        zoomIn();
    }
    if ((e.ctrlKey || e.metaKey) && e.key === "-") {
        e.preventDefault();
        zoomOut();
    }
    if ((e.ctrlKey || e.metaKey) && e.key === "0") {
        e.preventDefault();
        resetZoom();
    }
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "p") {
        if (parser && allPagesData.length) {
            e.preventDefault();
            printOFD();
        }
    }
}

function handleFileInputChange(e) {
    const file = e.target.files[0];
    if (file) parseAndRender(file);
}

function handleDragOver(e) {
    e.preventDefault();
    e.stopPropagation();
}

function handleDrop(e) {
    e.preventDefault();
    e.stopPropagation();
    const file = e.dataTransfer.files[0];
    if (file && file.name.toLowerCase().endsWith(".ofd")) parseAndRender(file);
}

function bindDomEvents() {
    const fileInput = getElement("fileInput");
    if (fileInput) fileInput.addEventListener("change", handleFileInputChange);
    doc.addEventListener("keydown", handleKeyDown);
    doc.body.addEventListener("dragover", handleDragOver);
    doc.body.addEventListener("drop", handleDrop);
}

function exposeWindowMethods(targetWindow = win) {
    targetWindow.showXmlModal = showXmlModal;
    targetWindow.hideXmlModal = hideXmlModal;
    targetWindow.zoomIn = zoomIn;
    targetWindow.zoomOut = zoomOut;
    targetWindow.resetZoom = resetZoom;
    targetWindow.setZoom = setZoom;
    targetWindow.startSealPlacement = startSealPlacement;
    targetWindow.startSealPlacementWithMock = startSealPlacementWithMock;
    targetWindow.printOFD = printOFD;
}
```

- [ ] **Step 2: Include binding helpers in the returned object**

Update the returned object to include:

```js
bindDomEvents,
exposeWindowMethods,
```

The final return object should contain all public viewer methods plus these two helpers.

- [ ] **Step 3: Update `destroy()` to remove all bound listeners**

Replace `destroy()` with:

```js
function destroy() {
    stopSealPlacement();
    const fileInput = getElement("fileInput");
    if (fileInput) fileInput.removeEventListener("change", handleFileInputChange);
    doc.removeEventListener("keydown", handleKeyDown);
    doc.body.removeEventListener("dragover", handleDragOver);
    doc.body.removeEventListener("drop", handleDrop);
}
```

- [ ] **Step 4: Add exported initializer after `createOFDViewer`**

Append after the factory:

```js
export async function initOFDViewer(options = {}) {
    await __wbg_init(options.wasmModuleOrPath);
    const viewer = createOFDViewer(options);
    viewer.bindDomEvents();
    viewer.exposeWindowMethods(options.window || globalThis.window);
    const updateStatus = options.updateStatus;
    if (typeof updateStatus === "function") {
        updateStatus("✅ 就绪");
    } else {
        const doc = options.document || globalThis.document;
        const status = doc?.getElementById("status");
        if (status) status.textContent = "✅ 就绪";
    }
    return viewer;
}
```

- [ ] **Step 5: Run syntax check**

Run:

```bash
node --check pkg/ofd_rust.js
```

Expected: no output and exit code 0.

## Task 3: Replace `index.js` with thin entrypoint

**Files:**
- Modify: `D:/GO/Open-Fixed-layout-Documents-Tools/ofd-rust/index.js`

- [ ] **Step 1: Replace file contents**

Replace all of `index.js` with:

```js
import { initOFDViewer } from "./pkg/ofd_rust.js";

initOFDViewer().catch((err) => {
  const status = document.getElementById("status");
  if (status) status.textContent = "❌ WASM 加载失败";
  console.error(err);
});
```

- [ ] **Step 2: Run syntax checks**

Run:

```bash
node --check index.js && node --check pkg/ofd_rust.js
```

Expected: no output and exit code 0.

## Task 4: Update TypeScript declarations if present

**Files:**
- Modify: `D:/GO/Open-Fixed-layout-Documents-Tools/ofd-rust/pkg/ofd_rust.d.ts`

- [ ] **Step 1: Inspect declaration file**

Read `pkg/ofd_rust.d.ts`. If it exists, add declarations for the new APIs without changing generated declarations.

- [ ] **Step 2: Append API declarations**

Append:

```ts
export interface OFDViewer {
  parseAndRender(file: File): Promise<void>;
  showXmlModal(): void;
  hideXmlModal(): void;
  zoomIn(): void;
  zoomOut(): void;
  resetZoom(): void;
  setZoom(zoom: number): void;
  startSealPlacement(sealBase64: string, mimeType?: string): void;
  startSealPlacementWithMock(): void;
  printOFD(): void;
  stopSealPlacement(): void;
  bindDomEvents(): void;
  exposeWindowMethods(targetWindow?: Window): void;
  destroy(): void;
}

export interface OFDViewerOptions {
  document?: Document;
  window?: Window;
  wasmModuleOrPath?: RequestInfo | URL | Response | BufferSource | WebAssembly.Module;
  baseScale?: number;
  zoomStep?: number;
  zoomMin?: number;
  zoomMax?: number;
  initialPages?: number;
  preloadThreshold?: number;
  updateStatus?: (message: string) => void;
}

export function createOFDViewer(options?: OFDViewerOptions): OFDViewer;
export function initOFDViewer(options?: OFDViewerOptions): Promise<OFDViewer>;
```

- [ ] **Step 3: Run TypeScript declaration syntax check if TypeScript is available**

Run:

```bash
node -e "import('typescript').then(ts=>{const fs=require('fs');const s=fs.readFileSync('pkg/ofd_rust.d.ts','utf8');const r=ts.transpileModule(s,{compilerOptions:{module:ts.ModuleKind.ESNext}}); if(r.diagnostics?.length){console.error(r.diagnostics);process.exit(1)}}).catch(()=>process.exit(0))"
```

Expected: exit code 0.

## Task 5: Browser smoke test

**Files:**
- Verify: `D:/GO/Open-Fixed-layout-Documents-Tools/ofd-rust/index.html` or the existing viewer HTML page loaded by `server.js`

- [ ] **Step 1: Start dev server**

Run:

```bash
npm run dev
```

Expected: server starts without throwing an import error.

- [ ] **Step 2: Open the viewer in a browser**

Open the local URL printed by `server.js`.

Expected initial UI state:

```text
status shows: ✅ 就绪
browser console has no module import error
```

- [ ] **Step 3: Test golden path**

Load a valid `.ofd` file through the file input or drag/drop.

Expected:

```text
status changes through 解析中... / 加载字体... / 渲染页面...
pages render in the viewer
XML button, zoom controls, print button, and seal button become visible
```

- [ ] **Step 4: Test exported window compatibility methods**

In the browser console, run:

```js
typeof window.zoomIn === "function" &&
typeof window.zoomOut === "function" &&
typeof window.showXmlModal === "function" &&
typeof window.printOFD === "function"
```

Expected: `true`.

- [ ] **Step 5: Test interactions**

Use the UI and shortcuts:

```text
Ctrl+= increases zoom percentage
Ctrl+- decreases zoom percentage
Ctrl+0 resets zoom to 100%
XML button opens the XML modal
Esc closes XML modal
seal placement mock button enters crosshair mode and click on a page reports OFD coordinates
Ctrl+P opens print flow after a document is loaded
```

Expected: behavior matches the original viewer.

## Task 6: Final verification

**Files:**
- Verify: `D:/GO/Open-Fixed-layout-Documents-Tools/ofd-rust/index.js`
- Verify: `D:/GO/Open-Fixed-layout-Documents-Tools/ofd-rust/pkg/ofd_rust.js`
- Verify: `D:/GO/Open-Fixed-layout-Documents-Tools/ofd-rust/pkg/ofd_rust.d.ts`

- [ ] **Step 1: Run static checks**

Run:

```bash
node --check index.js && node --check pkg/ofd_rust.js
```

Expected: no output and exit code 0.

- [ ] **Step 2: Run existing package checks if available**

Run:

```bash
npm run test:bundle
```

Expected: existing bundle artifact test passes, or fails only because required build artifacts are absent before this change. If it fails for an import/export reason introduced by this work, fix it.

- [ ] **Step 3: Inspect changed files**

Run:

```bash
git diff -- index.js pkg/ofd_rust.js pkg/ofd_rust.d.ts
```

Expected:

```text
index.js is a thin caller
pkg/ofd_rust.js still exports default wasm initializer and initSync
pkg/ofd_rust.js additionally exports createOFDViewer and initOFDViewer
pkg/ofd_rust.d.ts declares the new APIs if edited
```

- [ ] **Step 4: Commit if requested by the user**

Only commit if the user explicitly asks. Use a message like:

```bash
git add index.js pkg/ofd_rust.js pkg/ofd_rust.d.ts
git commit -m "refactor: expose reusable OFD viewer API"
```

## Self-Review

- Spec coverage: the plan moves reusable initialization, parsing/rendering, printing, XML modal, zoom, and seal placement from `index.js` to exported methods in `pkg/ofd_rust.js`; `index.js` becomes a caller only.
- Placeholder scan: no TBD/TODO/fill-in placeholders remain.
- Type consistency: public functions are consistently named `createOFDViewer` and `initOFDViewer`; returned viewer methods match the declaration names.

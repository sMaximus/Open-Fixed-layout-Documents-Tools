# React Demo Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a standalone React demo that mirrors the existing OFD viewer entry page while reusing the packaged `OFDViewer` browser API.

**Architecture:** Create a single `demo-react.html` alongside the existing demos. React owns the toolbar state, file input, and lifecycle; `OFDViewer` owns WASM initialization and page rendering inside a referenced container. Add a small static Node test that verifies the demo wiring.

**Tech Stack:** React 18 UMD via CDN, browser module import for `pkg/ofd_rust.js`, existing `dist/ofd-viewer.umd.js`, Node `assert` for static tests.

---

## Chunk 1: Static Demo Contract

### Task 1: React Demo Test

**Files:**
- Create: `tests-js/react-demo.test.mjs`
- Create: `demo-react.html`

- [ ] **Step 1: Write the failing test**

Add a static test that asserts `demo-react.html` exists and contains the expected integration points: React root, React scripts, `dist/ofd-viewer.umd.js`, `pkg/ofd_rust.js`, `new OFDViewerLib.OFDViewer`, `React.useEffect`, `loadFile(file)`, and `destroy()`.

- [ ] **Step 2: Run test to verify it fails**

Run: `node ./tests-js/react-demo.test.mjs`
Expected: FAIL because `demo-react.html` does not exist.

- [ ] **Step 3: Write minimal implementation**

Create `demo-react.html` with a React app:
- toolbar with title, file input, reset button, status, and page count
- viewer container managed by `useRef`
- `useEffect` initializes `OFDViewer` with `initWasm` and `OFDParser`
- `onFileChange` calls `viewer.loadFile(file)`
- cleanup calls `viewer.destroy()`

- [ ] **Step 4: Run test to verify it passes**

Run: `node ./tests-js/react-demo.test.mjs`
Expected: PASS.

- [ ] **Step 5: Run existing JS checks**

Run: `node ./tests-js/bundle-artifacts.test.mjs`
Expected: PASS.

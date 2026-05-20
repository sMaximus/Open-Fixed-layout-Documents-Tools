/* @ts-self-types="./ofd_rust.d.ts" */

/**
 * WASM 导出的 OFD 解析器
 */
export class OFDParser {
  __destroy_into_raw() {
    const ptr = this.__wbg_ptr;
    this.__wbg_ptr = 0;
    OFDParserFinalization.unregister(this);
    return ptr;
  }
  free() {
    const ptr = this.__destroy_into_raw();
    wasm.__wbg_ofdparser_free(ptr, 0);
  }
  /**
   * 获取文档信息
   * @returns {any}
   */
  get_doc_info() {
    const ret = wasm.ofdparser_get_doc_info(this.__wbg_ptr);
    return ret;
  }
  /**
   * 获取文件列表
   * @returns {any}
   */
  get_files() {
    const ret = wasm.ofdparser_get_files(this.__wbg_ptr);
    return ret;
  }
  /**
   * 获取字体信息
   * @returns {any}
   */
  get_fonts() {
    const ret = wasm.ofdparser_get_fonts(this.__wbg_ptr);
    return ret;
  }
  /**
   * 获取页数
   * @returns {number}
   */
  get_page_count() {
    const ret = wasm.ofdparser_get_page_count(this.__wbg_ptr);
    return ret >>> 0;
  }
  /**
   * 获取页面尺寸
   * @param {number} index
   * @returns {any}
   */
  get_page_size(index) {
    const ret = wasm.ofdparser_get_page_size(this.__wbg_ptr, index);
    return ret;
  }
  /**
   * 创建解析器
   * @param {Uint8Array} data
   */
  constructor(data) {
    const ptr0 = passArray8ToWasm0(data, wasm.__wbindgen_malloc);
    const len0 = WASM_VECTOR_LEN;
    const ret = wasm.ofdparser_new(ptr0, len0);
    if (ret[2]) {
      throw takeFromExternrefTable0(ret[1]);
    }
    this.__wbg_ptr = ret[0] >>> 0;
    OFDParserFinalization.register(this, this.__wbg_ptr, this);
    return this;
  }
  /**
   * 解析OFD文件
   * @returns {any}
   */
  parse() {
    const ret = wasm.ofdparser_parse(this.__wbg_ptr);
    if (ret[2]) {
      throw takeFromExternrefTable0(ret[1]);
    }
    return takeFromExternrefTable0(ret[0]);
  }
  /**
   * 读取OFD包内文件的文本内容（用于查看XML）
   * @param {string} name
   * @returns {any}
   */
  read_file_text(name) {
    const ptr0 = passStringToWasm0(
      name,
      wasm.__wbindgen_malloc,
      wasm.__wbindgen_realloc,
    );
    const len0 = WASM_VECTOR_LEN;
    const ret = wasm.ofdparser_read_file_text(this.__wbg_ptr, ptr0, len0);
    return ret;
  }
  /**
   * 渲染页面（Canvas 模式）
   * @param {number} index
   * @returns {any}
   */
  render_page(index) {
    const ret = wasm.ofdparser_render_page(this.__wbg_ptr, index);
    return ret;
  }
  /**
   * 渲染页面为 SVG
   * @param {number} index
   * @returns {any}
   */
  render_page_svg(index) {
    const ret = wasm.ofdparser_render_page_svg(this.__wbg_ptr, index);
    return ret;
  }
  /**
   * 渲染页面为 SVG（带缩放比例）
   * @param {number} index
   * @param {number} zoom
   * @returns {any}
   */
  render_page_svg_scaled(index, zoom) {
    const ret = wasm.ofdparser_render_page_svg_scaled(
      this.__wbg_ptr,
      index,
      zoom,
    );
    return ret;
  }
}
if (Symbol.dispose)
  OFDParser.prototype[Symbol.dispose] = OFDParser.prototype.free;

/**
 * 初始化 panic hook（用于调试）
 */
export function init() {
  wasm.init();
}

function __wbg_get_imports() {
  const import0 = {
    __proto__: null,
    __wbg_Error_8c4e43fe74559d73: function (arg0, arg1) {
      const ret = Error(getStringFromWasm0(arg0, arg1));
      return ret;
    },
    __wbg___wbindgen_is_string_cd444516edc5b180: function (arg0) {
      const ret = typeof arg0 === "string";
      return ret;
    },
    __wbg___wbindgen_throw_be289d5034ed271b: function (arg0, arg1) {
      throw new Error(getStringFromWasm0(arg0, arg1));
    },
    __wbg_log_6b5ca2e6124b2808: function (arg0) {
      console.log(arg0);
    },
    __wbg_new_361308b2356cecd0: function () {
      const ret = new Object();
      return ret;
    },
    __wbg_new_3eb36ae241fe6f44: function () {
      const ret = new Array();
      return ret;
    },
    __wbg_new_dca287b076112a51: function () {
      const ret = new Map();
      return ret;
    },
    __wbg_set_1eb0999cf5d27fc8: function (arg0, arg1, arg2) {
      const ret = arg0.set(arg1, arg2);
      return ret;
    },
    __wbg_set_3f1d0b984ed272ed: function (arg0, arg1, arg2) {
      arg0[arg1] = arg2;
    },
    __wbg_set_6cb8631f80447a67: function () {
      return handleError(function (arg0, arg1, arg2) {
        const ret = Reflect.set(arg0, arg1, arg2);
        return ret;
      }, arguments);
    },
    __wbg_set_f43e577aea94465b: function (arg0, arg1, arg2) {
      arg0[arg1 >>> 0] = arg2;
    },
    __wbg_warn_f7ae1b2e66ccb930: function (arg0) {
      console.warn(arg0);
    },
    __wbindgen_cast_0000000000000001: function (arg0) {
      // Cast intrinsic for `F64 -> Externref`.
      const ret = arg0;
      return ret;
    },
    __wbindgen_cast_0000000000000002: function (arg0) {
      // Cast intrinsic for `I64 -> Externref`.
      const ret = arg0;
      return ret;
    },
    __wbindgen_cast_0000000000000003: function (arg0, arg1) {
      // Cast intrinsic for `Ref(String) -> Externref`.
      const ret = getStringFromWasm0(arg0, arg1);
      return ret;
    },
    __wbindgen_cast_0000000000000004: function (arg0) {
      // Cast intrinsic for `U64 -> Externref`.
      const ret = BigInt.asUintN(64, arg0);
      return ret;
    },
    __wbindgen_init_externref_table: function () {
      const table = wasm.__wbindgen_externrefs;
      const offset = table.grow(4);
      table.set(0, undefined);
      table.set(offset + 0, undefined);
      table.set(offset + 1, null);
      table.set(offset + 2, true);
      table.set(offset + 3, false);
    },
  };
  return {
    __proto__: null,
    "./ofd_rust_bg.js": import0,
  };
}

const OFDParserFinalization =
  typeof FinalizationRegistry === "undefined"
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry((ptr) =>
        wasm.__wbg_ofdparser_free(ptr >>> 0, 1),
      );

function addToExternrefTable0(obj) {
  const idx = wasm.__externref_table_alloc();
  wasm.__wbindgen_externrefs.set(idx, obj);
  return idx;
}

function getStringFromWasm0(ptr, len) {
  ptr = ptr >>> 0;
  return decodeText(ptr, len);
}

let cachedUint8ArrayMemory0 = null;
function getUint8ArrayMemory0() {
  if (
    cachedUint8ArrayMemory0 === null ||
    cachedUint8ArrayMemory0.byteLength === 0
  ) {
    cachedUint8ArrayMemory0 = new Uint8Array(wasm.memory.buffer);
  }
  return cachedUint8ArrayMemory0;
}

function handleError(f, args) {
  try {
    return f.apply(this, args);
  } catch (e) {
    const idx = addToExternrefTable0(e);
    wasm.__wbindgen_exn_store(idx);
  }
}

function passArray8ToWasm0(arg, malloc) {
  const ptr = malloc(arg.length * 1, 1) >>> 0;
  getUint8ArrayMemory0().set(arg, ptr / 1);
  WASM_VECTOR_LEN = arg.length;
  return ptr;
}

function passStringToWasm0(arg, malloc, realloc) {
  if (realloc === undefined) {
    const buf = cachedTextEncoder.encode(arg);
    const ptr = malloc(buf.length, 1) >>> 0;
    getUint8ArrayMemory0()
      .subarray(ptr, ptr + buf.length)
      .set(buf);
    WASM_VECTOR_LEN = buf.length;
    return ptr;
  }

  let len = arg.length;
  let ptr = malloc(len, 1) >>> 0;

  const mem = getUint8ArrayMemory0();

  let offset = 0;

  for (; offset < len; offset++) {
    const code = arg.charCodeAt(offset);
    if (code > 0x7f) break;
    mem[ptr + offset] = code;
  }
  if (offset !== len) {
    if (offset !== 0) {
      arg = arg.slice(offset);
    }
    ptr = realloc(ptr, len, (len = offset + arg.length * 3), 1) >>> 0;
    const view = getUint8ArrayMemory0().subarray(ptr + offset, ptr + len);
    const ret = cachedTextEncoder.encodeInto(arg, view);

    offset += ret.written;
    ptr = realloc(ptr, len, offset, 1) >>> 0;
  }

  WASM_VECTOR_LEN = offset;
  return ptr;
}

function takeFromExternrefTable0(idx) {
  const value = wasm.__wbindgen_externrefs.get(idx);
  wasm.__externref_table_dealloc(idx);
  return value;
}

let cachedTextDecoder = new TextDecoder("utf-8", {
  ignoreBOM: true,
  fatal: true,
});
cachedTextDecoder.decode();
const MAX_SAFARI_DECODE_BYTES = 2146435072;
let numBytesDecoded = 0;
function decodeText(ptr, len) {
  numBytesDecoded += len;
  if (numBytesDecoded >= MAX_SAFARI_DECODE_BYTES) {
    cachedTextDecoder = new TextDecoder("utf-8", {
      ignoreBOM: true,
      fatal: true,
    });
    cachedTextDecoder.decode();
    numBytesDecoded = len;
  }
  return cachedTextDecoder.decode(
    getUint8ArrayMemory0().subarray(ptr, ptr + len),
  );
}

const cachedTextEncoder = new TextEncoder();

if (!("encodeInto" in cachedTextEncoder)) {
  cachedTextEncoder.encodeInto = function (arg, view) {
    const buf = cachedTextEncoder.encode(arg);
    view.set(buf);
    return {
      read: arg.length,
      written: buf.length,
    };
  };
}

let WASM_VECTOR_LEN = 0;

let wasmModule, wasm;
function __wbg_finalize_init(instance, module) {
  wasm = instance.exports;
  wasmModule = module;
  cachedUint8ArrayMemory0 = null;
  wasm.__wbindgen_start();
  return wasm;
}

async function __wbg_load(module, imports) {
  if (typeof Response === "function" && module instanceof Response) {
    if (typeof WebAssembly.instantiateStreaming === "function") {
      try {
        return await WebAssembly.instantiateStreaming(module, imports);
      } catch (e) {
        const validResponse = module.ok && expectedResponseType(module.type);

        if (
          validResponse &&
          module.headers.get("Content-Type") !== "application/wasm"
        ) {
          console.warn(
            "`WebAssembly.instantiateStreaming` failed because your server does not serve Wasm with `application/wasm` MIME type. Falling back to `WebAssembly.instantiate` which is slower. Original error:\n",
            e,
          );
        } else {
          throw e;
        }
      }
    }

    const bytes = await module.arrayBuffer();
    return await WebAssembly.instantiate(bytes, imports);
  } else {
    const instance = await WebAssembly.instantiate(module, imports);

    if (instance instanceof WebAssembly.Instance) {
      return { instance, module };
    } else {
      return instance;
    }
  }

  function expectedResponseType(type) {
    switch (type) {
      case "basic":
      case "cors":
      case "default":
        return true;
    }
    return false;
  }
}

function initSync(module) {
  if (wasm !== undefined) return wasm;

  if (module !== undefined) {
    if (Object.getPrototypeOf(module) === Object.prototype) {
      ({ module } = module);
    } else {
      console.warn(
        "using deprecated parameters for `initSync()`; pass a single object instead",
      );
    }
  }

  const imports = __wbg_get_imports();
  if (!(module instanceof WebAssembly.Module)) {
    module = new WebAssembly.Module(module);
  }
  const instance = new WebAssembly.Instance(module, imports);
  return __wbg_finalize_init(instance, module);
}

async function __wbg_init(module_or_path) {
  if (wasm !== undefined) return wasm;

  if (module_or_path !== undefined) {
    if (Object.getPrototypeOf(module_or_path) === Object.prototype) {
      ({ module_or_path } = module_or_path);
    } else {
      console.warn(
        "using deprecated parameters for the initialization function; pass a single object instead",
      );
    }
  }

  if (module_or_path === undefined) {
    module_or_path = new URL("ofd_rust_bg.wasm", import.meta.url);
  }
  const imports = __wbg_get_imports();

  if (
    typeof module_or_path === "string" ||
    (typeof Request === "function" && module_or_path instanceof Request) ||
    (typeof URL === "function" && module_or_path instanceof URL)
  ) {
    module_or_path = fetch(module_or_path);
  }

  const { instance, module } = await __wbg_load(await module_or_path, imports);

  return __wbg_finalize_init(instance, module);
}

export { initSync, __wbg_init as default };

export function createOFDViewer(options = {}) {
  const doc = options.document || globalThis.document;
  const win = options.window || globalThis.window;
  if (!doc || !win) {
    throw new Error(
      "createOFDViewer requires a browser-like document and window.",
    );
  }
  const FontFaceCtor = win.FontFace || globalThis.FontFace;

  function getElement(id) {
    return doc.getElementById(id);
  }

  let parser = null;
  let currentPageCount = 0;
  const BASE_SCALE = 3.78;
  let currentZoom = 1.0;
  const ZOOM_STEP = 0.25;
  const ZOOM_MIN = 0.25;
  const ZOOM_MAX = 5.0;
  const loadedFonts = new Map();
  let allPagesData = [];
  const renderedPages = new Set();
  const INITIAL_PAGES = 3;
  const PRELOAD_THRESHOLD = 200;
  const MOCK_SEAL_BASE64 =
    "PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIxMjAiIGhlaWdodD0iMTIwIiB2aWV3Qm94PSIwIDAgMTIwIDEyMCI+PGNpcmNsZSBjeD0iNjAiIGN5PSI2MCIgcj0iNTIiIGZpbGw9Im5vbmUiIHN0cm9rZT0iI2Q4MDAwMCIgc3Ryb2tlLXdpZHRoPSI2Ii8+PGNpcmNsZSBjeD0iNjAiIGN5PSI2MCIgcj0iMzIiIGZpbGw9Im5vbmUiIHN0cm9rZT0iI2Q4MDAwMCIgc3Ryb2tlLXdpZHRoPSIzIiBzdHJva2UtZGFzaGFycmF5PSI0IDQiLz48L3N2Zz4=";
  let sealPlacementActive = false;
  let sealCursorEl = null;
  let sealDataUrl = "";
  let previousBodyCursor = "";

  function getScale() {
    return BASE_SCALE * currentZoom;
  }

  function updateZoomDisplay() {
    const el = getElement("zoomLevel");
    if (el) el.textContent = Math.round(currentZoom * 100) + "%";
  }

  function applyZoomToPages() {
    const sc = getScale();
    const dpr = win.devicePixelRatio || 1;
    for (let i = 0; i < allPagesData.length; i++) {
      const container = getElement(`page-${i}`);
      if (!container) continue;
      const page = allPagesData[i];
      let pxW, pxH;
      if (page) {
        pxW = Math.round(page.width * sc * dpr) / dpr;
        pxH = Math.round(page.height * sc * dpr) / dpr;
      } else {
        pxW = Math.round(210 * sc * dpr) / dpr;
        pxH = Math.round(297 * sc * dpr) / dpr;
      }
      container.style.width = `${pxW}px`;
      container.style.height = `${pxH}px`;
    }
  }

  function setZoom(zoom) {
    currentZoom = Math.max(ZOOM_MIN, Math.min(ZOOM_MAX, zoom));
    updateZoomDisplay();
    applyZoomToPages();
  }

  function zoomIn() {
    setZoom(currentZoom + ZOOM_STEP);
  }

  function zoomOut() {
    setZoom(currentZoom - ZOOM_STEP);
  }

  function resetZoom() {
    setZoom(1.0);
  }

  function updateStatus(msg) {
    getElement("status").textContent = msg;
  }

  function normalizeSealDataUrl(sealBase64, mimeType = "image/png") {
    if (!sealBase64) return "";
    const trimmed = sealBase64.trim();
    if (trimmed.startsWith("data:")) return trimmed;
    return `data:${mimeType};base64,${trimmed}`;
  }

  function stopSealPlacement() {
    if (!sealPlacementActive) return;
    sealPlacementActive = false;

    doc.removeEventListener("mousemove", handleSealMouseMove, true);
    doc.removeEventListener("click", handleSealPlacementClick, true);

    if (sealCursorEl) {
      sealCursorEl.remove();
      sealCursorEl = null;
    }

    doc.body.style.cursor = previousBodyCursor;
    previousBodyCursor = "";

    const btn = getElement("sealPlaceBtn");
    if (btn) btn.classList.remove("seal-mode-on");
  }

  function startSealPlacement(sealBase64, mimeType = "image/png") {
    if (!allPagesData.length || !parser) {
      alert("请先加载 OFD 文件。");
      return;
    }

    stopSealPlacement();

    sealDataUrl = normalizeSealDataUrl(sealBase64, mimeType);
    if (!sealDataUrl) {
      alert("印章数据无效。");
      return;
    }

    sealPlacementActive = true;

    sealCursorEl = doc.createElement("img");
    sealCursorEl.className = "seal-follow-cursor";
    sealCursorEl.src = sealDataUrl;
    sealCursorEl.alt = "seal-cursor";
    doc.body.appendChild(sealCursorEl);

    previousBodyCursor = doc.body.style.cursor || "";
    doc.body.style.cursor = "crosshair";

    const btn = getElement("sealPlaceBtn");
    if (btn) btn.classList.add("seal-mode-on");

    doc.addEventListener("mousemove", handleSealMouseMove, true);
    doc.addEventListener("click", handleSealPlacementClick, true);
  }

  function handleSealMouseMove(e) {
    if (!sealPlacementActive || !sealCursorEl) return;
    sealCursorEl.style.left = `${e.clientX}px`;
    sealCursorEl.style.top = `${e.clientY}px`;
  }

  function getPageDataByIndex(pageIndex) {
    if (allPagesData[pageIndex]) return allPagesData[pageIndex];
    if (!parser) return null;
    try {
      const pageData = parser.render_page_svg(pageIndex);
      allPagesData[pageIndex] = pageData;
      return pageData;
    } catch (err) {
      console.error("加载页面数据失败:", err);
      return null;
    }
  }

  function handleSealPlacementClick(e) {
    if (!sealPlacementActive) return;
    const target = e.target instanceof Element ? e.target : null;
    if (target?.closest("#sealPlaceBtn")) return;

    e.preventDefault();
    e.stopPropagation();

    const pageContainer = target?.closest(".page-container");
    if (!pageContainer) {
      alert("错误：点击位置不在 OFD 页面内。");
      stopSealPlacement();
      return;
    }

    const rect = pageContainer.getBoundingClientRect();
    const localX = e.clientX - rect.left;
    const localY = e.clientY - rect.top;

    if (
      localX < 0 ||
      localY < 0 ||
      localX > rect.width ||
      localY > rect.height
    ) {
      alert("错误：点击位置不在 OFD 页面范围内。");
      stopSealPlacement();
      return;
    }

    const pageIndex = Number.parseInt(pageContainer.dataset.pageIndex, 10);
    if (Number.isNaN(pageIndex) || pageIndex < 0) {
      alert("错误：无法识别页码。");
      stopSealPlacement();
      return;
    }

    const pageData = getPageDataByIndex(pageIndex);
    if (!pageData || pageData.error || !pageData.width || !pageData.height) {
      alert("错误：无法获取页面尺寸，落章失败。");
      stopSealPlacement();
      return;
    }

    const ofdX = (localX / rect.width) * pageData.width * 0.9;
    const ofdY = (localY / rect.height) * pageData.height * 0.9;

    alert(
      `印章放置成功\n页码: ${pageIndex + 1}\nOFD坐标(mm): X=${ofdX.toFixed(2)}, Y=${ofdY.toFixed(2)}`,
    );

    stopSealPlacement();
  }

  function startSealPlacementWithMock() {
    startSealPlacement(MOCK_SEAL_BASE64, "image/svg+xml");
  }

  function infoItem(label, value) {
    return `<div class="info-item"><span class="info-label">${label}</span><span class="info-value">${value}</span></div>`;
  }

  function displayDocInfo(info) {
    const container = getElement("docInfo");
    let html = "";
    if (info) {
      html += infoItem("标题", info.title || "-");
      html += infoItem("作者", info.author || "-");
      html += infoItem("创建", info.creationDate || "-");
    }
    html += infoItem("页数", currentPageCount);
    container.innerHTML = html;
  }

  function displayPageList(count) {
    const container = getElement("pageList");
    let html = "";
    for (let i = 0; i < count; i++) {
      html += `<div class="page-item" data-page="${i}">第 ${i + 1} 页</div>`;
    }
    container.innerHTML =
      html || '<div style="color:#666;font-size:13px;">无页面</div>';

    container.querySelectorAll(".page-item").forEach((item) => {
      item.addEventListener("click", () => {
        const idx = parseInt(item.dataset.page, 10);
        scrollToPage(idx);
      });
    });
  }

  function scrollToPage(index) {
    const page = getElement(`page-${index}`);
    if (page) {
      page.scrollIntoView({ behavior: "smooth", block: "start" });
      doc.querySelectorAll(".page-item").forEach((item, i) => {
        item.classList.toggle("active", i === index);
      });
    }
  }

  async function loadOFDFonts() {
    try {
      const fonts = parser.get_fonts();
      console.log("[字体] get_fonts 返回:", JSON.stringify(fonts, null, 2));
      if (!fonts || fonts.length === 0) {
        console.warn("[字体] 没有字体信息");
        return;
      }

      // 打印每个字体的关键字段
      for (const f of fonts) {
        console.log(
          `[字体] id=${f.id}, hasFile=${f.hasFile}, dataUrl存在=${!!f.dataUrl}, dataUrl长度=${f.dataUrl ? f.dataUrl.length : 0}`,
        );
      }

      const embeddedFonts = fonts.filter((f) => f.hasFile && f.dataUrl);
      console.log(
        `[字体] 嵌入字体数量: ${embeddedFonts.length} / ${fonts.length}`,
      );

      // 清理旧的字体样式
      let styleEl = getElement("ofd-font-styles");
      if (styleEl) styleEl.remove();
      styleEl = doc.createElement("style");
      styleEl.id = "ofd-font-styles";
      let cssText = "";

      for (const font of embeddedFonts) {
        const fontName = `OFD_Font_${font.id}`;
        if (loadedFonts.has(fontName)) continue;
        try {
          // 通过 CSS @font-face 注册字体
          cssText += `@font-face { font-family: '${fontName}'; src: url(${font.dataUrl}); }\n`;

          // 同时通过 FontFace API 加载
          const fontFace = new FontFaceCtor(fontName, `url(${font.dataUrl})`);
          await fontFace.load();
          doc.fonts.add(fontFace);
          loadedFonts.set(fontName, fontFace);
          console.log(
            `[字体] 已加载: ${fontName} (${font.fontName}/${font.familyName}), status=${fontFace.status}`,
          );
        } catch (err) {
          console.warn(`字体加载失败: ${fontName}`, err);
        }
      }

      if (cssText) {
        styleEl.textContent = cssText;
        doc.head.appendChild(styleEl);
        console.log("[字体] @font-face CSS 已注入到 head");
      }

      // 等待所有字体就绪
      await doc.fonts.ready;
      console.log(`[字体] 所有字体就绪, 已加载 ${loadedFonts.size} 个`);
      // 列出所有已注册的字体
      for (const f of doc.fonts) {
        console.log(`[字体] doc.fonts: ${f.family} status=${f.status}`);
      }
    } catch (err) {
      console.warn("加载字体出错:", err);
    }
  }

  async function parseAndRender(file) {
    stopSealPlacement();

    updateStatus("解析中...");
    const viewer = getElement("viewer");
    const sealBtn = getElement("sealPlaceBtn");
    if (sealBtn) {
      sealBtn.style.display = "none";
      sealBtn.classList.remove("seal-mode-on");
    }
    viewer.innerHTML =
      '<div class="empty-state loading"><p>正在解析文档...</p></div>';

    try {
      const totalStart = performance.now();
      const arrayBuffer = await file.arrayBuffer();
      const data = new Uint8Array(arrayBuffer);

      parser = new OFDParser(data);
      const result = parser.parse();
      if (result.error) throw new Error(result.error);

      currentPageCount = result.pageCount || 0;
      const docInfo = parser.get_doc_info();
      displayDocInfo(docInfo);
      displayPageList(currentPageCount);

      updateStatus("加载字体...");
      await loadOFDFonts();

      updateStatus("渲染页面...");
      await renderAllPages();

      console.log(`[总耗时] ${(performance.now() - totalStart).toFixed(2)}ms`);
      updateStatus(`已加载 ${currentPageCount} 页`);
      getElement("xmlBtn").style.display = "";
      getElement("zoomControls").style.display = "";
      getElement("printBtn").style.display = "";
      if (sealBtn) sealBtn.style.display = "";
    } catch (err) {
      updateStatus("错误: " + err.message);
      viewer.innerHTML = `<div class="empty-state"><p>解析失败</p><p style="font-size:12px;">${err.message}</p></div>`;
      if (sealBtn) sealBtn.style.display = "none";
      console.error(err);
    }
  }

  async function renderAllPages() {
    const viewer = getElement("viewer");
    const pageCount = parser.get_page_count();
    if (pageCount <= 0) {
      viewer.innerHTML =
        '<div class="empty-state"><p>❌ 无法获取页数</p></div>';
      return;
    }

    allPagesData = new Array(pageCount).fill(null);
    renderedPages.clear();
    viewer.innerHTML = "";

    const initialCount = Math.min(INITIAL_PAGES, pageCount);

    for (let i = 0; i < initialCount; i++) {
      const stepStart = performance.now();
      const pageData = parser.render_page_svg(i);
      allPagesData[i] = pageData;
      console.log(
        `[渲染] 页面${i + 1} SVG: ${(performance.now() - stepStart).toFixed(2)}ms`,
      );
    }

    for (let i = 0; i < initialCount; i++) {
      viewer.appendChild(createPageContainer(i));
      await renderPageContent(i);
      await new Promise((r) => setTimeout(r, 0));
    }

    if (pageCount > initialCount) {
      requestAnimationFrame(() => {
        for (let i = initialCount; i < pageCount; i++) {
          viewer.appendChild(createPageContainer(i));
        }
        setupScrollListener();
      });
    }
  }

  function createPageContainer(index) {
    const container = doc.createElement("div");
    container.id = `page-${index}`;
    container.className = "page-container";
    container.dataset.pageIndex = index;

    // 用 devicePixelRatio 对齐物理像素，避免亚像素模糊
    const dpr = win.devicePixelRatio || 1;
    let pxWidth, pxHeight;
    if (allPagesData[index]) {
      pxWidth = Math.round(allPagesData[index].width * getScale() * dpr) / dpr;
      pxHeight =
        Math.round(allPagesData[index].height * getScale() * dpr) / dpr;
    } else {
      pxWidth = Math.round(210 * getScale() * dpr) / dpr;
      pxHeight = Math.round(297 * getScale() * dpr) / dpr;
    }

    container.style.cssText = `
      width: ${pxWidth}px; height: ${pxHeight}px;
      position: relative; background: #f5f5f5;
      box-shadow: 0 2px 10px rgba(0,0,0,0.2);
      border-radius: 4px; overflow: hidden; flex-shrink: 0;
    `;

    if (!allPagesData[index]) {
      const placeholder = doc.createElement("div");
      placeholder.style.cssText =
        "position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);color:#999;font-size:14px;";
      placeholder.textContent = `第 ${index + 1} 页 (滚动加载)`;
      container.appendChild(placeholder);
    }

    return container;
  }

  async function renderPageContent(pageIndex) {
    if (renderedPages.has(pageIndex)) return;
    const page = allPagesData[pageIndex];
    if (!page) return;

    const container = getElement(`page-${pageIndex}`);
    if (!container) return;

    renderedPages.add(pageIndex);

    if (page.error) {
      container.innerHTML = `<div style="color:red;padding:20px;">页面 ${pageIndex + 1} 渲染失败: ${page.error}</div>`;
      return;
    }

    const dpr = win.devicePixelRatio || 1;
    const pxWidth = Math.round(page.width * getScale() * dpr) / dpr;
    const pxHeight = Math.round(page.height * getScale() * dpr) / dpr;

    container.innerHTML = "";
    container.style.background = "#fff";
    container.style.width = `${pxWidth}px`;
    container.style.height = `${pxHeight}px`;

    if (page.stampDebug && page.stampDebug.length > 0) {
      console.groupCollapsed(
        `[StampDebug] 页面${pageIndex + 1}: ${page.stampDebug.length} 条`,
      );
      page.stampDebug.forEach((entry, idx) => {
        console.log(`[StampDebug] #${idx + 1}`, entry);
      });
      console.groupEnd();
    } else {
      console.log(`[StampDebug] 页面${pageIndex + 1}: 无签章调试信息`);
    }

    // SVG 层
    if (page.svg) {
      const svgContainer = doc.createElement("div");
      svgContainer.className = "svg-layer";
      svgContainer.innerHTML = page.svg;
      container.appendChild(svgContainer);

      // 注入字体样式到 SVG 内部，确保 SVG text 元素能使用嵌入字体
      const svgEl = svgContainer.querySelector("svg");
      if (svgEl) {
        if (loadedFonts.size > 0) {
          const existingStyle = getElement("ofd-font-styles");
          if (existingStyle && existingStyle.textContent) {
            const svgStyle = doc.createElementNS(
              "http://www.w3.org/2000/svg",
              "style",
            );
            svgStyle.textContent = existingStyle.textContent;
            svgEl.insertBefore(svgStyle, svgEl.firstChild);
          }
        }

        const paths = svgEl.querySelectorAll("path").length;
        const images = svgEl.querySelectorAll("image").length;
        const texts = svgEl.querySelectorAll("text").length;
        console.log(
          `[SVG] 页面${pageIndex + 1}: paths=${paths}, images=${images}, texts=${texts}`,
        );
        console.log(
          `[SVG尺寸调试] 容器: ${pxWidth}x${pxHeight}, SVG属性: width=${svgEl.getAttribute("width")} height=${svgEl.getAttribute("height")}, SVG实际: ${svgEl.getBoundingClientRect().width}x${svgEl.getBoundingClientRect().height}`,
        );
        // 调试：输出前几个 text 元素的完整属性
        const textEls = svgEl.querySelectorAll("text");
        for (let i = 0; i < Math.min(5, textEls.length); i++) {
          const t = textEls[i];
          console.log(
            `[SVG文字调试] text[${i}]: font-size="${t.getAttribute("font-size")}", data-mm-size="${t.getAttribute("data-mm-size")}", stroke="${t.getAttribute("stroke")}", stroke-width="${t.getAttribute("stroke-width")}", fill="${t.getAttribute("fill")}", content="${t.textContent}", parent=<${t.parentElement.tagName} transform="${t.parentElement.getAttribute("transform")}">`,
          );
        }
      }
    }

    // 文本蒙层（用于浏览器搜索 Ctrl+F）
    if (page.textOverlay && page.textOverlay.length > 0) {
      const hiDpi = page.hiDpi || 1;
      const textLayer = doc.createElement("div");
      textLayer.className = "text-overlay";
      for (const item of page.textOverlay) {
        const span = doc.createElement("span");
        span.textContent = item.text;
        const ox = item.x / hiDpi;
        const oy = item.y / hiDpi;
        const ow = item.width / hiDpi;
        const oh = item.height / hiDpi;
        span.style.cssText = `
          position: absolute;
          left: ${ox}px; top: ${oy}px;
          width: ${ow}px; height: ${oh}px;
          font-size: ${oh * 0.8}px;
          color: transparent; white-space: nowrap; overflow: hidden;
          line-height: ${oh}px;
          pointer-events: auto; user-select: text; -webkit-user-select: text;
        `;
        textLayer.appendChild(span);
      }
      container.appendChild(textLayer);
    }
  }

  function setupScrollListener() {
    const viewer = getElement("viewer");
    let initialized = false;
    requestAnimationFrame(() => {
      initialized = true;
    });

    const pendingPages = new Set();
    let isProcessing = false;

    async function processQueue() {
      if (isProcessing || pendingPages.size === 0) return;
      isProcessing = true;
      while (pendingPages.size > 0) {
        const pageIndex = Math.min(...pendingPages);
        pendingPages.delete(pageIndex);
        if (!renderedPages.has(pageIndex)) {
          await loadAndRenderPage(pageIndex);
          await new Promise((r) => setTimeout(r, 0));
        }
      }
      isProcessing = false;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        if (!initialized) return;
        let hasNew = false;
        for (const entry of entries) {
          if (entry.isIntersecting) {
            const pageIndex = parseInt(entry.target.dataset.pageIndex, 10);
            if (!isNaN(pageIndex) && !renderedPages.has(pageIndex)) {
              pendingPages.add(pageIndex);
              hasNew = true;
            }
          }
        }
        if (hasNew) processQueue();
      },
      { root: viewer, rootMargin: `${PRELOAD_THRESHOLD}px`, threshold: 0 },
    );

    doc.querySelectorAll(".page-container").forEach((container) => {
      const idx = parseInt(container.dataset.pageIndex, 10);
      if (!renderedPages.has(idx)) observer.observe(container);
    });
  }

  async function loadAndRenderPage(pageIndex) {
    if (renderedPages.has(pageIndex)) return;
    if (allPagesData[pageIndex]) {
      await renderPageContent(pageIndex);
      return;
    }
    try {
      const pageData = parser.render_page_svg(pageIndex);
      allPagesData[pageIndex] = pageData;
      await renderPageContent(pageIndex);
    } catch (err) {
      console.error(`加载页面 ${pageIndex + 1} 失败:`, err);
    }
  }

  // XML 弹窗逻辑

  function getFileIcon(name) {
    const ext = name.split(".").pop().toLowerCase();
    if (ext === "xml") return "📄";
    if (["ttf", "otf", "ttc", "woff", "woff2"].includes(ext)) return "🔤";
    if (["png", "jpg", "jpeg", "gif", "bmp", "svg", "jb2"].includes(ext))
      return "🖼️";
    if (["dat", "esl", "seal"].includes(ext)) return "🔏";
    return "📎";
  }

  function buildFileTree(files) {
    const root = new Map(); // key -> { children: Map, files: [] }
    for (const f of files) {
      const parts = f.split("/");
      let node = root;
      for (let i = 0; i < parts.length - 1; i++) {
        const seg = parts[i];
        if (!node.has(seg)) node.set(seg, { children: new Map(), files: [] });
        node = node.get(seg).children;
      }
      const fileName = parts[parts.length - 1];
      // 如果只有文件名没有目录，放到根
      if (parts.length === 1) {
        if (!node.has("__files__"))
          node.set("__files__", { children: new Map(), files: [] });
        node.get("__files__").files.push({ fullPath: f, fileName });
      } else {
        const parentSeg = parts[parts.length - 2];
        // 回溯找到父节点
        let parent = root;
        for (let i = 0; i < parts.length - 2; i++)
          parent = parent.get(parts[i]).children;
        parent.get(parentSeg).files.push({ fullPath: f, fileName });
      }
    }
    return root;
  }

  function countTreeFiles(nodeMap) {
    let count = 0;
    for (const [key, val] of nodeMap) {
      if (key === "__files__") {
        count += val.files.length;
        continue;
      }
      count += val.files.length + countTreeFiles(val.children);
    }
    return count;
  }

  function renderTreeNode(nodeMap, depth) {
    let html = "";
    // 先渲染根级散落文件
    if (nodeMap.has("__files__")) {
      for (const item of nodeMap.get("__files__").files) {
        const isXml = item.fullPath.toLowerCase().endsWith(".xml");
        html += `<div class="tree-file" data-file="${item.fullPath}" data-xml="${isXml}" title="${item.fullPath}" style="padding-left:${depth * 16 + 12}px">
          <span class="tree-file-icon">${getFileIcon(item.fileName)}</span>
          <span class="tree-file-name">${item.fileName}</span>
        </div>`;
      }
    }
    for (const [key, val] of nodeMap) {
      if (key === "__files__") continue;
      const subCount = val.files.length + countTreeFiles(val.children);
      html += `<div class="tree-folder">
        <div class="tree-folder-header" style="padding-left:${depth * 16 + 4}px">
          <span class="tree-folder-arrow">▶</span>
          <span class="tree-folder-icon">📁</span>
          <span class="tree-folder-name">${key}</span>
          <span class="tree-folder-count">${subCount}</span>
        </div>
        <div class="tree-folder-children">`;
      // 子文件夹
      html += renderTreeNode(val.children, depth + 1);
      // 当前文件夹的文件
      for (const item of val.files) {
        const isXml = item.fullPath.toLowerCase().endsWith(".xml");
        html += `<div class="tree-file" data-file="${item.fullPath}" data-xml="${isXml}" title="${item.fullPath}" style="padding-left:${(depth + 1) * 16 + 12}px">
          <span class="tree-file-icon">${getFileIcon(item.fileName)}</span>
          <span class="tree-file-name">${item.fileName}</span>
        </div>`;
      }
      html += `</div></div>`;
    }
    return html;
  }

  function showXmlModal() {
    if (!parser) return;
    const modal = getElement("xmlModal");
    const fileList = getElement("xmlFileList");
    const files = parser.get_files();

    const tree = buildFileTree(files);
    fileList.innerHTML = renderTreeNode(tree, 0);

    // 文件夹折叠/展开
    fileList.querySelectorAll(".tree-folder-header").forEach((header) => {
      header.addEventListener("click", () => {
        header.parentElement.classList.toggle("collapsed");
      });
    });

    // 文件点击
    fileList.querySelectorAll(".tree-file").forEach((item) => {
      item.addEventListener("click", () => {
        fileList
          .querySelectorAll(".tree-file")
          .forEach((el) => el.classList.remove("active"));
        item.classList.add("active");
        const name = item.dataset.file;
        const isXml = item.dataset.xml === "true";
        getElement("xmlFileName").textContent = name;
        if (isXml) {
          const content = parser.read_file_text(name);
          getElement("xmlContent").innerHTML = highlightXml(formatXml(content));
        } else {
          getElement("xmlContent").innerHTML =
            '<span style="color:#888;font-size:14px;">二进制文件，无法预览</span>';
        }
      });
    });

    // 默认选中第一个 XML 文件
    const firstXml = fileList.querySelector('.tree-file[data-xml="true"]');
    if (firstXml) firstXml.click();

    modal.classList.add("active");
  }

  function hideXmlModal() {
    getElement("xmlModal").classList.remove("active");
  }

  function formatXml(xml) {
    let formatted = "";
    let indent = 0;
    const parts = xml.replace(/(>)\s*(<)/g, "$1\n$2").split("\n");
    for (const part of parts) {
      const trimmed = part.trim();
      if (!trimmed) continue;
      if (trimmed.startsWith("</")) indent = Math.max(indent - 1, 0);
      formatted += "  ".repeat(indent) + trimmed + "\n";
      if (
        trimmed.startsWith("<") &&
        !trimmed.startsWith("</") &&
        !trimmed.startsWith("<?") &&
        !trimmed.endsWith("/>") &&
        !trimmed.includes("</")
      ) {
        indent++;
      }
    }
    return formatted.trim();
  }

  function escHtml(s) {
    return s
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function highlightXml(xml) {
    const lines = xml.split("\n");
    let result = "";
    for (let i = 0; i < lines.length; i++) {
      const line = escHtml(lines[i]);
      let h = line;

      // XML 声明
      h = h.replace(
        /^(\s*)(&lt;\?)(.*?)(\?&gt;)$/,
        '$1<span class="xml-bracket">$2</span><span class="xml-decl">$3</span><span class="xml-bracket">$4</span>',
      );

      // 标签名
      h = h.replace(
        /(&lt;\/?)([A-Za-z_][\w:.-]*)/g,
        '<span class="xml-bracket">$1</span><span class="xml-tag">$2</span>',
      );

      // 闭合括号
      h = h.replace(/(\/?)(&gt;)/g, '<span class="xml-bracket">$1$2</span>');

      // 属性
      h = h.replace(
        /\b([A-Za-z_][\w:.-]*)=(&quot;)(.*?)(&quot;)/g,
        '<span class="xml-attr">$1</span>=<span class="xml-value">$2$3$4</span>',
      );

      // 标签间的文本内容（非空白）
      h = h.replace(
        /(<\/span>)([^<]+)(<span class="xml-bracket">)/g,
        (_, before, text, after) => {
          if (text.trim())
            return `${before}<span class="xml-text">${text}</span>${after}`;
          return `${before}${text}${after}`;
        },
      );

      result += `<span class="xml-line"><span class="xml-linenum">${String(i + 1).padStart(4)}</span>${h}</span>\n`;
    }
    return result;
  }

  /**
   * 打印 OFD 文档
   * 收集所有已渲染的 SVG 页面，在新窗口中打开并触发浏览器打印对话框
   */
  function printOFD() {
    if (!parser || !allPagesData.length) {
      alert("请先加载 OFD 文件。");
      return;
    }

    updateStatus("🖨️ 准备打印...");

    // 确保所有页面都已渲染数据
    const pageCount = parser.get_page_count();
    for (let i = 0; i < pageCount; i++) {
      if (!allPagesData[i]) {
        try {
          allPagesData[i] = parser.render_page_svg(i);
        } catch (err) {
          console.error(`打印：加载页面 ${i + 1} 失败:`, err);
        }
      }
    }

    // 收集嵌入字体的 @font-face CSS
    let fontCss = "";
    const fontStyleEl = getElement("ofd-font-styles");
    if (fontStyleEl && fontStyleEl.textContent) {
      fontCss = fontStyleEl.textContent;
    }

    // 构建打印页面的 HTML
    let pagesHtml = "";
    for (let i = 0; i < pageCount; i++) {
      const page = allPagesData[i];
      if (!page || page.error || !page.svg) continue;

      // 计算页面宽高（mm），用于打印时的精确定位
      const widthMM = page.width;
      const heightMM = page.height;

      pagesHtml += `
        <div class="print-page" style="width: ${widthMM}mm; height: ${heightMM}mm;">
          ${page.svg}
        </div>
      `;
    }

    if (!pagesHtml) {
      alert("没有可打印的页面。");
      updateStatus(`已加载 ${currentPageCount} 页`);
      return;
    }

    const printHtml = `<!DOCTYPE html>
  <html lang="zh-CN">
  <head>
    <meta charset="UTF-8">
    <title>OFD 打印</title>
    <style>
      ${fontCss}
  
      * { margin: 0; padding: 0; box-sizing: border-box; }
  
      body {
        background: #fff;
      }
  
      .print-page {
        page-break-after: always;
        page-break-inside: avoid;
        position: relative;
        overflow: hidden;
        margin: 0 auto;
      }
  
      .print-page:last-child {
        page-break-after: auto;
      }
  
      .print-page svg {
        display: block;
        width: 100%;
        height: 100%;
      }
  
      /* 屏幕预览样式 */
      @media screen {
        body {
          background: #e0e0e0;
          padding: 20px;
        }
        .print-page {
          background: #fff;
          box-shadow: 0 2px 12px rgba(0,0,0,0.2);
          margin-bottom: 20px;
        }
      }
  
      /* 打印样式 */
      @media print {
        body { background: #fff; padding: 0; margin: 0; }
        .print-page {
          box-shadow: none;
          margin: 0;
        }
  
        @page {
          margin: 0;
        }
      }
    </style>
  </head>
  <body>
    ${pagesHtml}
    <script>
      // 页面加载完成后自动弹出打印对话框
      win.addEventListener('load', function() {
        // 等待字体和 SVG 渲染就绪
        setTimeout(function() {
          win.print();
        }, 500);
      });
    <\/script>
  </body>
  </html>`;

    // 在新窗口中打开打印页面
    const printWindow = win.open("", "_blank");
    if (!printWindow) {
      alert("无法打开打印窗口，请检查浏览器是否拦截了弹出窗口。");
      updateStatus(`已加载 ${currentPageCount} 页`);
      return;
    }

    printWindow.doc.open();
    printWindow.doc.write(printHtml);
    printWindow.doc.close();

    updateStatus(`已加载 ${currentPageCount} 页`);
  }

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

  function destroy() {
    stopSealPlacement();
    const fileInput = getElement("fileInput");
    if (fileInput)
      fileInput.removeEventListener("change", handleFileInputChange);
    doc.removeEventListener("keydown", handleKeyDown);
    doc.body.removeEventListener("dragover", handleDragOver);
    doc.body.removeEventListener("drop", handleDrop);
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
    bindDomEvents,
    destroy,
  };
}

export async function initOFDViewer(options = {}) {
  await __wbg_init(options.wasmModuleOrPath);
  const viewer = createOFDViewer(options);
  viewer.bindDomEvents();
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

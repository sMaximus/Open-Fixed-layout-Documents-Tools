(function (global, factory) {
  typeof exports === 'object' && typeof module !== 'undefined' ? factory(exports) :
  typeof define === 'function' && define.amd ? define(['exports'], factory) :
  (global = typeof globalThis !== 'undefined' ? globalThis : global || self, factory(global.OFDViewerLib = {}));
})(this, (function (exports) { 'use strict';

  var componentCSS = "/* OFD Viewer 组件样式 */\n.ofd-viewer {\n  position: relative;\n  width: 100%;\n  height: 100%;\n  overflow: auto;\n  display: flex;\n  flex-direction: column;\n  align-items: center;\n  gap: 20px;\n  padding: 20px;\n  box-sizing: border-box;\n}\n.ofd-viewer .ofd-page {\n  flex-shrink: 0;\n  position: relative;\n  background: #fff;\n  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.2);\n  border-radius: 4px;\n  overflow: hidden;\n}\n.ofd-viewer .ofd-svg-layer {\n  position: absolute;\n  left: 0;\n  top: 0;\n  width: 100%;\n  height: 100%;\n  z-index: 0;\n}\n.ofd-viewer .ofd-svg-layer svg {\n  display: block;\n  width: 100%;\n  height: 100%;\n}\n.ofd-viewer .ofd-text-overlay {\n  position: absolute;\n  left: 0;\n  top: 0;\n  width: 100%;\n  height: 100%;\n  z-index: 1;\n  overflow: hidden;\n  pointer-events: none;\n}\n.ofd-viewer .ofd-text-overlay span {\n  pointer-events: auto;\n}\n.ofd-viewer .ofd-text-overlay span::selection {\n  background: rgba(0, 100, 255, 0.3);\n}\n";

  /**
   * OFDViewer — 独立的 OFD 文档查看器组件
   *
   * 用法 (ESM):
   *   import initWasm, { OFDParser } from './pkg/ofd_rust.js';
   *   import { OFDViewer } from './dist/ofd-viewer.esm.js';
   *
   *   const viewer = new OFDViewer({ container: document.getElementById('viewer') });
   *   await viewer.init(initWasm, OFDParser);
   *   await viewer.loadFile(file);
   *
   * 用法 (UMD / script 标签):
   *   <script type="module">
   *     import initWasm, { OFDParser } from './pkg/ofd_rust.js';
   *     const { OFDViewer } = OFDViewerLib;
   *     const viewer = new OFDViewer({ container: document.getElementById('viewer') });
   *     await viewer.init(initWasm, OFDParser);
   *     await viewer.loadFile(file);
   *   </script>
   */


  const SCALE = 3.78;
  const INITIAL_PAGES = 3;
  const PRELOAD_THRESHOLD = 200;

  class OFDViewer {
    /**
     * @param {Object} options
     * @param {HTMLElement} options.container - 渲染容器
     * @param {number} [options.scale=3.78] - mm 到 px 的缩放比
     * @param {number} [options.initialPages=3] - 首次渲染页数
     * @param {Function} [options.onStatus] - 状态回调
     * @param {Function} [options.onDocInfo] - 文档信息回调
     * @param {Function} [options.onPageCount] - 页数回调
     * @param {Function} [options.onError] - 错误回调
     */
    constructor(options = {}) {
      this.container = options.container;
      this.scale = options.scale || SCALE;
      this.initialPages = options.initialPages || INITIAL_PAGES;
      this.onStatus = options.onStatus || (() => {});
      this.onDocInfo = options.onDocInfo || (() => {});
      this.onPageCount = options.onPageCount || (() => {});
      this.onError = options.onError || (() => {});

      this._OFDParser = null;
      this._parser = null;
      this._allPagesData = [];
      this._renderedPages = new Set();
      this._loadedFonts = new Map();
      this._pageCount = 0;
      this._ready = false;

      this._injectStyles();
      if (this.container && !this.container.classList.contains("ofd-viewer")) {
        this.container.classList.add("ofd-viewer");
      }
    }

    _injectStyles() {
      if (document.getElementById("ofd-viewer-component-styles")) return;
      const style = document.createElement("style");
      style.id = "ofd-viewer-component-styles";
      style.textContent = componentCSS;
      document.head.appendChild(style);
    }

    /**
     * 初始化 WASM
     * @param {Function} initWasm - wasm-pack 生成的 default export (init 函数)
     * @param {Function} ParserClass - OFDParser 类
     * @param {string|URL} [wasmUrl] - 可选的 .wasm 文件路径
     */
    async init(initWasm, ParserClass, wasmUrl) {
      if (this._ready) return;
      await initWasm(wasmUrl);
      this._OFDParser = ParserClass;
      this._ready = true;
      this.onStatus("就绪");
    }

    /**
     * 加载并渲染 OFD 文件
     * @param {File|ArrayBuffer|Uint8Array} source
     */
    async loadFile(source) {
      if (!this._ready) throw new Error("请先调用 init()");
      this.onStatus("解析中...");
      this.container.innerHTML = "";

      try {
        const t0 = performance.now();
        let data;
        if (source instanceof File) {
          data = new Uint8Array(await source.arrayBuffer());
        } else if (source instanceof ArrayBuffer) {
          data = new Uint8Array(source);
        } else if (source instanceof Uint8Array) {
          data = source;
        } else {
          throw new Error(
            "不支持的数据类型，需要 File / ArrayBuffer / Uint8Array",
          );
        }

        this._parser = new this._OFDParser(data);
        const result = this._parser.parse();
        if (result.error) throw new Error(result.error);

        this._pageCount = result.pageCount || 0;
        this.onDocInfo(this._parser.get_doc_info());
        this.onPageCount(this._pageCount);

        this.onStatus("加载字体...");
        await this._loadFonts();

        this.onStatus("渲染页面...");
        await this._renderAllPages();

        const ms = (performance.now() - t0).toFixed(0);
        this.onStatus(`已加载 ${this._pageCount} 页 (${ms}ms)`);
      } catch (err) {
        this.onError(err);
        this.onStatus("解析失败: " + err.message);
        throw err;
      }
    }

    get pageCount() {
      return this._pageCount;
    }
    get parser() {
      return this._parser;
    }

    getFonts() {
      return this._parser ? this._parser.get_fonts() : [];
    }
    getFiles() {
      return this._parser ? this._parser.get_files() : [];
    }
    readFileText(name) {
      return this._parser ? this._parser.read_file_text(name) : "";
    }

    scrollToPage(index) {
      const el = this.container.querySelector(`[data-page-index="${index}"]`);
      if (el) el.scrollIntoView({ behavior: "smooth", block: "start" });
    }

    destroy() {
      this.container.innerHTML = "";
      this._allPagesData = [];
      this._renderedPages.clear();
      if (this._parser) {
        this._parser.free();
        this._parser = null;
      }
      this._pageCount = 0;
    }

    // ---- 内部方法 ----

    async _loadFonts() {
      try {
        const fonts = this._parser.get_fonts();
        if (!fonts || fonts.length === 0) return;
        const embedded = fonts.filter((f) => f.hasFile && f.dataUrl);
        if (embedded.length === 0) return;

        let styleEl = document.getElementById("ofd-font-styles");
        if (styleEl) styleEl.remove();
        styleEl = document.createElement("style");
        styleEl.id = "ofd-font-styles";
        let css = "";

        for (const font of embedded) {
          const name = `OFD_Font_${font.id}`;
          if (this._loadedFonts.has(name)) continue;
          try {
            css += `@font-face { font-family: '${name}'; src: url(${font.dataUrl}); }\n`;
            const face = new FontFace(name, `url(${font.dataUrl})`);
            await face.load();
            document.fonts.add(face);
            this._loadedFonts.set(name, face);
          } catch (e) {
            console.warn(`字体加载失败: ${name}`, e);
          }
        }

        if (css) {
          styleEl.textContent = css;
          document.head.appendChild(styleEl);
        }
        await document.fonts.ready;
      } catch (e) {
        console.warn("加载字体出错:", e);
      }
    }

    async _renderAllPages() {
      const count = this._parser.get_page_count();
      if (count <= 0) return;

      this._allPagesData = new Array(count).fill(null);
      this._renderedPages.clear();
      this.container.innerHTML = "";

      const initial = Math.min(this.initialPages, count);
      for (let i = 0; i < initial; i++) {
        this._allPagesData[i] = this._parser.render_page_svg(i);
      }
      for (let i = 0; i < initial; i++) {
        this.container.appendChild(this._createPage(i));
        await this._renderPage(i);
        await new Promise((r) => setTimeout(r, 0));
      }

      if (count > initial) {
        requestAnimationFrame(() => {
          for (let i = initial; i < count; i++) {
            this.container.appendChild(this._createPage(i));
          }
          this._setupLazyLoad();
        });
      }
    }

    _createPage(index) {
      const el = document.createElement("div");
      el.className = "ofd-page";
      el.dataset.pageIndex = index;
      const pd = this._allPagesData[index];
      const w = pd ? pd.width * this.scale : 210 * this.scale;
      const h = pd ? pd.height * this.scale : 297 * this.scale;
      el.style.cssText = `width:${w}px;height:${h}px;position:relative;background:#f5f5f5;`;
      if (!pd) {
        const ph = document.createElement("div");
        ph.style.cssText =
          "position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);color:#999;font-size:14px;";
        ph.textContent = `第 ${index + 1} 页`;
        el.appendChild(ph);
      }
      return el;
    }

    async _renderPage(index) {
      if (this._renderedPages.has(index)) return;
      const page = this._allPagesData[index];
      if (!page) return;
      const el = this.container.querySelector(`[data-page-index="${index}"]`);
      if (!el) return;
      this._renderedPages.add(index);

      if (page.error) {
        el.innerHTML = `<div style="color:red;padding:20px;">页面 ${index + 1} 渲染失败: ${page.error}</div>`;
        return;
      }

      const w = page.width * this.scale;
      const h = page.height * this.scale;
      el.innerHTML = "";
      el.style.cssText = `width:${w}px;height:${h}px;position:relative;background:#fff;box-shadow:0 2px 10px rgba(0,0,0,0.2);border-radius:4px;overflow:hidden;`;

      if (page.svg) {
        const svgDiv = document.createElement("div");
        svgDiv.className = "ofd-svg-layer";
        svgDiv.innerHTML = page.svg;
        el.appendChild(svgDiv);

        const svgEl = svgDiv.querySelector("svg");
        if (svgEl && this._loadedFonts.size > 0) {
          const fontStyle = document.getElementById("ofd-font-styles");
          if (fontStyle && fontStyle.textContent) {
            const s = document.createElementNS(
              "http://www.w3.org/2000/svg",
              "style",
            );
            s.textContent = fontStyle.textContent;
            svgEl.insertBefore(s, svgEl.firstChild);
          }
        }
      }

      if (page.textOverlay && page.textOverlay.length > 0) {
        const layer = document.createElement("div");
        layer.className = "ofd-text-overlay";
        for (const item of page.textOverlay) {
          const span = document.createElement("span");
          span.textContent = item.text;
          span.style.cssText = `position:absolute;left:${item.x}px;top:${item.y}px;width:${item.width}px;height:${item.height}px;font-size:${item.height * 0.8}px;color:transparent;white-space:nowrap;overflow:hidden;line-height:${item.height}px;pointer-events:auto;user-select:text;-webkit-user-select:text;`;
          layer.appendChild(span);
        }
        el.appendChild(layer);
      }
    }

    _setupLazyLoad() {
      const pending = new Set();
      let processing = false;
      const process = async () => {
        if (processing || pending.size === 0) return;
        processing = true;
        while (pending.size > 0) {
          const idx = Math.min(...pending);
          pending.delete(idx);
          if (!this._renderedPages.has(idx)) {
            await this._loadAndRender(idx);
            await new Promise((r) => setTimeout(r, 0));
          }
        }
        processing = false;
      };

      const observer = new IntersectionObserver(
        (entries) => {
          let hasNew = false;
          for (const entry of entries) {
            if (entry.isIntersecting) {
              const idx = parseInt(entry.target.dataset.pageIndex, 10);
              if (!isNaN(idx) && !this._renderedPages.has(idx)) {
                pending.add(idx);
                hasNew = true;
              }
            }
          }
          if (hasNew) process();
        },
        {
          root: this.container,
          rootMargin: `${PRELOAD_THRESHOLD}px`,
          threshold: 0,
        },
      );

      this.container.querySelectorAll(".ofd-page").forEach((el) => {
        const idx = parseInt(el.dataset.pageIndex, 10);
        if (!this._renderedPages.has(idx)) observer.observe(el);
      });
    }

    async _loadAndRender(index) {
      if (this._renderedPages.has(index)) return;
      if (this._allPagesData[index]) {
        await this._renderPage(index);
        return;
      }
      try {
        this._allPagesData[index] = this._parser.render_page_svg(index);
        await this._renderPage(index);
      } catch (e) {
        console.error(`加载页面 ${index + 1} 失败:`, e);
      }
    }
  }

  exports.OFDViewer = OFDViewer;

}));
//# sourceMappingURL=ofd-viewer.umd.js.map

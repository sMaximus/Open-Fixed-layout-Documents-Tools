/**
 * OFD Viewer 组件
 * 纯净的 OFD 文档渲染组件，只负责解析和渲染 OFD 文件
 *
 * 使用方法：
 * const viewer = new OFDViewer(containerElement);
 * await viewer.init();
 * await viewer.loadFile(file);  // File 对象
 * // 或
 * await viewer.loadArrayBuffer(arrayBuffer);  // ArrayBuffer
 */

class OFDViewer {
  constructor(container, options = {}) {
    this.container =
      typeof container === "string"
        ? document.querySelector(container)
        : container;

    this.options = {
      scale: options.scale || 3.78, // mm to px
      initialPages: options.initialPages || 3, // 首次渲染页数
      preloadThreshold: options.preloadThreshold || 200, // 预加载阈值(px)
      wasmPath: options.wasmPath || "ofd.wasm",
      ...options,
    };

    this.wasmReady = false;
    this.pageCount = 0;
    this.allPagesData = [];
    this.renderedPages = new Set();
    this.loadedFonts = new Map();
    this.observer = null;

    // 事件回调
    this.onReady = options.onReady || (() => {});
    this.onLoad = options.onLoad || (() => {});
    this.onError = options.onError || (() => {});
    this.onPageRender = options.onPageRender || (() => {});
  }

  /**
   * 初始化 WASM
   */
  async init() {
    try {
      const go = new Go();
      const result = await WebAssembly.instantiateStreaming(
        fetch(this.options.wasmPath),
        go.importObject,
      );
      go.run(result.instance);
      this.wasmReady = true;
      this.onReady();
      return true;
    } catch (err) {
      this.onError(err);
      throw err;
    }
  }

  /**
   * 加载 File 对象
   */
  async loadFile(file) {
    const arrayBuffer = await file.arrayBuffer();
    return this.loadArrayBuffer(arrayBuffer);
  }

  /**
   * 加载 ArrayBuffer
   */
  async loadArrayBuffer(arrayBuffer) {
    if (!this.wasmReady) {
      throw new Error("WASM 未初始化，请先调用 init()");
    }

    try {
      // 解析 OFD
      const uint8Array = new Uint8Array(arrayBuffer);
      const resultJson = ofdParseFile(uint8Array);
      const result = JSON.parse(resultJson);

      if (result.error) {
        throw new Error(result.error);
      }

      this.pageCount = result.PageCount || 0;

      // 加载字体
      await this._loadFonts();

      // 渲染页面
      await this._renderAllPages();

      this.onLoad({ pageCount: this.pageCount, docInfo: result });
      return result;
    } catch (err) {
      this.onError(err);
      throw err;
    }
  }

  /**
   * 获取页数
   */
  getPageCount() {
    return this.pageCount;
  }

  /**
   * 滚动到指定页
   */
  scrollToPage(index) {
    const page = this.container.querySelector(`[data-page-index="${index}"]`);
    if (page) {
      page.scrollIntoView({ behavior: "smooth", block: "start" });
    }
  }

  /**
   * 销毁组件
   */
  destroy() {
    if (this.observer) {
      this.observer.disconnect();
      this.observer = null;
    }
    this.container.innerHTML = "";
    this.allPagesData = [];
    this.renderedPages.clear();
  }

  // ============ 私有方法 ============

  async _loadFonts() {
    try {
      const fontsJson = ofdGetFonts();
      const fonts = JSON.parse(fontsJson);
      if (fonts.error) return;

      const embeddedFonts = fonts.filter((f) => f.hasFile && f.dataURL);
      for (const font of embeddedFonts) {
        const fontName = `OFD_Font_${font.id}`;
        if (this.loadedFonts.has(fontName)) continue;

        try {
          const fontFace = new FontFace(fontName, `url(${font.dataURL})`);
          await fontFace.load();
          document.fonts.add(fontFace);
          this.loadedFonts.set(fontName, fontFace);
        } catch (err) {
          console.warn(`字体加载失败: ${fontName}`, err);
        }
      }
    } catch (err) {
      console.warn("加载字体出错:", err);
    }
  }

  async _renderAllPages() {
    const pageCount = ofdGetPageCount();
    if (pageCount <= 0) return;

    this.allPagesData = new Array(pageCount).fill(null);
    this.renderedPages.clear();
    this.container.innerHTML = "";

    // 滚动到顶部
    this.container.scrollTop = 0;

    // 解析前几页
    const initialCount = Math.min(this.options.initialPages, pageCount);
    for (let i = 0; i < initialCount; i++) {
      const pageJson = ofdRenderPage(i);
      this.allPagesData[i] = JSON.parse(pageJson);
    }

    // 创建所有页面容器
    for (let i = 0; i < pageCount; i++) {
      const pageContainer = this._createPageContainer(i);
      this.container.appendChild(pageContainer);
    }

    // 渲染前几页
    for (let i = 0; i < initialCount; i++) {
      await this._renderPageContent(i);
    }

    // 设置懒加载
    this._setupLazyLoad();
  }

  _createPageContainer(index) {
    const scale = this.options.scale;
    let pxWidth, pxHeight;

    if (this.allPagesData[index]) {
      pxWidth = this.allPagesData[index].width * scale;
      pxHeight = this.allPagesData[index].height * scale;
    } else {
      // 未解析的页面，先获取尺寸信息
      // 尝试从第一页获取尺寸作为默认值
      if (this.allPagesData[0]) {
        pxWidth = this.allPagesData[0].width * scale;
        pxHeight = this.allPagesData[0].height * scale;
      } else {
        pxWidth = 210 * scale;
        pxHeight = 297 * scale;
      }
    }

    const container = document.createElement("div");
    container.className = "ofd-page";
    container.dataset.pageIndex = index;
    // 使用 !important 防止被外部样式覆盖
    container.style.setProperty("width", `${pxWidth}px`, "important");
    container.style.setProperty("height", `${pxHeight}px`, "important");
    container.style.setProperty("min-height", `${pxHeight}px`, "important");
    container.style.setProperty("position", "relative");
    container.style.setProperty("background", "#fff");
    container.style.setProperty("box-shadow", "0 2px 10px rgba(0,0,0,0.2)");
    container.style.setProperty("margin-bottom", "20px");
    container.style.setProperty("overflow", "hidden");
    container.style.setProperty("flex-shrink", "0");

    if (!this.allPagesData[index]) {
      const placeholder = document.createElement("div");
      placeholder.className = "ofd-page-placeholder";
      placeholder.style.cssText = `
        position: absolute;
        top: 50%; left: 50%;
        transform: translate(-50%, -50%);
        color: #999;
      `;
      placeholder.textContent = `第 ${index + 1} 页`;
      container.appendChild(placeholder);
    }

    return container;
  }

  async _renderPageContent(pageIndex) {
    if (this.renderedPages.has(pageIndex)) return;

    const page = this.allPagesData[pageIndex];
    if (!page) return;

    const container = this.container.querySelector(
      `[data-page-index="${pageIndex}"]`,
    );
    if (!container) return;

    this.renderedPages.add(pageIndex);

    if (page.error) {
      container.innerHTML = `<div style="color:red;padding:20px;">页面渲染失败: ${page.error}</div>`;
      return;
    }

    const scale = this.options.scale;
    const pxWidth = page.width * scale;
    const pxHeight = page.height * scale;

    container.innerHTML = "";
    container.style.setProperty("width", `${pxWidth}px`, "important");
    container.style.setProperty("height", `${pxHeight}px`, "important");
    container.style.setProperty("min-height", `${pxHeight}px`, "important");

    // Canvas 层
    const canvas = document.createElement("canvas");
    canvas.width = pxWidth;
    canvas.height = pxHeight;
    canvas.style.cssText = "position:absolute;left:0;top:0;z-index:0;";
    container.appendChild(canvas);

    // 文本层
    const textLayer = document.createElement("div");
    textLayer.className = "ofd-text-layer";
    textLayer.style.cssText = `
      position:absolute;left:0;top:0;
      width:100%;height:100%;
      z-index:1;overflow:hidden;pointer-events:auto;
    `;
    container.appendChild(textLayer);

    // 渲染
    await this._renderCanvas(canvas, page.canvasData, page.textLayer);
    this._renderTextLayer(textLayer, page.textLayer);

    this.onPageRender(pageIndex);
  }

  _setupLazyLoad() {
    if (this.observer) {
      this.observer.disconnect();
    }

    // 判断滚动容器：如果 container 有 overflow:auto/scroll，用它作为 root
    // 否则用 null（viewport）
    const style = getComputedStyle(this.container);
    const isScrollContainer =
      style.overflow === "auto" ||
      style.overflow === "scroll" ||
      style.overflowY === "auto" ||
      style.overflowY === "scroll";

    this.observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            const index = parseInt(entry.target.dataset.pageIndex, 10);
            if (!isNaN(index) && !this.renderedPages.has(index)) {
              this._loadAndRenderPage(index);
            }
          }
        });
      },
      {
        root: isScrollContainer ? this.container : null,
        rootMargin: `${this.options.preloadThreshold}px`,
        threshold: 0,
      },
    );

    this.container.querySelectorAll(".ofd-page").forEach((page) => {
      this.observer.observe(page);
    });
  }

  async _loadAndRenderPage(pageIndex) {
    if (this.renderedPages.has(pageIndex)) return;

    if (!this.allPagesData[pageIndex]) {
      const pageJson = ofdRenderPage(pageIndex);
      this.allPagesData[pageIndex] = JSON.parse(pageJson);
    }

    await this._renderPageContent(pageIndex);
  }

  // ============ Canvas 渲染 ============

  async _renderCanvas(canvas, canvasData, textLayer) {
    const ctx = canvas.getContext("2d");
    ctx.fillStyle = "#fff";
    ctx.fillRect(0, 0, canvas.width, canvas.height);

    if (!canvasData) canvasData = {};

    // 渲染路径
    if (canvasData.paths) {
      for (const pathData of canvasData.paths) {
        await this._drawPath(ctx, pathData);
      }
    }

    // 渲染图片
    if (canvasData.images) {
      for (const img of canvasData.images) {
        await this._drawImage(ctx, img);
      }
    }

    // 渲染文字
    if (textLayer) {
      for (const item of textLayer) {
        this._drawText(ctx, item);
      }
    }
  }

  _drawText(ctx, item) {
    ctx.save();
    ctx.font = `${item.fontSize}px ${item.fontFamily}`;
    ctx.textBaseline = "top";

    let topY =
      item.boundaryY !== undefined
        ? item.boundaryY
        : item.y - item.fontSize * 0.8;

    if (item.ctm && item.ctm.length >= 4) {
      const [a, b, c, d, e, f] = item.ctm;
      ctx.translate(item.x, topY);
      ctx.transform(a, b, c, d, e || 0, f || 0);

      if (item.fill) {
        ctx.fillStyle = item.color || "#000";
        ctx.fillText(item.text, 0, 0);
      }
      if (item.stroke && item.strokeColor) {
        ctx.strokeStyle = item.strokeColor;
        ctx.lineWidth = item.lineWidth / a || 1;
        ctx.strokeText(item.text, 0, 0);
      }
    } else {
      if (item.fill) {
        ctx.fillStyle = item.color || "#000";
        ctx.fillText(item.text, item.x, topY);
      }
      if (item.stroke && item.strokeColor) {
        ctx.strokeStyle = item.strokeColor;
        ctx.lineWidth = item.lineWidth || 1;
        ctx.strokeText(item.text, item.x, topY);
      }
      if (!item.fill && !item.stroke) {
        ctx.fillStyle = item.color || "#000";
        ctx.fillText(item.text, item.x, topY);
      }
    }
    ctx.restore();
  }

  _drawImage(ctx, imgData) {
    return new Promise((resolve) => {
      const img = new Image();
      img.onload = () => {
        ctx.save();
        if (imgData.ctm && imgData.ctm.length >= 4) {
          const [a, b, c, d, e, f] = imgData.ctm;
          ctx.translate(imgData.x, imgData.y);
          ctx.transform(a, b, c, d, e || 0, f || 0);
          ctx.drawImage(img, 0, 0, 1, 1);
        } else {
          ctx.drawImage(
            img,
            imgData.x,
            imgData.y,
            imgData.width,
            imgData.height,
          );
        }
        ctx.restore();
        resolve();
      };
      img.onerror = () => resolve();
      img.src = imgData.dataURL;
    });
  }

  async _drawPath(ctx, pathData) {
    try {
      const commands = JSON.parse(pathData.commands);
      if (!commands || commands.length === 0) return;

      ctx.save();
      ctx.beginPath();

      const offsetX = pathData.x || 0;
      const offsetY = pathData.y || 0;

      for (const cmd of commands) {
        switch (cmd.cmd) {
          case "M":
            ctx.moveTo(offsetX + cmd.x, offsetY + cmd.y);
            break;
          case "L":
            ctx.lineTo(offsetX + cmd.x, offsetY + cmd.y);
            break;
          case "C":
            ctx.bezierCurveTo(
              offsetX + cmd.x1,
              offsetY + cmd.y1,
              offsetX + cmd.x2,
              offsetY + cmd.y2,
              offsetX + cmd.x,
              offsetY + cmd.y,
            );
            break;
          case "Q":
            ctx.quadraticCurveTo(
              offsetX + cmd.x1,
              offsetY + cmd.y1,
              offsetX + cmd.x,
              offsetY + cmd.y,
            );
            break;
          case "Z":
            ctx.closePath();
            break;
        }
      }

      // 填充
      if (pathData.pattern) {
        const pattern = await this._createPattern(ctx, pathData.pattern);
        if (pattern) {
          ctx.fillStyle = pattern;
          ctx.fill();
        }
      } else if (pathData.gradient) {
        const gradient = this._createGradient(ctx, pathData.gradient);
        if (gradient) {
          ctx.fillStyle = gradient;
          ctx.fill();
        }
      } else if (pathData.fillColor && pathData.fillColor !== "transparent") {
        ctx.fillStyle = pathData.fillColor;
        ctx.fill();
      }

      // 描边
      if (pathData.strokeColor && pathData.strokeColor !== "transparent") {
        ctx.strokeStyle = pathData.strokeColor;
        ctx.lineWidth = pathData.lineWidth || 1;
        ctx.lineJoin = pathData.lineJoin || "miter";
        ctx.lineCap = pathData.lineCap || "butt";
        ctx.stroke();
      }

      ctx.restore();
    } catch (e) {
      console.warn("Path render error:", e);
    }
  }

  _createGradient(ctx, gradientData) {
    let gradient;
    if (gradientData.type === "linear") {
      gradient = ctx.createLinearGradient(
        gradientData.x0,
        gradientData.y0,
        gradientData.x1,
        gradientData.y1,
      );
    } else if (gradientData.type === "radial") {
      gradient = ctx.createRadialGradient(
        gradientData.x0,
        gradientData.y0,
        gradientData.r0 || 0,
        gradientData.x1,
        gradientData.y1,
        gradientData.r1 || 0,
      );
    }

    if (gradient && gradientData.stops) {
      for (const stop of gradientData.stops) {
        gradient.addColorStop(stop.position, stop.color);
      }
    }
    return gradient;
  }

  async _createPattern(ctx, patternData) {
    if (!patternData || patternData.xStep <= 0 || patternData.yStep <= 0) {
      return null;
    }

    const mmToPx = 3.78;
    let ctmScaleX = 1,
      ctmScaleY = 1,
      ctmE = 0,
      ctmF = 0;

    if (patternData.ctm && patternData.ctm.length >= 4) {
      ctmScaleX = patternData.ctm[0];
      ctmScaleY = patternData.ctm[3];
      if (patternData.ctm.length >= 6) {
        ctmE = patternData.ctm[4] * mmToPx;
        ctmF = patternData.ctm[5] * mmToPx;
      }
    }

    const patternWidth = Math.ceil(patternData.xStep * mmToPx * ctmScaleX);
    const patternHeight = Math.ceil(patternData.yStep * mmToPx * ctmScaleY);

    const offscreen = document.createElement("canvas");
    offscreen.width = patternWidth;
    offscreen.height = patternHeight;
    const offCtx = offscreen.getContext("2d");

    const scaleX = mmToPx * ctmScaleX;
    const scaleY = mmToPx * ctmScaleY;

    // 渲染路径
    if (patternData.cellPaths) {
      for (const cellPath of patternData.cellPaths) {
        this._drawCellPath(offCtx, cellPath, scaleX, scaleY);
      }
    }

    // 渲染图片
    if (patternData.cellImages) {
      await Promise.all(
        patternData.cellImages.map((img) =>
          this._drawCellImage(offCtx, img, scaleX, scaleY),
        ),
      );
    }

    const pattern = ctx.createPattern(offscreen, "repeat");
    if (pattern) {
      const matrix = new DOMMatrix();
      matrix.translateSelf(ctmE, ctmF);
      pattern.setTransform(matrix);
    }
    return pattern;
  }

  _drawCellPath(ctx, pathData, scaleX, scaleY) {
    try {
      const commands = JSON.parse(pathData.commands);
      if (!commands || commands.length === 0) return;

      ctx.save();
      ctx.beginPath();

      for (const cmd of commands) {
        switch (cmd.cmd) {
          case "M":
            ctx.moveTo(cmd.x * scaleX, cmd.y * scaleY);
            break;
          case "L":
            ctx.lineTo(cmd.x * scaleX, cmd.y * scaleY);
            break;
          case "C":
            ctx.bezierCurveTo(
              cmd.x1 * scaleX,
              cmd.y1 * scaleY,
              cmd.x2 * scaleX,
              cmd.y2 * scaleY,
              cmd.x * scaleX,
              cmd.y * scaleY,
            );
            break;
          case "Q":
            ctx.quadraticCurveTo(
              cmd.x1 * scaleX,
              cmd.y1 * scaleY,
              cmd.x * scaleX,
              cmd.y * scaleY,
            );
            break;
          case "Z":
            ctx.closePath();
            break;
        }
      }

      if (pathData.fillColor && pathData.fillColor !== "transparent") {
        ctx.fillStyle = pathData.fillColor;
        ctx.fill();
      }
      if (pathData.strokeColor && pathData.strokeColor !== "transparent") {
        ctx.strokeStyle = pathData.strokeColor;
        ctx.lineWidth = (pathData.lineWidth || 1) * Math.min(scaleX, scaleY);
        ctx.stroke();
      }
      ctx.restore();
    } catch (e) {}
  }

  _drawCellImage(ctx, imgData, scaleX, scaleY) {
    return new Promise((resolve) => {
      if (!imgData.dataURL) {
        resolve();
        return;
      }

      const img = new Image();
      img.onload = () => {
        ctx.drawImage(
          img,
          imgData.x * scaleX,
          imgData.y * scaleY,
          imgData.width * scaleX,
          imgData.height * scaleY,
        );
        resolve();
      };
      img.onerror = () => resolve();
      img.src = imgData.dataURL;
    });
  }

  // ============ 文本层渲染 ============

  _renderTextLayer(container, textItems) {
    if (!textItems) return;

    for (const item of textItems) {
      if (item.text === "__PLACEHOLDER__") {
        const div = document.createElement("div");
        div.style.cssText = `
          position:absolute;
          left:${item.x}px;top:${item.y}px;
          width:${item.width}px;height:${item.height}px;
          background:transparent;pointer-events:none;
        `;
        container.appendChild(div);
        continue;
      }

      const span = document.createElement("span");
      span.textContent = item.text;

      let transform = "";
      if (item.ctm && item.ctm.length >= 4) {
        const [a, b, c, d, e, f] = item.ctm;
        transform = `transform:matrix(${a},${b},${c},${d},${e || 0},${f || 0});transform-origin:left top;`;
      }

      const topPos =
        item.boundaryY !== undefined
          ? item.boundaryY
          : item.y - item.fontSize * 0.8;

      span.style.cssText = `
        position:absolute;
        left:${item.x}px;top:${topPos}px;
        font-family:${item.fontFamily};
        font-size:${item.fontSize}px;
        color:transparent;
        white-space:pre;
        line-height:1;
        cursor:text;
        user-select:text;
        ${transform}
      `;
      container.appendChild(span);
    }
  }
}

// 导出
if (typeof module !== "undefined" && module.exports) {
  module.exports = OFDViewer;
} else if (typeof window !== "undefined") {
  window.OFDViewer = OFDViewer;
}

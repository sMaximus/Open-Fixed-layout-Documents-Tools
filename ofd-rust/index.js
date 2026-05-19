import init, { OFDParser } from "./pkg/ofd_rust.js";

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
  const el = document.getElementById("zoomLevel");
  if (el) el.textContent = Math.round(currentZoom * 100) + "%";
}

function applyZoomToPages() {
  const sc = getScale();
  const dpr = window.devicePixelRatio || 1;
  for (let i = 0; i < allPagesData.length; i++) {
    const container = document.getElementById(`page-${i}`);
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
  document.getElementById("status").textContent = msg;
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

  document.removeEventListener("mousemove", handleSealMouseMove, true);
  document.removeEventListener("click", handleSealPlacementClick, true);

  if (sealCursorEl) {
    sealCursorEl.remove();
    sealCursorEl = null;
  }

  document.body.style.cursor = previousBodyCursor;
  previousBodyCursor = "";

  const btn = document.getElementById("sealPlaceBtn");
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

  sealCursorEl = document.createElement("img");
  sealCursorEl.className = "seal-follow-cursor";
  sealCursorEl.src = sealDataUrl;
  sealCursorEl.alt = "seal-cursor";
  document.body.appendChild(sealCursorEl);

  previousBodyCursor = document.body.style.cursor || "";
  document.body.style.cursor = "crosshair";

  const btn = document.getElementById("sealPlaceBtn");
  if (btn) btn.classList.add("seal-mode-on");

  document.addEventListener("mousemove", handleSealMouseMove, true);
  document.addEventListener("click", handleSealPlacementClick, true);
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

  if (localX < 0 || localY < 0 || localX > rect.width || localY > rect.height) {
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
  const container = document.getElementById("docInfo");
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
  const container = document.getElementById("pageList");
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
  const page = document.getElementById(`page-${index}`);
  if (page) {
    page.scrollIntoView({ behavior: "smooth", block: "start" });
    document.querySelectorAll(".page-item").forEach((item, i) => {
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
    let styleEl = document.getElementById("ofd-font-styles");
    if (styleEl) styleEl.remove();
    styleEl = document.createElement("style");
    styleEl.id = "ofd-font-styles";
    let cssText = "";

    for (const font of embeddedFonts) {
      const fontName = `OFD_Font_${font.id}`;
      if (loadedFonts.has(fontName)) continue;
      try {
        // 通过 CSS @font-face 注册字体
        cssText += `@font-face { font-family: '${fontName}'; src: url(${font.dataUrl}); }\n`;

        // 同时通过 FontFace API 加载
        const fontFace = new FontFace(fontName, `url(${font.dataUrl})`);
        await fontFace.load();
        document.fonts.add(fontFace);
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
      document.head.appendChild(styleEl);
      console.log("[字体] @font-face CSS 已注入到 head");
    }

    // 等待所有字体就绪
    await document.fonts.ready;
    console.log(`[字体] 所有字体就绪, 已加载 ${loadedFonts.size} 个`);
    // 列出所有已注册的字体
    for (const f of document.fonts) {
      console.log(`[字体] document.fonts: ${f.family} status=${f.status}`);
    }
  } catch (err) {
    console.warn("加载字体出错:", err);
  }
}

async function parseAndRender(file) {
  stopSealPlacement();

  updateStatus("解析中...");
  const viewer = document.getElementById("viewer");
  const sealBtn = document.getElementById("sealPlaceBtn");
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
    document.getElementById("xmlBtn").style.display = "";
    document.getElementById("zoomControls").style.display = "";
    document.getElementById("printBtn").style.display = "";
    if (sealBtn) sealBtn.style.display = "";
  } catch (err) {
    updateStatus("错误: " + err.message);
    viewer.innerHTML = `<div class="empty-state"><p>解析失败</p><p style="font-size:12px;">${err.message}</p></div>`;
    if (sealBtn) sealBtn.style.display = "none";
    console.error(err);
  }
}

async function renderAllPages() {
  const viewer = document.getElementById("viewer");
  const pageCount = parser.get_page_count();
  if (pageCount <= 0) {
    viewer.innerHTML = '<div class="empty-state"><p>❌ 无法获取页数</p></div>';
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
  const container = document.createElement("div");
  container.id = `page-${index}`;
  container.className = "page-container";
  container.dataset.pageIndex = index;

  // 用 devicePixelRatio 对齐物理像素，避免亚像素模糊
  const dpr = window.devicePixelRatio || 1;
  let pxWidth, pxHeight;
  if (allPagesData[index]) {
    pxWidth = Math.round(allPagesData[index].width * getScale() * dpr) / dpr;
    pxHeight = Math.round(allPagesData[index].height * getScale() * dpr) / dpr;
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
    const placeholder = document.createElement("div");
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

  const container = document.getElementById(`page-${pageIndex}`);
  if (!container) return;

  renderedPages.add(pageIndex);

  if (page.error) {
    container.innerHTML = `<div style="color:red;padding:20px;">页面 ${pageIndex + 1} 渲染失败: ${page.error}</div>`;
    return;
  }

  const dpr = window.devicePixelRatio || 1;
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
    const svgContainer = document.createElement("div");
    svgContainer.className = "svg-layer";
    svgContainer.innerHTML = page.svg;
    container.appendChild(svgContainer);

    // 注入字体样式到 SVG 内部，确保 SVG text 元素能使用嵌入字体
    const svgEl = svgContainer.querySelector("svg");
    if (svgEl) {
      if (loadedFonts.size > 0) {
        const existingStyle = document.getElementById("ofd-font-styles");
        if (existingStyle && existingStyle.textContent) {
          const svgStyle = document.createElementNS(
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
    const textLayer = document.createElement("div");
    textLayer.className = "text-overlay";
    for (const item of page.textOverlay) {
      const span = document.createElement("span");
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
  const viewer = document.getElementById("viewer");
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

  document.querySelectorAll(".page-container").forEach((container) => {
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
  const modal = document.getElementById("xmlModal");
  const fileList = document.getElementById("xmlFileList");
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
      document.getElementById("xmlFileName").textContent = name;
      if (isXml) {
        const content = parser.read_file_text(name);
        document.getElementById("xmlContent").innerHTML = highlightXml(
          formatXml(content),
        );
      } else {
        document.getElementById("xmlContent").innerHTML =
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
  document.getElementById("xmlModal").classList.remove("active");
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
  const fontStyleEl = document.getElementById("ofd-font-styles");
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
    window.addEventListener('load', function() {
      // 等待字体和 SVG 渲染就绪
      setTimeout(function() {
        window.print();
      }, 500);
    });
  <\/script>
</body>
</html>`;

  // 在新窗口中打开打印页面
  const printWindow = window.open("", "_blank");
  if (!printWindow) {
    alert("无法打开打印窗口，请检查浏览器是否拦截了弹出窗口。");
    updateStatus(`已加载 ${currentPageCount} 页`);
    return;
  }

  printWindow.document.open();
  printWindow.document.write(printHtml);
  printWindow.document.close();

  updateStatus(`已加载 ${currentPageCount} 页`);
}

window.showXmlModal = showXmlModal;
window.hideXmlModal = hideXmlModal;
window.zoomIn = zoomIn;
window.zoomOut = zoomOut;
window.resetZoom = resetZoom;
window.setZoom = setZoom;
window.startSealPlacement = startSealPlacement;
window.startSealPlacementWithMock = startSealPlacementWithMock;
window.printOFD = printOFD;

// 按 Esc 关闭弹窗，Ctrl+/- 缩放
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape") {
    hideXmlModal();
    stopSealPlacement();
  }
  // Ctrl++ 或 Ctrl+= 放大
  if ((e.ctrlKey || e.metaKey) && (e.key === "+" || e.key === "=")) {
    e.preventDefault();
    zoomIn();
  }
  // Ctrl+- 缩小
  if ((e.ctrlKey || e.metaKey) && e.key === "-") {
    e.preventDefault();
    zoomOut();
  }
  // Ctrl+0 重置
  if ((e.ctrlKey || e.metaKey) && e.key === "0") {
    e.preventDefault();
    resetZoom();
  }
  // Ctrl+P 打印
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "p") {
    if (parser && allPagesData.length) {
      e.preventDefault();
      printOFD();
    }
  }
});

// 事件绑定
document.getElementById("fileInput").addEventListener("change", (e) => {
  const file = e.target.files[0];
  if (file) parseAndRender(file);
});

document.body.addEventListener("dragover", (e) => {
  e.preventDefault();
  e.stopPropagation();
});
document.body.addEventListener("drop", (e) => {
  e.preventDefault();
  e.stopPropagation();
  const file = e.dataTransfer.files[0];
  if (file && file.name.toLowerCase().endsWith(".ofd")) parseAndRender(file);
});

// 初始化
async function main() {
  await init();
  updateStatus("✅ 就绪");
}
main().catch((err) => {
  updateStatus("❌ WASM 加载失败");
  console.error(err);
});

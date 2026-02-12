// OFD WASM 解析器 - SVG 渲染架构
// SVG 层：渲染版式、图片、矢量、文字（freetype 渲染为图片）
// 蒙层：透明文本层，用于浏览器搜索（Ctrl+F）

let wasmReady = false;
let currentPageCount = 0;
const scale = 3.78; // mm to px

// 懒加载相关
let allPagesData = [];
const renderedPages = new Set();
const INITIAL_PAGES = 3;
const PRELOAD_THRESHOLD = 200;

async function initWasm() {
  const go = new Go();
  const result = await WebAssembly.instantiateStreaming(
    fetch("ofd.wasm"),
    go.importObject,
  );
  go.run(result.instance);
  wasmReady = true;
  updateStatus("✅ 就绪");
}

function updateStatus(msg) {
  document.getElementById("status").textContent = msg;
}

async function parseAndRender(file) {
  if (!wasmReady) {
    updateStatus("⏳ 等待 WASM 加载...");
    return;
  }

  updateStatus("⏳ 解析中...");
  const viewer = document.getElementById("viewer");
  viewer.innerHTML =
    '<div class="empty-state loading"><p>⏳ 正在解析文档...</p></div>';

  try {
    const totalStart = performance.now();

    const arrayBuffer = await file.arrayBuffer();
    const uint8Array = new Uint8Array(arrayBuffer);

    const resultJson = ofdParseFile(uint8Array);
    const result = JSON.parse(resultJson);
    if (result.error) throw new Error(result.error);

    currentPageCount = result.PageCount || 0;
    displayDocInfo(result);
    displayPageList(currentPageCount);

    updateStatus("⏳ 渲染页面...");
    await renderAllPages();

    console.log(`[总耗时] ${(performance.now() - totalStart).toFixed(2)}ms`);
    updateStatus(`✅ 已加载 ${currentPageCount} 页`);
  } catch (err) {
    updateStatus("❌ " + err.message);
    viewer.innerHTML = `<div class="empty-state"><p>❌ 解析失败</p><p style="font-size:12px;">${err.message}</p></div>`;
    console.error(err);
  }
}

function displayDocInfo(result) {
  const container = document.getElementById("docInfo");
  let html = "";
  if (result.OFD) {
    html += infoItem("版本", result.OFD.Version || "-");
    if (result.OFD.DocBody && result.OFD.DocBody.length > 0) {
      const info = result.OFD.DocBody[0].DocInfo;
      if (info) {
        html += infoItem("标题", info.Title || "-");
        html += infoItem("作者", info.Author || "-");
        html += infoItem("创建", info.CreationDate || "-");
      }
    }
  }
  html += infoItem("页数", result.PageCount || 0);
  html += infoItem("文件数", result.Files ? result.Files.length : 0);
  container.innerHTML = html;
}

function infoItem(label, value) {
  return `<div class="info-item"><span class="info-label">${label}</span><span class="info-value">${value}</span></div>`;
}

function displayPageList(count) {
  const container = document.getElementById("pageList");
  let html = "";
  for (let i = 0; i < count; i++) {
    html += `<div class="page-item" data-page="${i}" onclick="scrollToPage(${i})">第 ${i + 1} 页</div>`;
  }
  container.innerHTML =
    html || '<div style="color:#666;font-size:13px;">无页面</div>';
}

// ============ SVG 渲染架构 ============

async function renderAllPages() {
  const viewer = document.getElementById("viewer");

  try {
    const pageCount = ofdGetPageCount();
    if (pageCount <= 0) {
      viewer.innerHTML = `<div class="empty-state"><p>❌ 无法获取页数</p></div>`;
      return;
    }

    allPagesData = new Array(pageCount).fill(null);
    renderedPages.clear();
    viewer.innerHTML = "";

    const initialCount = Math.min(INITIAL_PAGES, pageCount);

    // 解析前几页
    for (let i = 0; i < initialCount; i++) {
      const stepStart = performance.now();
      const pageJson = ofdRenderPageSVG(i);
      allPagesData[i] = JSON.parse(pageJson);
      console.log(
        `[渲染] 页面${i + 1} SVG 数据解析完成: ${(performance.now() - stepStart).toFixed(2)}ms`,
      );
    }

    // 创建容器并渲染
    for (let i = 0; i < initialCount; i++) {
      const container = createPageContainer(i);
      viewer.appendChild(container);
      await renderPageContent(i);
      await new Promise((r) => setTimeout(r, 0));
    }

    // 剩余页面懒加载
    if (pageCount > initialCount) {
      requestAnimationFrame(() => {
        for (let i = initialCount; i < pageCount; i++) {
          viewer.appendChild(createPageContainer(i));
        }
        setupScrollListener();
      });
    }
  } catch (err) {
    viewer.innerHTML = `<div class="empty-state"><p>❌ 渲染失败: ${err.message}</p></div>`;
    console.error(err);
  }
}

function createPageContainer(index) {
  const pageContainer = document.createElement("div");
  pageContainer.id = `page-${index}`;
  pageContainer.className = "page-container";
  pageContainer.dataset.pageIndex = index;

  let pxWidth, pxHeight;
  if (allPagesData[index]) {
    pxWidth = allPagesData[index].width * scale;
    pxHeight = allPagesData[index].height * scale;
  } else {
    pxWidth = 210 * scale;
    pxHeight = 297 * scale;
  }

  pageContainer.style.cssText = `
    width: ${pxWidth}px;
    height: ${pxHeight}px;
    position: relative;
    background: #f5f5f5;
    box-shadow: 0 2px 10px rgba(0,0,0,0.2);
    border-radius: 4px;
    overflow: hidden;
    flex-shrink: 0;
  `;

  if (!allPagesData[index]) {
    const placeholder = document.createElement("div");
    placeholder.className = "page-placeholder";
    placeholder.style.cssText = `
      position: absolute; top: 50%; left: 50%;
      transform: translate(-50%, -50%);
      color: #999; font-size: 14px;
    `;
    placeholder.textContent = `第 ${index + 1} 页 (滚动加载)`;
    pageContainer.appendChild(placeholder);
  }

  return pageContainer;
}

async function renderPageContent(pageIndex) {
  if (renderedPages.has(pageIndex)) return;

  const page = allPagesData[pageIndex];
  if (!page) return;

  const pageContainer = document.getElementById(`page-${pageIndex}`);
  if (!pageContainer) return;

  renderedPages.add(pageIndex);

  if (page.error) {
    pageContainer.innerHTML = `<div class="page-error">页面 ${pageIndex + 1} 渲染失败: ${page.error}</div>`;
    return;
  }

  const pxWidth = page.width * scale;
  const pxHeight = page.height * scale;

  pageContainer.innerHTML = "";
  pageContainer.style.background = "#fff";
  pageContainer.style.width = `${pxWidth}px`;
  pageContainer.style.height = `${pxHeight}px`;

  // SVG 层
  if (page.svg) {
    const svgContainer = document.createElement("div");
    svgContainer.className = "svg-layer";
    svgContainer.style.cssText = `
      position: absolute; left: 0; top: 0;
      width: 100%; height: 100%;
      z-index: 0;
    `;
    svgContainer.innerHTML = page.svg;
    pageContainer.appendChild(svgContainer);

    // 调试：统计 SVG 中的元素
    const svgEl = svgContainer.querySelector("svg");
    if (svgEl) {
      const paths = svgEl.querySelectorAll("path").length;
      const images = svgEl.querySelectorAll("image").length;
      console.log(
        `[SVG] 页面${pageIndex + 1}: paths=${paths}, images=${images}, svgSize=${page.svg.length}`,
      );
    }
  } else {
    console.warn(`[SVG] 页面${pageIndex + 1}: 没有 SVG 数据`);
  }

  // 调试：输出后端调试信息
  if (page.debug) {
    if (page.debug.textDebug && page.debug.textDebug.length > 0) {
      console.log(`[TextDebug] 页面${pageIndex + 1}:`, page.debug.textDebug);
    }
  }

  // 调试：输出 textOverlay 统计
  console.log(
    `[TextOverlay] 页面${pageIndex + 1}: ${page.textOverlay ? page.textOverlay.length : 0} 项`,
  );

  // 文本蒙层（用于浏览器搜索 Ctrl+F）
  if (page.textOverlay && page.textOverlay.length > 0) {
    const textLayer = document.createElement("div");
    textLayer.className = "text-overlay";
    textLayer.style.cssText = `
      position: absolute; left: 0; top: 0;
      width: 100%; height: 100%;
      z-index: 1; overflow: hidden;
      pointer-events: none;
    `;

    for (const item of page.textOverlay) {
      const span = document.createElement("span");
      span.textContent = item.text;
      span.style.cssText = `
        position: absolute;
        left: ${item.x}px;
        top: ${item.y}px;
        width: ${item.width}px;
        height: ${item.height}px;
        font-size: ${item.height * 0.8}px;
        color: transparent;
        white-space: nowrap;
        overflow: hidden;
        line-height: ${item.height}px;
        pointer-events: auto;
        user-select: text;
        -webkit-user-select: text;
      `;
      textLayer.appendChild(span);
    }

    pageContainer.appendChild(textLayer);
  }

  console.log(`页面 ${pageIndex + 1} SVG 渲染完成`);
}

// ============ 懒加载 ============

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
    if (!renderedPages.has(idx)) {
      observer.observe(container);
    }
  });
}

async function loadAndRenderPage(pageIndex) {
  if (renderedPages.has(pageIndex)) return;
  if (allPagesData[pageIndex]) {
    await renderPageContent(pageIndex);
    return;
  }

  try {
    const pageJson = ofdRenderPageSVG(pageIndex);
    allPagesData[pageIndex] = JSON.parse(pageJson);
    await renderPageContent(pageIndex);
  } catch (err) {
    console.error(`加载页面 ${pageIndex + 1} 失败:`, err);
  }
}

// ============ 页面导航 ============

function scrollToPage(index) {
  const page = document.getElementById(`page-${index}`);
  if (page) {
    page.scrollIntoView({ behavior: "smooth", block: "start" });
    document.querySelectorAll(".page-item").forEach((item, i) => {
      item.classList.toggle("active", i === index);
    });
  }
}

// ============ 事件绑定 ============

const fileInput = document.getElementById("fileInput");
fileInput.addEventListener("change", (e) => {
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
  if (file && file.name.toLowerCase().endsWith(".ofd")) {
    parseAndRender(file);
  }
});

// 初始化
initWasm().catch((err) => {
  updateStatus("❌ WASM 加载失败");
  console.error(err);
});

import init, { OFDParser } from "./pkg/ofd_rust.js";

let parser = null;
let currentPageCount = 0;
const scale = 3.78;
const loadedFonts = new Map();
let allPagesData = [];
const renderedPages = new Set();
const INITIAL_PAGES = 3;
const PRELOAD_THRESHOLD = 200;

function updateStatus(msg) {
  document.getElementById("status").textContent = msg;
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
    if (!fonts || fonts.length === 0) return;
    const embeddedFonts = fonts.filter((f) => f.hasFile && f.dataURL);
    for (const font of embeddedFonts) {
      const fontName = `OFD_Font_${font.id}`;
      if (loadedFonts.has(fontName)) continue;
      try {
        const fontFace = new FontFace(fontName, `url(${font.dataURL})`);
        await fontFace.load();
        document.fonts.add(fontFace);
        loadedFonts.set(fontName, fontFace);
      } catch (err) {
        console.warn(`字体加载失败: ${fontName}`, err);
      }
    }
  } catch (err) {
    console.warn("加载字体出错:", err);
  }
}

async function parseAndRender(file) {
  updateStatus("⏳ 解析中...");
  const viewer = document.getElementById("viewer");
  viewer.innerHTML =
    '<div class="empty-state loading"><p>⏳ 正在解析文档...</p></div>';

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

    updateStatus("⏳ 加载字体...");
    await loadOFDFonts();

    updateStatus("⏳ 渲染页面...");
    await renderAllPages();

    console.log(`[总耗时] ${(performance.now() - totalStart).toFixed(2)}ms`);
    updateStatus(`✅ 已加载 ${currentPageCount} 页`);
    document.getElementById("xmlBtn").style.display = "";
  } catch (err) {
    updateStatus("❌ " + err.message);
    viewer.innerHTML = `<div class="empty-state"><p>❌ 解析失败</p><p style="font-size:12px;">${err.message}</p></div>`;
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

  let pxWidth, pxHeight;
  if (allPagesData[index]) {
    pxWidth = allPagesData[index].width * scale;
    pxHeight = allPagesData[index].height * scale;
  } else {
    pxWidth = 210 * scale;
    pxHeight = 297 * scale;
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

  const pxWidth = page.width * scale;
  const pxHeight = page.height * scale;

  container.innerHTML = "";
  container.style.background = "#fff";
  container.style.width = `${pxWidth}px`;
  container.style.height = `${pxHeight}px`;

  // SVG 层
  if (page.svg) {
    const svgContainer = document.createElement("div");
    svgContainer.className = "svg-layer";
    svgContainer.innerHTML = page.svg;
    container.appendChild(svgContainer);

    const svgEl = svgContainer.querySelector("svg");
    if (svgEl) {
      const paths = svgEl.querySelectorAll("path").length;
      const images = svgEl.querySelectorAll("image").length;
      const texts = svgEl.querySelectorAll("text").length;
      console.log(
        `[SVG] 页面${pageIndex + 1}: paths=${paths}, images=${images}, texts=${texts}`,
      );
    }
  }

  // 文本蒙层（用于浏览器搜索 Ctrl+F）
  if (page.textOverlay && page.textOverlay.length > 0) {
    const textLayer = document.createElement("div");
    textLayer.className = "text-overlay";
    for (const item of page.textOverlay) {
      const span = document.createElement("span");
      span.textContent = item.text;
      span.style.cssText = `
        position: absolute;
        left: ${item.x}px; top: ${item.y}px;
        width: ${item.width}px; height: ${item.height}px;
        font-size: ${item.height * 0.8}px;
        color: transparent; white-space: nowrap; overflow: hidden;
        line-height: ${item.height}px;
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
function showXmlModal() {
  if (!parser) return;
  const modal = document.getElementById("xmlModal");
  const fileList = document.getElementById("xmlFileList");
  const files = parser.get_files();

  let html = "";
  for (const f of files) {
    const isXml = f.toLowerCase().endsWith(".xml");
    html += `<div class="file-item${isXml ? "" : " non-xml"}" data-file="${f}" title="${f}">${f}</div>`;
  }
  fileList.innerHTML = html;

  fileList.querySelectorAll(".file-item:not(.non-xml)").forEach((item) => {
    item.addEventListener("click", () => {
      fileList
        .querySelectorAll(".file-item")
        .forEach((el) => el.classList.remove("active"));
      item.classList.add("active");
      const name = item.dataset.file;
      document.getElementById("xmlFileName").textContent = name;
      const content = parser.read_file_text(name);
      document.getElementById("xmlContent").textContent = formatXml(content);
    });
  });

  modal.classList.add("active");
}

function hideXmlModal() {
  document.getElementById("xmlModal").classList.remove("active");
}

function formatXml(xml) {
  // 简单的 XML 格式化
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

window.showXmlModal = showXmlModal;
window.hideXmlModal = hideXmlModal;

// 按 Esc 关闭弹窗
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape") hideXmlModal();
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

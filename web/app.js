// OFD WASM 解析器 - 双层渲染架构
// 底层 Canvas：渲染版式、图片、矢量、印章
// 上层 HTML：透明文本层，用于文字选择

let wasmReady = false;
let currentPageCount = 0;
const scale = 3.78; // mm to px
const loadedFonts = new Map();

// 懒加载相关
let allPagesData = []; // 存储所有页面数据
const renderedPages = new Set(); // 已渲染的页面索引
const INITIAL_PAGES = 3; // 首次渲染页数
const PRELOAD_THRESHOLD = 200; // 预加载阈值（像素）

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

// 加载 OFD 嵌入字体（优化版：快速跳过无嵌入字体的情况）
async function loadOFDFonts() {
  try {
    const startTime = performance.now();
    const fontsJson = ofdGetFonts();
    console.log(
      `[耗时] ofdGetFonts: ${(performance.now() - startTime).toFixed(2)}ms`,
    );

    const fonts = JSON.parse(fontsJson);
    if (fonts.error) {
      console.warn("获取字体失败:", fonts.error);
      return;
    }

    // 快速统计有嵌入文件的字体数量
    const embeddedFonts = fonts.filter((f) => f.hasFile && f.dataURL);
    console.log(
      `发现 ${fonts.length} 个字体声明，${embeddedFonts.length} 个有嵌入文件`,
    );

    // 如果没有嵌入字体，直接返回
    if (embeddedFonts.length === 0) {
      console.log("无嵌入字体，将使用系统字体");
      return;
    }

    // 只加载有嵌入文件的字体
    for (const font of embeddedFonts) {
      const fontName = `OFD_Font_${font.id}`;
      if (loadedFonts.has(fontName)) continue;

      try {
        const fontFace = new FontFace(fontName, `url(${font.dataURL})`);
        await fontFace.load();
        document.fonts.add(fontFace);
        loadedFonts.set(fontName, fontFace);
        console.log(`字体加载成功: ${fontName} (${font.fontName})`);
      } catch (err) {
        console.warn(`字体加载失败: ${fontName}`, err);
      }
    }

    console.log(`已加载 ${loadedFonts.size} 个字体`);
  } catch (err) {
    console.warn("加载字体出错:", err);
  }
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

    // 步骤1: 读取文件
    let stepStart = performance.now();
    const arrayBuffer = await file.arrayBuffer();
    const uint8Array = new Uint8Array(arrayBuffer);
    console.log(
      `[耗时] 读取文件: ${(performance.now() - stepStart).toFixed(2)}ms, 大小: ${(uint8Array.length / 1024 / 1024).toFixed(2)}MB`,
    );

    // 步骤2: 解析OFD文件
    stepStart = performance.now();
    const resultJson = ofdParseFile(uint8Array);
    console.log(
      `[耗时] ofdParseFile: ${(performance.now() - stepStart).toFixed(2)}ms`,
    );

    // 步骤3: 解析JSON
    stepStart = performance.now();
    const result = JSON.parse(resultJson);
    console.log(
      `[耗时] JSON.parse: ${(performance.now() - stepStart).toFixed(2)}ms`,
    );

    if (result.error) throw new Error(result.error);

    currentPageCount = result.PageCount || 0;
    console.log(`[信息] 文档页数: ${currentPageCount}`);

    displayDocInfo(result);
    displayPageList(currentPageCount);

    // 步骤4: 加载字体
    updateStatus("⏳ 加载字体...");
    stepStart = performance.now();
    await loadOFDFonts();
    console.log(
      `[耗时] 加载字体: ${(performance.now() - stepStart).toFixed(2)}ms`,
    );

    // 步骤5: 渲染页面
    updateStatus("⏳ 渲染页面...");
    stepStart = performance.now();
    await renderAllPages();
    console.log(
      `[耗时] 渲染页面: ${(performance.now() - stepStart).toFixed(2)}ms`,
    );

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
    html += `<div class="page-item" data-page="${i}" onclick="scrollToPage(${i})">第 ${
      i + 1
    } 页</div>`;
  }
  container.innerHTML =
    html || '<div style="color:#666;font-size:13px;">无页面</div>';
}

// ============ 双层渲染架构 ============

/**
 * 渲染页面（懒加载模式）
 * 只解析和渲染前3页，其余页面滚动时按需加载
 */
async function renderAllPages() {
  const viewer = document.getElementById("viewer");

  try {
    // 获取总页数
    let stepStart = performance.now();
    const pageCount = ofdGetPageCount();
    console.log(
      `[耗时] ofdGetPageCount: ${(performance.now() - stepStart).toFixed(2)}ms, 页数: ${pageCount}`,
    );

    if (pageCount <= 0) {
      viewer.innerHTML = `<div class="empty-state"><p>❌ 无法获取页数</p></div>`;
      return;
    }

    // 初始化
    allPagesData = new Array(pageCount).fill(null);
    renderedPages.clear();
    viewer.innerHTML = "";

    // 只解析前 INITIAL_PAGES 页
    const initialCount = Math.min(INITIAL_PAGES, pageCount);

    for (let i = 0; i < initialCount; i++) {
      stepStart = performance.now();
      const pageJson = ofdRenderPage(i);
      const parseTime = performance.now() - stepStart;

      stepStart = performance.now();
      const page = JSON.parse(pageJson);
      const jsonTime = performance.now() - stepStart;

      allPagesData[i] = page;
      console.log(
        `[耗时] 页面${i + 1}: ofdRenderPage=${parseTime.toFixed(2)}ms, JSON=${jsonTime.toFixed(2)}ms, 路径=${page.canvasData?.paths?.length || 0}, 图片=${page.canvasData?.images?.length || 0}`,
      );
    }

    // 创建所有页面容器
    stepStart = performance.now();
    for (let i = 0; i < pageCount; i++) {
      const pageContainer = document.createElement("div");
      pageContainer.id = `page-${i}`;
      pageContainer.className = "page-container";
      pageContainer.dataset.pageIndex = i;

      // 获取页面尺寸
      let pxWidth, pxHeight;
      if (allPagesData[i]) {
        pxWidth = allPagesData[i].width * scale;
        pxHeight = allPagesData[i].height * scale;
      } else {
        // 未解析的页面使用默认尺寸（A4）
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

      if (i >= initialCount) {
        // 未解析的页面显示占位符
        const placeholder = document.createElement("div");
        placeholder.className = "page-placeholder";
        placeholder.style.cssText = `
          position: absolute;
          top: 50%;
          left: 50%;
          transform: translate(-50%, -50%);
          color: #999;
          font-size: 14px;
        `;
        placeholder.textContent = `第 ${i + 1} 页 (滚动加载)`;
        pageContainer.appendChild(placeholder);
      }

      viewer.appendChild(pageContainer);
    }
    console.log(
      `[耗时] 创建${pageCount}个容器: ${(performance.now() - stepStart).toFixed(2)}ms`,
    );

    // 渲染前 INITIAL_PAGES 页
    for (let i = 0; i < initialCount; i++) {
      stepStart = performance.now();
      await renderPageContent(i);
      console.log(
        `[耗时] 渲染页面${i + 1}内容: ${(performance.now() - stepStart).toFixed(2)}ms`,
      );
    }

    // 设置滚动监听
    setupScrollListener();

    console.log(`初始加载完成: 渲染了 ${initialCount} 页，共 ${pageCount} 页`);
  } catch (err) {
    viewer.innerHTML = `<div class="empty-state"><p>❌ 渲染失败: ${err.message}</p></div>`;
    console.error(err);
  }
}

/**
 * 渲染单个页面的内容
 */
async function renderPageContent(pageIndex) {
  if (renderedPages.has(pageIndex)) {
    return;
  }

  const page = allPagesData[pageIndex];
  if (!page) return;

  const pageContainer = document.getElementById(`page-${pageIndex}`);
  if (!pageContainer) return;

  // 标记为已渲染
  renderedPages.add(pageIndex);

  if (page.error) {
    pageContainer.innerHTML = `<div class="page-error">页面 ${pageIndex + 1} 渲染失败: ${page.error}</div>`;
    return;
  }

  const pxWidth = page.width * scale;
  const pxHeight = page.height * scale;

  // 清空并更新容器
  pageContainer.innerHTML = "";
  pageContainer.style.background = "#fff";
  pageContainer.style.width = `${pxWidth}px`;
  pageContainer.style.height = `${pxHeight}px`;

  // Canvas 层
  const canvas = document.createElement("canvas");
  canvas.width = pxWidth;
  canvas.height = pxHeight;
  canvas.style.cssText = `position: absolute; left: 0; top: 0; z-index: 0;`;
  pageContainer.appendChild(canvas); // 文本层
  const textLayer = document.createElement("div");
  textLayer.className = "text-layer";
  textLayer.style.cssText = `
    position: absolute; left: 0; top: 0;
    width: 100%; height: 100%;
    z-index: 1; overflow: hidden; pointer-events: auto;
  `;
  pageContainer.appendChild(textLayer);

  // 渲染内容
  await renderCanvasLayer(canvas, page.canvasData, page.textLayer);
  renderTransparentTextLayer(textLayer, page.textLayer);

  console.log(`页面 ${pageIndex + 1} 渲染完成`);
}

/**
 * 设置滚动监听，实现懒加载
 */
function setupScrollListener() {
  const viewer = document.getElementById("viewer");

  const observer = new IntersectionObserver(
    (entries) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting) {
          const pageIndex = parseInt(entry.target.dataset.pageIndex, 10);
          if (!isNaN(pageIndex) && !renderedPages.has(pageIndex)) {
            // 按需解析和渲染
            loadAndRenderPage(pageIndex);
          }
        }
      });
    },
    {
      root: viewer,
      rootMargin: `${PRELOAD_THRESHOLD}px`,
      threshold: 0,
    },
  );

  document.querySelectorAll(".page-container").forEach((container) => {
    observer.observe(container);
  });
}

/**
 * 按需加载并渲染页面
 */
async function loadAndRenderPage(pageIndex) {
  if (renderedPages.has(pageIndex)) return;
  if (allPagesData[pageIndex]) {
    // 已解析但未渲染
    await renderPageContent(pageIndex);
    return;
  }

  console.log(`按需解析页面 ${pageIndex + 1}...`);

  try {
    const pageJson = ofdRenderPage(pageIndex);
    const page = JSON.parse(pageJson);
    allPagesData[pageIndex] = page;
    await renderPageContent(pageIndex);
  } catch (err) {
    console.error(`加载页面 ${pageIndex + 1} 失败:`, err);
  }
}

// ============ 底层 Canvas 渲染 ============
// 渲染顺序：背景 → 路径 → 文字 → 图片（印章在最上层）

async function renderCanvasLayer(canvas, canvasData, textLayer) {
  const ctx = canvas.getContext("2d");

  // 1. 白色背景
  ctx.fillStyle = "#fff";
  ctx.fillRect(0, 0, canvas.width, canvas.height);

  if (!canvasData) canvasData = {};

  // 2. 渲染路径（矢量图形、线条）- 需要 await 因为 Pattern 是异步的
  if (canvasData.paths) {
    for (const pathData of canvasData.paths) {
      await drawPath(ctx, pathData);
    }
  }

  // 4. 渲染图片（印章等，最上层）
  if (canvasData.images && canvasData.images.length > 0) {
    console.log(`渲染 ${canvasData.images.length} 个图片/印章`);
    for (const img of canvasData.images) {
      console.log(
        `图片: x=${img.x}, y=${img.y}, w=${img.width}, h=${img.height}`,
      );
      await drawImage(ctx, img);
    }
  }

  // 3. 渲染文字
  if (textLayer && textLayer.length > 0) {
    for (const item of textLayer) {
      drawText(ctx, item);
    }
  }
}

// Canvas 绘制文字
function drawText(ctx, item) {
  ctx.save();

  let fontFamily = item.fontFamily;
  ctx.font = `${item.fontSize}px ${fontFamily}`;
  ctx.textBaseline = "top";

  // OFD 坐标系统：
  // - item.boundaryY 是 Boundary 的顶部位置
  // - item.textCodeY 是相对于 Boundary 顶部的基线偏移
  // 不同字体的 textCodeY 不同，这是为了让不同字体对齐
  // 我们应该使用 boundaryY 作为统一的顶部参考点

  let topY;
  if (item.boundaryY !== undefined && item.textCodeY !== undefined) {
    // 使用 Boundary Y 作为顶部参考点
    topY = item.boundaryY;
  } else {
    // 兼容旧版本：使用固定比例
    const baselineOffset = item.fontSize * 0.8;
    topY = item.y - baselineOffset;
  }

  // 应用 CTM 变换矩阵
  if (item.ctm && item.ctm.length >= 4) {
    const [a, b, c, d, e, f] = item.ctm;
    // 先平移到文字位置，再应用 CTM 变换
    ctx.translate(item.x, topY);
    ctx.transform(a, b, c, d, e || 0, f || 0);

    // 处理填充 - 后端已经计算好 fill 值
    if (item.fill) {
      ctx.fillStyle = item.color || "#000";
      ctx.fillText(item.text, 0, 0);
    }

    // 处理描边
    if (item.stroke && item.strokeColor) {
      ctx.strokeStyle = item.strokeColor;
      ctx.lineWidth = item.lineWidth / a || 1;
      ctx.strokeText(item.text, 0, 0);
    }
  } else {
    // 没有 CTM，直接绘制
    if (item.fill) {
      ctx.fillStyle = item.color || "#000";
      ctx.fillText(item.text, item.x, topY);
    }
    if (item.stroke && item.strokeColor) {
      ctx.strokeStyle = item.strokeColor;
      ctx.lineWidth = item.lineWidth || 1;
      ctx.strokeText(item.text, item.x, topY);
    }
    // 如果既没有 fill 也没有 stroke，默认填充
    if (!item.fill && !item.stroke) {
      ctx.fillStyle = item.color || "#000";
      ctx.fillText(item.text, item.x, topY);
    }
  }

  ctx.restore();
}

// Canvas 绘制图片
function drawImage(ctx, imgData) {
  return new Promise((resolve) => {
    const img = new Image();
    img.onload = () => {
      ctx.save();

      // 如果有 CTM 变换（旋转/倾斜）
      if (imgData.ctm && imgData.ctm.length >= 4) {
        const [a, b, c, d, e, f] = imgData.ctm;

        // 移动到 Boundary 的位置
        ctx.translate(imgData.x, imgData.y);

        // 图片在对象坐标系中是单位正方形 (0,0)-(1,1)
        // CTM 将其变换到实际尺寸和旋转
        // 需要将 CTM 应用到单位正方形，然后绘制图片
        ctx.transform(a, b, c, d, e || 0, f || 0);

        // 绘制单位正方形大小的图片（CTM 会将其变换到正确尺寸）
        ctx.drawImage(img, 0, 0, 1, 1);
      } else {
        // 普通绘制，直接使用 Boundary 的尺寸
        ctx.drawImage(img, imgData.x, imgData.y, imgData.width, imgData.height);
      }

      ctx.restore();
      resolve();
    };
    img.onerror = (err) => {
      console.error("图片加载失败:", err);
      resolve();
    };
    img.src = imgData.dataURL;
  });
}

/**
 * 创建 Pattern 填充（异步版本）
 *
 * Pattern 元素示例：
 * <ofd:Pattern Width="467" Height="155" XStep="1920" YStep="1080" RelativeTo="Page"
 *              CTM="0.2393 0 0 0.2393 165.7258 -152.0952">
 *
 * 关键参数：
 * - Width, Height: 单元格内容尺寸 (mm)
 * - XStep, YStep: 平铺步长 (mm)
 * - CTM: [a, b, c, d, e, f]
 *   - a, d: 缩放因子（如 0.2393）
 *   - e, f: 起始偏移（mm），f 可能为负数表示第一个 tile 在页面外
 *
 * 平铺计算示例（页面高 190.5mm）：
 * - 起始 Y = f = -152.1mm（页面外）
 * - 步长 = YStep * d = 1080 * 0.2393 = 258.44mm
 * - Tile 0: Y = -152.1mm（不可见）
 * - Tile 1: Y = -152.1 + 258.44 = 106.34mm（可见，在页面中下部）
 *
 * @param {CanvasRenderingContext2D} ctx - 主画布上下文
 * @param {Object} patternData - Pattern 数据
 * @returns {Promise<CanvasPattern|null>} Canvas 图案对象
 */
async function createPatternFill(ctx, patternData) {
  console.log("=== createPatternFill 开始 ===");
  console.log("Pattern Data:", patternData);

  if (!patternData || patternData.xStep <= 0 || patternData.yStep <= 0) {
    console.warn("Pattern 数据无效:", patternData);
    return null;
  }

  const mmToPx = 3.78; // mm to px

  // 获取 CTM 参数
  // CTM = [a, b, c, d, e, f] 其中：
  // - a, d 是缩放因子（如 0.2393）
  // - e, f 是起始偏移（mm）
  let ctmScaleX = 1,
    ctmScaleY = 1;
  let ctmE_mm = 0,
    ctmF_mm = 0;
  if (patternData.ctm && patternData.ctm.length >= 4) {
    ctmScaleX = patternData.ctm[0];
    ctmScaleY = patternData.ctm[3];
    if (patternData.ctm.length >= 6) {
      ctmE_mm = patternData.ctm[4];
      ctmF_mm = patternData.ctm[5];
    }
  }

  // 将 CTM 平移转换为像素
  const ctmE_px = ctmE_mm * mmToPx;
  const ctmF_px = ctmF_mm * mmToPx;

  // 计算应用 CTM 缩放后的实际重复单元尺寸
  // XStep/YStep 是 mm，需要先转像素再乘缩放
  // 例如：YStep=1080mm * 3.78 * 0.2393 ≈ 976px
  const patternWidth = Math.ceil(patternData.xStep * mmToPx * ctmScaleX);
  const patternHeight = Math.ceil(patternData.yStep * mmToPx * ctmScaleY);

  console.log(
    `Pattern 分析:\n` +
      `  - XStep=${patternData.xStep}mm, YStep=${patternData.yStep}mm\n` +
      `  - CTM scale=(${ctmScaleX}, ${ctmScaleY})\n` +
      `  - CTM offset=(${ctmE_mm}mm, ${ctmF_mm}mm) = (${ctmE_px.toFixed(2)}px, ${ctmF_px.toFixed(2)}px)\n` +
      `  - 烘焙后单元尺寸=(${patternWidth}px, ${patternHeight}px)\n` +
      `  - 平铺计算:\n` +
      `    * Tile 0: Y = ${ctmF_mm.toFixed(2)}mm = ${ctmF_px.toFixed(2)}px (${ctmF_mm < 0 ? "页面外" : "页面内"})\n` +
      `    * Tile 1: Y = ${ctmF_mm.toFixed(2)} + ${(patternData.yStep * ctmScaleY).toFixed(2)} = ${(ctmF_mm + patternData.yStep * ctmScaleY).toFixed(2)}mm = ${(ctmF_px + patternHeight).toFixed(2)}px`,
  );

  // 1. 创建离屏 Canvas，尺寸为应用 CTM 后的实际尺寸
  const offscreenCanvas = document.createElement("canvas");
  offscreenCanvas.width = patternWidth;
  offscreenCanvas.height = patternHeight;
  const offCtx = offscreenCanvas.getContext("2d");

  // 清空画布
  offCtx.clearRect(0, 0, offscreenCanvas.width, offscreenCanvas.height);

  // 2. 渲染 CellContent，应用 CTM 缩放
  // 图片坐标需要乘以 mmToPx * ctmScale
  const combinedScaleX = mmToPx * ctmScaleX;
  const combinedScaleY = mmToPx * ctmScaleY;

  // 渲染路径（同步）
  if (patternData.cellPaths && patternData.cellPaths.length > 0) {
    for (const cellPath of patternData.cellPaths) {
      drawCellPathWithScale(offCtx, cellPath, combinedScaleX, combinedScaleY);
    }
  }

  // 渲染图片（异步，等待所有图片加载完成）
  if (patternData.cellImages && patternData.cellImages.length > 0) {
    const imagePromises = patternData.cellImages.map((cellImg) =>
      drawCellImageAsync(offCtx, cellImg, combinedScaleX, combinedScaleY),
    );
    await Promise.all(imagePromises);
    console.log("Pattern: 所有图片加载完成");
  }

  // 3. 创建 CanvasPattern
  const pattern = ctx.createPattern(offscreenCanvas, "repeat");
  if (!pattern) {
    return null;
  }

  // 调试：将离屏 Canvas 添加到页面上查看
  if (window.DEBUG_PATTERN) {
    const debugDiv = document.createElement("div");
    debugDiv.style.cssText = `
      position: fixed;
      top: 10px;
      right: 10px;
      z-index: 9999;
      background: white;
      border: 2px solid red;
      padding: 10px;
    `;
    debugDiv.innerHTML = `<div>Pattern Debug (${patternWidth}×${patternHeight})</div>`;
    offscreenCanvas.style.border = "1px solid black";
    offscreenCanvas.style.maxWidth = "300px";
    offscreenCanvas.style.maxHeight = "300px";
    debugDiv.appendChild(offscreenCanvas.cloneNode(true));
    document.body.appendChild(debugDiv);
  }

  // 4. 处理 RelativeTo 坐标对齐和负偏移
  // Canvas Pattern 的 repeat 会自动平铺
  // setTransform 设置第一个 tile 的起始位置
  // 如果 f 为负数，第一个 tile 在页面外，第二个 tile 会自动出现在正确位置
  const matrix = new DOMMatrix();

  if (patternData.relativeTo === "Page") {
    // Pattern 相对于页面原点 (0,0) 对齐
    // CTM 的 e, f 定义了 Pattern 的起始位置（像素）
    matrix.translateSelf(ctmE_px, ctmF_px);
    console.log(
      `Pattern RelativeTo=Page:\n` +
        `  - 起始位置: (${ctmE_px.toFixed(2)}px, ${ctmF_px.toFixed(2)}px)\n` +
        `  - 第 2 行位置: Y = ${ctmF_px.toFixed(2)} + ${patternHeight} = ${(ctmF_px + patternHeight).toFixed(2)}px`,
    );
  } else {
    // RelativeTo="Object" (默认)
    // Pattern 相对于对象边界框左上角对齐
    matrix.translateSelf(ctmE_px, ctmF_px);
    console.log(
      `Pattern RelativeTo=Object:\n` +
        `  - 总平移: (${ctmE_px.toFixed(2)}px, ${ctmF_px.toFixed(2)}px)`,
    );
  }

  pattern.setTransform(matrix);
  return pattern;
}

/**
 * 在离屏 Canvas 上绘制单元格路径（带独立 X/Y 缩放）
 */
function drawCellPathWithScale(ctx, pathData, scaleX, scaleY) {
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
  } catch (e) {
    console.warn("Pattern path render error:", e);
  }
}

/**
 * 异步绘制单元格图片（带独立 X/Y 缩放）
 * @returns {Promise<void>}
 */
function drawCellImageAsync(ctx, imgData, scaleX, scaleY) {
  return new Promise((resolve) => {
    if (!imgData.dataURL) {
      console.warn("CellImage: 没有 dataURL");
      resolve();
      return;
    }

    const img = new Image();

    img.onload = () => {
      ctx.save();

      // 坐标和尺寸应用各自的缩放因子
      const x = imgData.x * scaleX;
      const y = imgData.y * scaleY;
      const w = imgData.width * scaleX;
      const h = imgData.height * scaleY;

      console.log(
        `CellImage: 绘制图片 original=(${imgData.x}, ${imgData.y}, ${imgData.width}, ${imgData.height}), ` +
          `scaled=(${x.toFixed(2)}, ${y.toFixed(2)}, ${w.toFixed(2)}, ${h.toFixed(2)})`,
      );

      ctx.drawImage(img, x, y, w, h);
      ctx.restore();
      resolve();
    };

    img.onerror = (err) => {
      console.error("CellImage: 图片加载失败", err);
      resolve();
    };

    img.src = imgData.dataURL;
  });
}

// Canvas 绘制路径（异步版本，支持 Pattern）
async function drawPath(ctx, pathData) {
  try {
    const commands = JSON.parse(pathData.commands);
    console.log("Path Commands:", commands.length, commands);
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

    // 处理填充（Pattern、渐变或纯色）
    if (pathData.pattern) {
      // Pattern 图案填充（异步等待图片加载）
      const pattern = await createPatternFill(ctx, pathData.pattern);
      if (pattern) {
        ctx.fillStyle = pattern;
        ctx.fill();
      }
    } else if (pathData.gradient) {
      // 渐变填充
      let gradient;
      if (pathData.gradient.type === "linear") {
        gradient = ctx.createLinearGradient(
          pathData.gradient.x0,
          pathData.gradient.y0,
          pathData.gradient.x1,
          pathData.gradient.y1,
        );
      } else if (pathData.gradient.type === "radial") {
        gradient = ctx.createRadialGradient(
          pathData.gradient.x0,
          pathData.gradient.y0,
          pathData.gradient.r0 || 0,
          pathData.gradient.x1,
          pathData.gradient.y1,
          pathData.gradient.r1 || 0,
        );
      }

      if (gradient && pathData.gradient.stops) {
        for (const stop of pathData.gradient.stops) {
          gradient.addColorStop(stop.position, stop.color);
        }
        ctx.fillStyle = gradient;
        ctx.fill();
      }
    } else if (pathData.fillColor && pathData.fillColor !== "transparent") {
      // 纯色填充
      ctx.fillStyle = pathData.fillColor;
      ctx.fill();
    }

    // 处理描边
    if (pathData.strokeColor && pathData.strokeColor !== "transparent") {
      ctx.strokeStyle = pathData.strokeColor;
      ctx.lineWidth = pathData.lineWidth || 1;
      console.warn(ctx.lineWidth);
      // 设置线条连接样式
      if (pathData.lineJoin) {
        ctx.lineJoin = pathData.lineJoin;
      } else {
        ctx.lineJoin = "miter"; // 默认值
      }

      // 设置线条端点样式
      if (pathData.lineCap) {
        ctx.lineCap = pathData.lineCap;
      } else {
        ctx.lineCap = "butt"; // 默认值
      }

      // 如果是 miter 连接，设置 miterLimit 以避免尖角过长
      if (ctx.lineJoin === "miter") {
        ctx.miterLimit = 10; // 默认值
      }

      ctx.stroke();
    }

    ctx.restore();
  } catch (e) {
    console.warn("Path render error:", e);
  }
}

// ============ 上层透明文本层 ============
// 关键：color: transparent，位置与 Canvas 文字完全重合

// 创建一个隐藏的 canvas 用于测量文字
const measureCanvas = document.createElement("canvas");
const measureCtx = measureCanvas.getContext("2d");

function renderTransparentTextLayer(container, textItems) {
  if (!textItems || textItems.length === 0) return;

  for (const item of textItems) {
    // 检查是否是占位标记
    if (item.text === "__PLACEHOLDER__") {
      // 创建透明占位 div
      const placeholderDiv = document.createElement("div");
      console.log(item);
      placeholderDiv.style.cssText = `
        position: absolute;
        left: ${item.x}px;
        top: ${item.y}px;
        width: ${item.width}px;
        height: ${item.height}px;
        background: transparent;
        pointer-events: none;
        z-index: 3;
      `;

      placeholderDiv.setAttribute("data-placeholder", "true");
      container.appendChild(placeholderDiv);
      continue;
    }

    const span = document.createElement("span");
    span.textContent = item.text;

    // 计算 CTM 变换
    let transform = "";
    if (item.ctm && item.ctm.length >= 4) {
      const [a, b, c, d, e, f] = item.ctm;
      transform = `transform: matrix(${a}, ${b}, ${c}, ${d}, ${e || 0}, ${f || 0}); transform-origin: left top;`;
    }

    // OFD 坐标系统：使用 Boundary Y 作为统一的顶部参考点
    let topPos;
    if (item.boundaryY !== undefined && item.textCodeY !== undefined) {
      topPos = item.boundaryY;
    } else {
      // 兼容旧版本
      const baselineOffset = item.fontSize * 0.8;
      topPos = item.y - baselineOffset;
    }

    // 关键样式：
    // - color: transparent 让文字透明（用户看不见）
    // - 字号、字体、位置与 Canvas 完全一致
    // - user-select: text 允许选择
    span.style.cssText = `
      position: absolute;
      left: ${item.x}px;
      top: ${topPos}px;
      font-family: ${item.fontFamily};
      font-size: ${item.fontSize}px;
      color: transparent;
      white-space: pre;
      line-height: 1;
      letter-spacing: 0;
      cursor: text;
      user-select: text;
      -webkit-user-select: text;
      -moz-user-select: text;
      ${transform}
    `;

    container.appendChild(span);
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

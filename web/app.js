// OFD WASM 解析器 - 双层渲染架构
// 底层 Canvas：渲染版式、图片、矢量、印章
// 上层 HTML：透明文本层，用于文字选择

let wasmReady = false;
let currentPageCount = 0;
const scale = 3.78; // mm to px
const loadedFonts = new Map();

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

// 加载 OFD 嵌入字体
async function loadOFDFonts() {
  try {
    const fontsJson = ofdGetFonts();
    const fonts = JSON.parse(fontsJson);
    if (fonts.error) {
      console.warn("获取字体失败:", fonts.error);
      return;
    }

    console.log(`发现 ${fonts.length} 个字体`);

    // 打印每个字体的详细信息
    for (const font of fonts) {
      console.log(
        `字体: ID=${font.id}, Name=${font.fontName}, Family=${font.familyName}, HasFile=${font.hasFile}`,
      );
    }

    for (const font of fonts) {
      if (font.hasFile && font.dataURL) {
        const fontName = `OFD_Font_${font.id}`;
        if (loadedFonts.has(fontName)) continue;

        try {
          // 直接使用 data URL 创建 FontFace
          const fontFace = new FontFace(fontName, `url(${font.dataURL})`);
          await fontFace.load();
          document.fonts.add(fontFace);
          loadedFonts.set(fontName, fontFace);
          console.log(`字体加载成功: ${fontName} (${font.fontName})`);
        } catch (err) {
          console.warn(`字体加载失败: ${fontName}`, err);
        }
      } else {
        console.log(
          `字体 ${font.id} 没有嵌入文件，将使用系统字体: ${
            font.fontName || font.familyName
          }`,
        );
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
    const arrayBuffer = await file.arrayBuffer();
    const uint8Array = new Uint8Array(arrayBuffer);
    const resultJson = ofdParseFile(uint8Array);
    const result = JSON.parse(resultJson);

    if (result.error) throw new Error(result.error);

    currentPageCount = result.PageCount || 0;
    displayDocInfo(result);
    displayPageList(currentPageCount);

    updateStatus("⏳ 加载字体...");
    await loadOFDFonts();

    updateStatus("⏳ 渲染页面...");
    await renderAllPages();

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

async function renderAllPages() {
  const viewer = document.getElementById("viewer");

  try {
    const resultJson = ofdRenderAllPages();
    const pages = JSON.parse(resultJson);
    if (pages.error) {
      viewer.innerHTML = `<div class="empty-state"><p>❌ ${pages.error}</p></div>`;
      return;
    }

    viewer.innerHTML = "";

    for (let i = 0; i < pages.length; i++) {
      const page = pages[i];
      const pageContainer = document.createElement("div");
      pageContainer.id = `page-${i}`;
      pageContainer.className = "page-container";

      // 输出调试信息
      if (page.debug) {
        console.log(`页面 ${i + 1} 调试信息:`);
        if (page.debug.textDebug) {
          console.log("文本调试:", page.debug.textDebug);
        }
        if (page.debug.stamps) {
          console.log("印章调试:", page.debug.stamps);
        }
      }

      console.log(
        `页面 ${i + 1}: 文本=${page.textLayer?.length || 0}, 路径=${
          page.canvasData?.paths?.length || 0
        }, 图片=${page.canvasData?.images?.length || 0}`,
      );

      if (page.error) {
        pageContainer.innerHTML = `<div class="page-error">页面 ${
          i + 1
        } 渲染失败: ${page.error}</div>`;
      } else {
        const pxWidth = page.width * scale;
        const pxHeight = page.height * scale;

        // 页面容器样式
        pageContainer.style.cssText = `
          width: ${pxWidth}px;
          height: ${pxHeight}px;
          position: relative;
          background: #fff;
          box-shadow: 0 2px 10px rgba(0,0,0,0.2);
          border-radius: 4px;
          overflow: hidden;
          flex-shrink: 0;
        `;

        // ===== 底层：Canvas 层 =====
        const canvas = document.createElement("canvas");
        canvas.width = pxWidth;
        canvas.height = pxHeight;
        canvas.style.cssText = `
          position: absolute;
          left: 0;
          top: 0;
          z-index: 0;
        `;
        pageContainer.appendChild(canvas);

        // ===== 上层：透明文本层 =====
        const textLayer = document.createElement("div");
        textLayer.className = "text-layer";
        textLayer.style.cssText = `
          position: absolute;
          left: 0;
          top: 0;
          width: 100%;
          height: 100%;
          z-index: 1;
          overflow: hidden;
          pointer-events: auto;
        `;
        pageContainer.appendChild(textLayer);

        // 等待所有字体加载完成
        await document.fonts.ready;

        // 渲染底层 Canvas（完整视觉内容）
        console.log(
          `渲染页面 ${i + 1}, 图片数量: ${page.canvasData?.images?.length || 0}`,
        );
        await renderCanvasLayer(canvas, page.canvasData, page.textLayer);

        // 渲染上层透明文本（用于选择）
        renderTransparentTextLayer(textLayer, page.textLayer);

        // 只为印章图片添加透明 div 层（用于选中和交互）
        if (page.canvasData?.images) {
          for (const img of page.canvasData.images) {
            // 只为印章添加蒙层，普通图片不需要
            if (!img.isSeal) continue;

            const sealDiv = document.createElement("div");
            sealDiv.className = img.placeholder
              ? "seal-placeholder"
              : "seal-overlay";
            sealDiv.style.cssText = `
              position: absolute;
              left: ${img.x}px;
              top: ${img.y}px;
              width: ${img.width}px;
              height: ${img.height}px;
              background: transparent;
              pointer-events: auto;
              cursor: ${img.placeholder ? "help" : "pointer"};
              z-index: 2;
            `;

            if (img.placeholder) {
              sealDiv.title = "印章占位框（加载失败）";
              sealDiv.setAttribute("data-seal-placeholder", "true");
            } else {
              sealDiv.title = "电子印章";
              sealDiv.setAttribute("data-seal", "true");
            }

            pageContainer.appendChild(sealDiv);
          }
        }
      }

      viewer.appendChild(pageContainer);
    }
  } catch (err) {
    viewer.innerHTML = `<div class="empty-state"><p>❌ 渲染失败: ${err.message}</p></div>`;
    console.error(err);
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

  // 2. 渲染路径（矢量图形、线条）
  if (canvasData.paths) {
    for (const pathData of canvasData.paths) {
      drawPath(ctx, pathData);
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
  // ctx.textBaseline = "alphabetic";

  ctx.textBaseline = "alphabetic";
  const ascentRatio = 0.88; // 与 HTML 保持完全一致的魔法数值
  const correctedY = item.y - item.fontSize * ascentRatio;

  // 应用 CTM 变换矩阵
  if (item.ctm && item.ctm.length >= 4) {
    const a = item.ctm[0];
    const b = item.ctm[1];
    const c = item.ctm[2];
    const d = item.ctm[3];
    const e = item.ctm.length > 4 ? item.ctm[4] : 0;
    const f = item.ctm.length > 5 ? item.ctm[5] : 0;

    // 先平移到文字位置，再应用 CTM 变换
    ctx.translate(item.x, correctedY);
    ctx.transform(a, b, c, d, e, f);

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
      ctx.fillText(item.text, item.x, correctedY);
    }
    if (item.stroke && item.strokeColor) {
      ctx.strokeStyle = item.strokeColor;
      ctx.lineWidth = item.lineWidth || 1;
      ctx.strokeText(item.text, item.x, correctedY);
    }
    // 如果既没有 fill 也没有 stroke，默认填充
    if (!item.fill && !item.stroke) {
      ctx.fillStyle = item.color || "#000";
      ctx.fillText(item.text, item.x, correctedY);
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

// Canvas 绘制路径
function drawPath(ctx, pathData) {
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

    // 处理填充（纯色或渐变）
    if (pathData.gradient) {
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
      ctx.stroke();
    }

    ctx.restore();
  } catch (e) {
    console.warn("Path render error:", e);
  }
}

// ============ 上层透明文本层 ============
// 关键：color: transparent，位置与 Canvas 文字完全重合

function renderTransparentTextLayer(container, textItems) {
  if (!textItems || textItems.length === 0) return;

  // Ascent 比例：基线到顶部的距离约占字号的 88%
  const ascentRatio = 0.88;

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
      const a = item.ctm[0];
      const b = item.ctm[1];
      const c = item.ctm[2];
      const d = item.ctm[3];
      const e = item.ctm[4] || 0;
      const f = item.ctm[5] || 0;
      // CSS transform matrix(a, b, c, d, e, f)
      transform = `transform: matrix(${a}, ${b}, ${c}, ${d}, ${e}, ${f}); transform-origin: left top;`;
    }

    // OFD 的 Y 坐标是基线位置，HTML 的 top 是元素顶部
    // 需要减去 Ascent（基线到顶部的距离）
    const topPos = item.y - item.fontSize * ascentRatio;

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

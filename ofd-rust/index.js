import { initOFDViewer } from "./pkg/ofd_rust.js";

function bindViewerControls(viewer) {
  document
    .getElementById("uploadBtn")
    ?.addEventListener("click", () =>
      document.getElementById("fileInput")?.click(),
    );
  document
    .getElementById("xmlBtn")
    ?.addEventListener("click", () => viewer.showXmlModal());
  document
    .getElementById("sealPlaceBtn")
    ?.addEventListener("click", () => viewer.startSealPlacementWithMock());
  document
    .getElementById("printBtn")
    ?.addEventListener("click", () => viewer.printOFD());
  document
    .getElementById("zoomOutBtn")
    ?.addEventListener("click", () => viewer.zoomOut());
  document
    .getElementById("zoomInBtn")
    ?.addEventListener("click", () => viewer.zoomIn());
  document
    .getElementById("zoomResetBtn")
    ?.addEventListener("click", () => viewer.resetZoom());
  document
    .getElementById("xmlModalCloseBtn")
    ?.addEventListener("click", () => viewer.hideXmlModal());
}

initOFDViewer()
  .then((viewer) => {
    bindViewerControls(viewer);
  })
  .catch((err) => {
    const status = document.getElementById("status");
    if (status) status.textContent = "❌ WASM 加载失败";
  });

<template>
  <div class="app-container ofd-viewer-container">
    <div class="toolbar">
      <el-upload
        ref="upload"
        accept=".ofd"
        action=""
        :on-change="handleFileChange"
        :auto-upload="false"
        :show-file-list="false"
      >
        <el-button type="primary" icon="el-icon-upload2">选择OFD文件</el-button>
      </el-upload>

      <div class="toolbar-actions" v-if="totalPages > 0">
        <span class="page-info">共 {{ totalPages }} 页</span>
        <span class="file-name">{{ fileName }}</span>
      </div>

      <span class="status-text">{{ statusText }}</span>
    </div>

    <div
      class="viewer-wrapper"
      v-loading="loading"
      element-loading-text="解析中..."
    >
      <div id="ofd-container" ref="ofdContainer" class="ofd-container">
        <el-empty
          v-if="!loading && totalPages === 0"
          description="请上传OFD文件进行预览，支持拖拽上传"
        ></el-empty>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: "OfdViewer",
  data() {
    return {
      loading: false,
      fileName: "",
      totalPages: 0,
      statusText: "加载WASM中...",
      viewer: null,
      wasmReady: false,
    };
  },
  mounted() {
    this.initWasm();
    this.setupDragDrop();
  },
  beforeDestroy() {
    if (this.viewer) {
      this.viewer.destroy();
    }
  },
  methods: {
    async initWasm() {
      try {
        // 动态加载 wasm_exec.js
        await this.loadScript("/static/ofdwasm/wasm_exec.js");
        // 动态加载 ofd-viewer.js
        await this.loadScript("/static/ofdwasm/ofd-viewer.js");

        // 创建 OFDViewer 实例
        this.viewer = new window.OFDViewer(this.$refs.ofdContainer, {
          scale: 3.78,
          initialPages: 3,
          wasmPath: "/static/ofdwasm/ofd.wasm",
          onReady: () => {
            this.wasmReady = true;
            this.statusText = "就绪，请选择文件";
          },
          onLoad: ({ pageCount }) => {
            this.totalPages = pageCount;
            this.statusText = `已加载 ${pageCount} 页`;
            this.loading = false;
          },
          onError: (err) => {
            this.statusText = `错误: ${err.message}`;
            this.loading = false;
            this.$message.error("OFD解析失败: " + err.message);
          },
          onPageRender: (index) => {
            console.log(`页面 ${index + 1} 渲染完成`);
          },
        });

        await this.viewer.init();
      } catch (error) {
        console.error("WASM初始化失败:", error);
        this.statusText = "WASM加载失败";
        this.$message.error("OFD组件初始化失败");
      }
    },

    loadScript(src) {
      return new Promise((resolve, reject) => {
        if (document.querySelector(`script[src="${src}"]`)) {
          resolve();
          return;
        }
        const script = document.createElement("script");
        script.src = src;
        script.onload = resolve;
        script.onerror = reject;
        document.head.appendChild(script);
      });
    },

    async handleFileChange(file) {
      if (!file || !file.raw) return;
      if (!this.wasmReady) {
        this.$message.warning("WASM尚未加载完成，请稍候");
        return;
      }

      this.loading = true;
      this.fileName = file.name;
      this.statusText = "解析中...";

      try {
        await this.viewer.loadFile(file.raw);
        this.$message.success("OFD文件加载成功");
      } catch (error) {
        console.error("OFD解析错误:", error);
      }
    },

    setupDragDrop() {
      this.$nextTick(() => {
        const container = this.$refs.ofdContainer;
        if (!container) return;

        container.addEventListener("dragover", (e) => {
          e.preventDefault();
          e.stopPropagation();
        });

        container.addEventListener("drop", async (e) => {
          e.preventDefault();
          e.stopPropagation();

          const file = e.dataTransfer.files[0];
          if (file && file.name.toLowerCase().endsWith(".ofd")) {
            if (!this.wasmReady) {
              this.$message.warning("WASM尚未加载完成，请稍候");
              return;
            }

            this.loading = true;
            this.fileName = file.name;
            this.statusText = "解析中...";

            try {
              await this.viewer.loadFile(file);
              this.$message.success("OFD文件加载成功");
            } catch (error) {
              console.error("OFD解析错误:", error);
            }
          } else {
            this.$message.warning("请上传.ofd格式文件");
          }
        });
      });
    },
  },
};
</script>

<style lang="scss" scoped>
.ofd-viewer-container {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 84px);
  background: #f0f0f0;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 16px 20px;
  background: #fff;
  border-bottom: 1px solid #e4e7ed;
  flex-wrap: wrap;

  .toolbar-actions {
    display: flex;
    align-items: center;
    gap: 16px;
  }

  .page-info {
    font-size: 14px;
    color: #606266;
  }

  .file-name {
    font-size: 14px;
    color: #409eff;
    max-width: 300px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .status-text {
    margin-left: auto;
    font-size: 14px;
    color: #909399;
  }
}

.viewer-wrapper {
  flex: 1;
  overflow: hidden;
}

.ofd-container {
  width: 100%;
  height: 100%;
  overflow: auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 20px;
  gap: 20px;
}
</style>

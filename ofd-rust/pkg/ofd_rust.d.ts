/* tslint:disable */
/* eslint-disable */

/**
 * WASM 导出的 OFD 解析器
 */
export class OFDParser {
    free(): void;
    [Symbol.dispose](): void;
    /**
     * 获取文档信息
     */
    get_doc_info(): any;
    /**
     * 获取文件列表
     */
    get_files(): any;
    /**
     * 获取字体信息
     */
    get_fonts(): any;
    /**
     * 获取页数
     */
    get_page_count(): number;
    /**
     * 获取页面尺寸
     */
    get_page_size(index: number): any;
    /**
     * 创建解析器
     */
    constructor(data: Uint8Array);
    /**
     * 解析OFD文件
     */
    parse(): any;
    /**
     * 读取OFD包内文件的文本内容（用于查看XML）
     */
    read_file_text(name: string): any;
    /**
     * 渲染页面（Canvas 模式）
     */
    render_page(index: number): any;
    /**
     * 渲染页面为 SVG
     */
    render_page_svg(index: number): any;
    /**
     * 渲染页面为 SVG（带缩放比例）
     */
    render_page_svg_scaled(index: number, zoom: number): any;
}

/**
 * 初始化 panic hook（用于调试）
 */
export function init(): void;

export type InitInput = RequestInfo | URL | Response | BufferSource | WebAssembly.Module;

export interface InitOutput {
    readonly memory: WebAssembly.Memory;
    readonly __wbg_ofdparser_free: (a: number, b: number) => void;
    readonly init: () => void;
    readonly ofdparser_get_doc_info: (a: number) => any;
    readonly ofdparser_get_files: (a: number) => any;
    readonly ofdparser_get_fonts: (a: number) => any;
    readonly ofdparser_get_page_count: (a: number) => number;
    readonly ofdparser_get_page_size: (a: number, b: number) => any;
    readonly ofdparser_new: (a: number, b: number) => [number, number, number];
    readonly ofdparser_parse: (a: number) => [number, number, number];
    readonly ofdparser_read_file_text: (a: number, b: number, c: number) => any;
    readonly ofdparser_render_page: (a: number, b: number) => any;
    readonly ofdparser_render_page_svg: (a: number, b: number) => any;
    readonly ofdparser_render_page_svg_scaled: (a: number, b: number, c: number) => any;
    readonly __wbindgen_exn_store: (a: number) => void;
    readonly __externref_table_alloc: () => number;
    readonly __wbindgen_externrefs: WebAssembly.Table;
    readonly __wbindgen_malloc: (a: number, b: number) => number;
    readonly __externref_table_dealloc: (a: number) => void;
    readonly __wbindgen_realloc: (a: number, b: number, c: number, d: number) => number;
    readonly __wbindgen_start: () => void;
}

export type SyncInitInput = BufferSource | WebAssembly.Module;

/**
 * Instantiates the given `module`, which can either be bytes or
 * a precompiled `WebAssembly.Module`.
 *
 * @param {{ module: SyncInitInput }} module - Passing `SyncInitInput` directly is deprecated.
 *
 * @returns {InitOutput}
 */
export function initSync(module: { module: SyncInitInput } | SyncInitInput): InitOutput;

/**
 * If `module_or_path` is {RequestInfo} or {URL}, makes a request and
 * for everything else, calls `WebAssembly.instantiate` directly.
 *
 * @param {{ module_or_path: InitInput | Promise<InitInput> }} module_or_path - Passing `InitInput` directly is deprecated.
 *
 * @returns {Promise<InitOutput>}
 */
export default function __wbg_init (module_or_path?: { module_or_path: InitInput | Promise<InitInput> } | InitInput | Promise<InitInput>): Promise<InitOutput>;

export interface OFDViewer {
  parseAndRender(file: File): Promise<void>;
  showXmlModal(): void;
  hideXmlModal(): void;
  zoomIn(): void;
  zoomOut(): void;
  resetZoom(): void;
  setZoom(zoom: number): void;
  startSealPlacement(sealBase64: string, mimeType?: string): void;
  startSealPlacementWithMock(): void;
  printOFD(): void;
  stopSealPlacement(): void;
  bindDomEvents(): void;
  destroy(): void;
}

export interface OFDViewerOptions {
  document?: Document;
  window?: Window;
  wasmModuleOrPath?: RequestInfo | URL | Response | BufferSource | WebAssembly.Module;
  baseScale?: number;
  zoomStep?: number;
  zoomMin?: number;
  zoomMax?: number;
  initialPages?: number;
  preloadThreshold?: number;
  updateStatus?: (message: string) => void;
}

export function createOFDViewer(options?: OFDViewerOptions): OFDViewer;
export function initOFDViewer(options?: OFDViewerOptions): Promise<OFDViewer>;

import initWasmImpl, { OFDParser, initSync } from "../pkg/ofd_rust.js";
import { OFDViewer as BaseOFDViewer } from "./ofd-viewer.js";

const DEFAULT_WASM_URL = new URL("./ofd-viewer.wasm", import.meta.url);

function usesLegacyInitSignature(initCandidate, parserCandidate) {
  return (
    typeof initCandidate === "function" && typeof parserCandidate === "function"
  );
}

export class OFDViewer extends BaseOFDViewer {
  async init(initOrWasmUrl, ParserClass, wasmUrl) {
    if (usesLegacyInitSignature(initOrWasmUrl, ParserClass)) {
      return super.init(initOrWasmUrl, ParserClass, wasmUrl);
    }

    const resolvedWasmUrl =
      initOrWasmUrl === undefined ? DEFAULT_WASM_URL : initOrWasmUrl;

    return super.init(initWasm, OFDParser, resolvedWasmUrl);
  }
}

export async function initWasm(moduleOrPath = DEFAULT_WASM_URL) {
  return initWasmImpl(moduleOrPath);
}

export { DEFAULT_WASM_URL as defaultWasmUrl, initSync, OFDParser };

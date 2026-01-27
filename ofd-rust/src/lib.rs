//! OFD Rust WASM 库
//! 
//! 用于解析和渲染 OFD (Open Fixed-layout Document) 文档

mod types;
mod page;
mod parser;
mod render;

use wasm_bindgen::prelude::*;
use serde_json;

pub use types::*;
pub use page::*;
pub use parser::*;
pub use render::*;

/// WASM 导出的 OFD 解析器
#[wasm_bindgen]
pub struct OFDParser {
    parser: Parser,
}

#[wasm_bindgen]
impl OFDParser {
    /// 创建解析器
    #[wasm_bindgen(constructor)]
    pub fn new(data: &[u8]) -> Result<OFDParser, JsValue> {
        let parser = Parser::new(data.to_vec())
            .map_err(|e| JsValue::from_str(&e))?;
        Ok(OFDParser { parser })
    }

    /// 解析OFD文件
    pub fn parse(&mut self) -> Result<JsValue, JsValue> {
        let result = self.parser.parse()
            .map_err(|e| JsValue::from_str(&e))?;
        
        let json = serde_json::json!({
            "files": result.files,
            "pageCount": result.page_count,
            "error": result.error,
        });
        
        serde_wasm_bindgen::to_value(&json)
            .map_err(|e| JsValue::from_str(&format!("{:?}", e)))
    }

    /// 获取页数
    pub fn get_page_count(&self) -> usize {
        self.parser.get_page_count()
    }

    /// 获取页面尺寸
    pub fn get_page_size(&self, index: usize) -> JsValue {
        let (w, h) = self.parser.get_page_size(index);
        serde_wasm_bindgen::to_value(&serde_json::json!({
            "width": w,
            "height": h,
        })).unwrap_or(JsValue::NULL)
    }

    /// 渲染页面
    pub fn render_page(&mut self, index: usize) -> JsValue {
        let result = self.parser.render_page(index);
        serde_wasm_bindgen::to_value(&result).unwrap_or(JsValue::NULL)
    }

    /// 获取字体信息
    pub fn get_fonts(&mut self) -> JsValue {
        let fonts = self.parser.get_fonts();
        serde_wasm_bindgen::to_value(&fonts).unwrap_or(JsValue::NULL)
    }

    /// 获取文件列表
    pub fn get_files(&self) -> JsValue {
        let files = self.parser.get_files();
        serde_wasm_bindgen::to_value(&files).unwrap_or(JsValue::NULL)
    }

    /// 获取文档信息
    pub fn get_doc_info(&self) -> JsValue {
        if let Some(info) = self.parser.get_doc_info() {
            serde_wasm_bindgen::to_value(&serde_json::json!({
                "docId": info.doc_id,
                "title": info.title,
                "author": info.author,
                "creationDate": info.creation_date,
                "creator": info.creator,
            })).unwrap_or(JsValue::NULL)
        } else {
            JsValue::NULL
        }
    }
}

/// 初始化 panic hook（用于调试）
#[wasm_bindgen(start)]
pub fn init() {
    #[cfg(feature = "console_error_panic_hook")]
    console_error_panic_hook::set_once();
}

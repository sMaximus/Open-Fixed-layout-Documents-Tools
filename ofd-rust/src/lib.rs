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
        
        let obj = js_sys::Object::new();
        js_sys::Reflect::set(&obj, &"files".into(), 
            &serde_wasm_bindgen::to_value(&result.files).unwrap_or(JsValue::NULL))?;
        js_sys::Reflect::set(&obj, &"pageCount".into(), &JsValue::from(result.page_count as u32))?;
        js_sys::Reflect::set(&obj, &"error".into(), 
            &result.error.map(|e| JsValue::from_str(&e)).unwrap_or(JsValue::NULL))?;
        
        Ok(obj.into())
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
        
        // 调试输出
        web_sys::console::log_1(&format!("Rust render_page: index={}, width={}, height={}", 
            result.page_index, result.width, result.height).into());
        
        // 手动构建 JS 对象以确保正确的属性名
        let obj = js_sys::Object::new();
        let _ = js_sys::Reflect::set(&obj, &"pageIndex".into(), &JsValue::from(result.page_index as u32));
        let _ = js_sys::Reflect::set(&obj, &"width".into(), &JsValue::from(result.width));
        let _ = js_sys::Reflect::set(&obj, &"height".into(), &JsValue::from(result.height));
        let _ = js_sys::Reflect::set(&obj, &"canvasData".into(), 
            &serde_wasm_bindgen::to_value(&result.canvas_data).unwrap_or(JsValue::NULL));
        let _ = js_sys::Reflect::set(&obj, &"textLayer".into(), 
            &serde_wasm_bindgen::to_value(&result.text_layer).unwrap_or(JsValue::NULL));
        if let Some(err) = result.error {
            let _ = js_sys::Reflect::set(&obj, &"error".into(), &JsValue::from_str(&err));
        }
        
        obj.into()
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

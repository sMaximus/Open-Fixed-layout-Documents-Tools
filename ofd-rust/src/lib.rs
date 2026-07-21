//! OFD Rust WASM 库
//!
//! 用于解析和渲染 OFD (Open Fixed-layout Document) 文档

mod font;
mod page;
mod parser;
mod render;
mod stamp;
mod svg_render;
mod types;

use serde_json;
use wasm_bindgen::prelude::*;

pub use font::*;
pub use page::*;
pub use parser::*;
pub use render::*;
pub use svg_render::*;
pub use types::*;

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
        let parser = Parser::new(data.to_vec()).map_err(|e| JsValue::from_str(&e))?;
        Ok(OFDParser { parser })
    }

    /// 解析OFD文件
    pub fn parse(&mut self) -> Result<JsValue, JsValue> {
        let result = self.parser.parse().map_err(|e| JsValue::from_str(&e))?;

        let obj = js_sys::Object::new();
        js_sys::Reflect::set(
            &obj,
            &"files".into(),
            &serde_wasm_bindgen::to_value(&result.files).unwrap_or(JsValue::NULL),
        )?;
        js_sys::Reflect::set(
            &obj,
            &"pageCount".into(),
            &JsValue::from(result.page_count as u32),
        )?;
        js_sys::Reflect::set(
            &obj,
            &"error".into(),
            &result
                .error
                .map(|e| JsValue::from_str(&e))
                .unwrap_or(JsValue::NULL),
        )?;

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
        }))
        .unwrap_or(JsValue::NULL)
    }

    /// 渲染页面（Canvas 模式）
    pub fn render_page(&mut self, index: usize) -> JsValue {
        let result = self.parser.render_page(index);

        let obj = js_sys::Object::new();
        let _ = js_sys::Reflect::set(
            &obj,
            &"pageIndex".into(),
            &JsValue::from(result.page_index as u32),
        );
        let _ = js_sys::Reflect::set(&obj, &"width".into(), &JsValue::from(result.width));
        let _ = js_sys::Reflect::set(&obj, &"height".into(), &JsValue::from(result.height));
        let _ = js_sys::Reflect::set(
            &obj,
            &"canvasData".into(),
            &serde_wasm_bindgen::to_value(&result.canvas_data).unwrap_or(JsValue::NULL),
        );
        let _ = js_sys::Reflect::set(
            &obj,
            &"textLayer".into(),
            &serde_wasm_bindgen::to_value(&result.text_layer).unwrap_or(JsValue::NULL),
        );
        if let Some(err) = result.error {
            let _ = js_sys::Reflect::set(&obj, &"error".into(), &JsValue::from_str(&err));
        }

        obj.into()
    }

    /// 渲染页面为 SVG
    pub fn render_page_svg(&mut self, index: usize) -> JsValue {
        let result = self.parser.render_page_svg(index);
        serde_wasm_bindgen::to_value(&result).unwrap_or(JsValue::NULL)
    }

    /// 渲染页面为 SVG（带缩放比例）
    pub fn render_page_svg_scaled(&mut self, index: usize, zoom: f64) -> JsValue {
        let result = self.parser.render_page_svg_with_zoom(index, zoom);
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

    /// 读取OFD包内文件的文本内容（用于查看XML）
    pub fn read_file_text(&self, name: &str) -> JsValue {
        match self.parser.read_file(name) {
            Ok(data) => JsValue::from_str(&String::from_utf8_lossy(&data)),
            Err(e) => JsValue::from_str(&format!("读取失败: {}", e)),
        }
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
            }))
            .unwrap_or(JsValue::NULL)
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

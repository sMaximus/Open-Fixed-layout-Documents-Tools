//! SVG 渲染模块
//! 将 OFD 页面渲染为 SVG 字符串，文字转为矢量路径（text-to-path）实现高清渲染

use base64::{engine::general_purpose::STANDARD as BASE64, Engine as _};
use std::fmt::Write;

use crate::page::*;
use crate::parser::*;
use crate::render::*;

/// 字形轮廓构建器：将 ttf-parser 的回调转为 SVG path d 属性
struct GlyphPathBuilder {
    path: String,
}

impl GlyphPathBuilder {
    fn new() -> Self {
        GlyphPathBuilder {
            path: String::new(),
        }
    }
}

impl ttf_parser::OutlineBuilder for GlyphPathBuilder {
    fn move_to(&mut self, x: f32, y: f32) {
        let _ = write!(self.path, "M{:.4},{:.4} ", x, -y);
    }
    fn line_to(&mut self, x: f32, y: f32) {
        let _ = write!(self.path, "L{:.4},{:.4} ", x, -y);
    }
    fn quad_to(&mut self, x1: f32, y1: f32, x: f32, y: f32) {
        let _ = write!(self.path, "Q{:.4},{:.4} {:.4},{:.4} ", x1, -y1, x, -y);
    }
    fn curve_to(&mut self, x1: f32, y1: f32, x2: f32, y2: f32, x: f32, y: f32) {
        let _ = write!(
            self.path,
            "C{:.4},{:.4} {:.4},{:.4} {:.4},{:.4} ",
            x1, -y1, x2, -y2, x, -y
        );
    }
    fn close(&mut self) {
        self.path.push_str("Z ");
    }
}

/// 从字体数据中提取字符的 SVG path
/// 返回 (path_d, advance_width) — advance_width 是字体单位的前进宽度
fn glyph_to_svg_path(font_data: &[u8], ch: char) -> Option<(String, f64)> {
    let face = ttf_parser::Face::parse(font_data, 0).ok()?;
    let glyph_id = face.glyph_index(ch)?;
    let mut builder = GlyphPathBuilder::new();
    let _bbox = face.outline_glyph(glyph_id, &mut builder)?;
    if builder.path.is_empty() {
        return None;
    }
    let advance = face
        .glyph_hor_advance(glyph_id)
        .map(|a| a as f64)
        .unwrap_or(0.0);
    Some((builder.path, advance))
}

/// 通过 GlyphID 直接提取字形的 SVG path（用于 CGTransform 映射）
fn glyph_id_to_svg_path(font_data: &[u8], glyph_id: u16) -> Option<(String, f64)> {
    let face = ttf_parser::Face::parse(font_data, 0).ok()?;
    let gid = ttf_parser::GlyphId(glyph_id);
    let mut builder = GlyphPathBuilder::new();
    let _bbox = face.outline_glyph(gid, &mut builder)?;
    if builder.path.is_empty() {
        return None;
    }
    let advance = face.glyph_hor_advance(gid).map(|a| a as f64).unwrap_or(0.0);
    Some((builder.path, advance))
}

/// SVG 渲染结果
#[derive(Debug, Default, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SVGRenderResult {
    pub page_index: usize,
    pub width: f64,
    pub height: f64,
    /// 缩放比例（前端用于计算容器尺寸）
    #[serde(skip_serializing_if = "Option::is_none")]
    pub zoom: Option<f64>,
    /// 高清倍率，前端需要用此值缩放文本蒙层坐标
    #[serde(skip_serializing_if = "Option::is_none")]
    pub hi_dpi: Option<f64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub svg: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub text_overlay: Option<Vec<TextOverlayItem>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub stamp_debug: Option<Vec<StampDebugInfo>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub html_texts: Option<Vec<HtmlTextItem>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

/// 文本蒙层项
#[derive(Debug, Default, Clone, serde::Serialize, serde::Deserialize)]
pub struct TextOverlayItem {
    pub text: String,
    pub x: f64,
    pub y: f64,
    pub width: f64,
    pub height: f64,
}

/// HTML 文字渲染项（用于无嵌入字体的文字，HTML 渲染比 SVG 更清晰）
#[derive(Debug, Default, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HtmlTextItem {
    pub text: String,
    pub x: f64,
    pub y: f64,
    pub font_size: f64,
    pub font_family: String,
    pub color: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub font_weight: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub font_style: Option<String>,
}

use std::sync::atomic::{AtomicUsize, Ordering};

static SVG_GRADIENT_COUNTER: AtomicUsize = AtomicUsize::new(0);
static SVG_PATTERN_COUNTER: AtomicUsize = AtomicUsize::new(0);

fn next_gradient_id() -> String {
    let id = SVG_GRADIENT_COUNTER.fetch_add(1, Ordering::Relaxed);
    format!("grad_{}", id)
}

fn next_pattern_id() -> String {
    let id = SVG_PATTERN_COUNTER.fetch_add(1, Ordering::Relaxed);
    format!("pat_{}", id)
}

/// 获取中文字体名对应的系统英文别名
pub fn get_font_aliases(name: &str) -> Vec<&'static str> {
    // 去掉竖排字体前缀 @
    let name = name.strip_prefix('@').unwrap_or(name);
    match name {
        "仿宋" | "仿宋_GB2312" => {
            vec!["FangSong", "FangSong_GB2312", "STFangsong", "STFangSong"]
        }
        "黑体" => vec!["SimHei", "STHeiti", "Heiti SC"],
        "宋体" | "SimSun" => vec!["SimSun", "STSong", "Songti SC", "NSimSun"],
        "楷体" | "楷体_GB2312" => vec!["KaiTi", "KaiTi_GB2312", "STKaiti", "Kaiti SC"],
        "隶书" => vec!["LiSu", "STLiti", "Baoli SC"],
        "幼圆" => vec!["YouYuan"],
        "华文仿宋" | "STFangsong" => vec!["STFangsong", "STFangSong", "FangSong"],
        "华文黑体" | "STHeiti" => vec!["STHeiti", "Heiti SC"],
        "华文宋体" | "STSong" => vec!["STSong", "Songti SC"],
        "华文楷体" | "STKaiti" => vec!["STKaiti", "Kaiti SC"],
        "华文中宋" => vec!["STZhongsong"],
        "华文细黑" => vec!["STXihei", "Heiti SC"],
        "微软雅黑" => vec!["Microsoft YaHei", "PingFang SC"],
        "新宋体" | "NSimSun" => vec!["NSimSun", "Songti SC", "STSong"],
        _ => vec![],
    }
}

/// 获取通用的中文 fallback 字体链（适配 Windows/macOS/Linux）
pub fn cjk_fallback_fonts() -> &'static str {
    "STSong, Songti SC, SimSun, STHeiti, Heiti SC, PingFang SC, Microsoft YaHei, sans-serif"
}

/// 去掉竖排字体前缀 @，返回 (去掉@的名称, 是否竖排)
pub fn strip_vertical_prefix(name: &str) -> (&str, bool) {
    if let Some(stripped) = name.strip_prefix('@') {
        (stripped, true)
    } else {
        (name, false)
    }
}

/// XML 转义
fn xml_escape(s: &str) -> String {
    s.replace('&', "&amp;")
        .replace('<', "&lt;")
        .replace('>', "&gt;")
        .replace('"', "&quot;")
}

impl Parser {
    /// 渲染页面为 SVG
    pub fn render_page_svg(&mut self, page_index: usize) -> SVGRenderResult {
        self.render_page_svg_with_zoom(page_index, 1.0)
    }

    /// 渲染页面为 SVG（带缩放比例）
    pub fn render_page_svg_with_zoom(&mut self, page_index: usize, zoom: f64) -> SVGRenderResult {
        let zoom = if zoom <= 0.0 { 1.0 } else { zoom };
        let mut result = SVGRenderResult {
            page_index,
            ..Default::default()
        };

        let page_count = self.get_page_count();
        if page_index >= page_count {
            result.error = Some("页面不存在".to_string());
            return result;
        }

        let page_path = match self.get_page_path(page_index) {
            Some(p) => p,
            None => {
                result.error = Some("无法获取页面路径".to_string());
                return result;
            }
        };

        let page_data = match self.read_file(&page_path) {
            Ok(d) => d,
            Err(e) => {
                result.error = Some(format!("读取页面失败: {}", e));
                return result;
            }
        };

        let page_xml = Self::remove_namespace_prefix(&String::from_utf8_lossy(&page_data));
        let page: Page = match quick_xml::de::from_str(&page_xml) {
            Ok(p) => p,
            Err(e) => {
                result.error = Some(format!("解析页面失败: {}", e));
                return result;
            }
        };

        let (width, height) = self.get_page_size_from_page(&page);
        let hi_dpi = 10.0_f64; // 高清倍率：内部以 10x 分辨率渲染，CSS 缩回 1x 显示，文字边缘如刀锋般锐利
        let scale = 3.78_f64 * hi_dpi; // mm → px（高清）
        let px_w = width * scale;
        let px_h = height * scale;
        result.width = width;
        result.height = height;

        self.load_resources();

        let mut svg_parts: Vec<String> = Vec::new();
        let mut text_overlay: Vec<TextOverlayItem> = Vec::new();
        let mut html_texts: Vec<HtmlTextItem> = Vec::new();

        // 嵌入字体 @font-face 到 SVG 内部
        let mut font_css = String::new();
        for (id, _font) in &self.fonts {
            if let Some(font_data) = self.font_files.get(id) {
                if let Some(mime_type) = detect_font_mime(font_data) {
                    let b64 = BASE64.encode(font_data);
                    let _ = write!(font_css,
                        "@font-face {{ font-family: 'OFD_Font_{}'; src: url('data:{};base64,{}'); }}\n",
                        id, mime_type, b64
                    );
                }
            }
        }
        if !font_css.is_empty() {
            svg_parts.push(format!("<defs><style>{}</style></defs>", font_css));
        }

        // 白色背景
        svg_parts.push(format!(
            r#"<rect width="{:.2}" height="{:.2}" fill="white"/>"#,
            px_w, px_h
        ));

        // 渲染模板层
        self.render_template_svg(
            &mut svg_parts,
            &mut text_overlay,
            &page,
            scale,
            width,
            height,
        );

        let stamps = self.collect_page_stamps(page_index);
        if !stamps.is_empty() {
            result.stamp_debug = Some(stamps.iter().map(|stamp| stamp.debug_info()).collect());
        }

        // 获取页面层
        let layers = self.get_page_layers(&page);

        for layer in layers.iter() {
            // Resolve layer-level DrawParam
            let layer_dp = if !layer.draw_param.is_empty() {
                self.draw_params.get(&layer.draw_param).cloned()
            } else {
                None
            };

            // Keep original object order to preserve z-order (e.g. strikethrough paths).
            for obj in &layer.objects {
                self.render_layer_object_svg(
                    &mut svg_parts,
                    &mut text_overlay,
                    &mut html_texts,
                    obj,
                    scale,
                    width,
                    height,
                    &layer_dp,
                    true,
                );
            }
        }

        // 注释
        self.load_page_annot_svg(
            &mut svg_parts,
            &mut text_overlay,
            scale,
            page_index,
            width,
            height,
        );

        // 签章最后渲染，并使用混合模式模拟纸面盖章效果。
        self.load_stamps_svg(
            &mut svg_parts,
            &mut text_overlay,
            &stamps,
            scale,
            page_index,
            width,
            height,
        );

        // SVG 不设固定 width/height，只用 viewBox + CSS 100% 填充容器
        // 这样避免 SVG 固有尺寸与容器尺寸不一致导致的二次缩放模糊
        let svg = format!(
            r#"<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 {:.2} {:.2}" text-rendering="geometricPrecision" shape-rendering="geometricPrecision" color-interpolation="linearRGB">
{}
</svg>"#,
            px_w,
            px_h,
            svg_parts.join("\n")
        );

        result.svg = Some(svg);
        result.zoom = Some(zoom);
        result.hi_dpi = Some(hi_dpi);
        result.text_overlay = Some(text_overlay);
        if !html_texts.is_empty() {
            result.html_texts = Some(html_texts);
        }
        result
    }

    fn get_page_layers<'a>(&self, page: &'a Page) -> Vec<&'a Layer> {
        let mut layers: Vec<&Layer> = page.content.layer.iter().collect();
        if layers.is_empty() {
            layers = page.layer.iter().collect();
        }
        layers
    }

    fn render_layer_object_svg(
        &mut self,
        svg_parts: &mut Vec<String>,
        text_overlay: &mut Vec<TextOverlayItem>,
        html_texts: &mut Vec<HtmlTextItem>,
        obj: &LayerObject,
        scale: f64,
        page_w: f64,
        page_h: f64,
        layer_dp: &Option<DrawParam>,
        render_composites: bool,
    ) {
        match obj {
            LayerObject::PathObject(path_obj) => {
                if let Some(s) =
                    self.path_object_to_svg_with_dp(path_obj, scale, page_w, page_h, layer_dp)
                {
                    svg_parts.push(s);
                }
            }
            LayerObject::ImageObject(img) => {
                if let Some(s) = self.image_object_to_svg(img, scale) {
                    svg_parts.push(s);
                }
            }
            LayerObject::TextObject(text) => {
                let (text_svgs, overlays, html_items) = self.render_text_svg(text, scale, layer_dp);
                svg_parts.extend(text_svgs);
                text_overlay.extend(overlays);
                html_texts.extend(html_items);
            }
            LayerObject::CompositeObject(comp) if render_composites => {
                self.render_composite_object_svg(
                    svg_parts,
                    text_overlay,
                    comp,
                    scale,
                    page_w,
                    page_h,
                );
            }
            LayerObject::CompositeObject(_) => {}
            LayerObject::PageBlock(block) => {
                for child in &block.objects {
                    self.render_layer_object_svg(
                        svg_parts,
                        text_overlay,
                        html_texts,
                        child,
                        scale,
                        page_w,
                        page_h,
                        layer_dp,
                        render_composites,
                    );
                }
            }
        }
    }

    /// 渲染模板层为 SVG
    fn render_template_svg(
        &mut self,
        svg_parts: &mut Vec<String>,
        text_overlay: &mut Vec<TextOverlayItem>,
        page: &Page,
        scale: f64,
        page_w: f64,
        page_h: f64,
    ) {
        if page.template.is_empty() {
            return;
        }
        let doc = match self.document.as_ref() {
            Some(d) => d.clone(),
            None => return,
        };

        let doc_base = self
            .ofd
            .as_ref()
            .and_then(|ofd| ofd.doc_body.first())
            .map(|body| {
                let doc_root = body.doc_root.trim_start_matches('/');
                std::path::Path::new(doc_root)
                    .parent()
                    .and_then(|p| p.to_str())
                    .unwrap_or("")
                    .to_string()
            })
            .unwrap_or_default();

        let mut tpl_paths = std::collections::HashMap::new();
        for tpl in &doc.common_data.template_page {
            let tpl_loc = tpl.base_loc.trim_start_matches('/');
            tpl_paths.insert(tpl.id.clone(), format!("{}/{}", doc_base, tpl_loc));
        }

        for tpl_ref in &page.template {
            let tpl_path = match tpl_paths.get(&tpl_ref.template_id) {
                Some(p) => p.clone(),
                None => continue,
            };

            let tpl_data = match self.read_file(&tpl_path) {
                Ok(d) => d,
                Err(_) => continue,
            };

            let tpl_xml = Self::remove_namespace_prefix(&String::from_utf8_lossy(&tpl_data));
            let tpl_page: Page = match quick_xml::de::from_str(&tpl_xml) {
                Ok(p) => p,
                Err(_) => continue,
            };

            let tpl_layers = self.get_page_layers(&tpl_page);
            for layer in &tpl_layers {
                // Resolve template layer DrawParam
                let layer_dp = if !layer.draw_param.is_empty() {
                    self.draw_params.get(&layer.draw_param).cloned()
                } else {
                    None
                };

                // Keep original object order to preserve z-order (e.g. strikethrough paths).
                for obj in &layer.objects {
                    let mut ignored_html_texts = Vec::new();
                    self.render_layer_object_svg(
                        svg_parts,
                        text_overlay,
                        &mut ignored_html_texts,
                        obj,
                        scale,
                        page_w,
                        page_h,
                        &layer_dp,
                        false,
                    );
                }
            }
        }
    }

    /// 将 PathObject 转换为 SVG path 元素
    fn path_object_to_svg(
        &mut self,
        path_obj: &PathObject,
        scale: f64,
        page_w: f64,
        page_h: f64,
    ) -> Option<String> {
        self.path_object_to_svg_with_dp(path_obj, scale, page_w, page_h, &None)
    }

    fn pattern_cell_path_to_svg(&self, path_obj: &PathObject, scale: f64) -> Option<String> {
        if path_obj.abbreviated_data.is_empty() {
            return None;
        }

        let (bx, by, _, _) = parse_boundary(&path_obj.boundary);
        let ctm = if !path_obj.ctm.is_empty() {
            parse_ctm(&path_obj.ctm)
        } else {
            Vec::new()
        };
        let cmd_json =
            convert_ofd_path_to_canvas(&path_obj.abbreviated_data, scale, bx, by, &ctm, 0.0, 0.0);
        let cmds: Vec<serde_json::Value> = match serde_json::from_str(&cmd_json) {
            Ok(c) => c,
            Err(_) => return None,
        };

        let mut d = String::new();
        for c in &cmds {
            let cmd_type = c.get("cmd").and_then(|v| v.as_str()).unwrap_or("");
            match cmd_type {
                "M" => {
                    let x = c.get("x").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let y = c.get("y").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let _ = write!(d, "M{:.4},{:.4} ", x, y);
                }
                "L" => {
                    let x = c.get("x").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let y = c.get("y").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let _ = write!(d, "L{:.4},{:.4} ", x, y);
                }
                "C" => {
                    let x1 = c.get("x1").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let y1 = c.get("y1").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let x2 = c.get("x2").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let y2 = c.get("y2").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let x = c.get("x").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let y = c.get("y").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let _ = write!(
                        d,
                        "C{:.4},{:.4} {:.4},{:.4} {:.4},{:.4} ",
                        x1, y1, x2, y2, x, y
                    );
                }
                "Q" => {
                    let x1 = c.get("x1").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let y1 = c.get("y1").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let x = c.get("x").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let y = c.get("y").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let _ = write!(d, "Q{:.4},{:.4} {:.4},{:.4} ", x1, y1, x, y);
                }
                "Z" => d.push_str("Z "),
                _ => {}
            }
        }
        let path_d = d.trim();
        if path_d.is_empty() {
            return None;
        }

        let fill_color = if let Some(ref fc) = path_obj.fill_color {
            if fc.pattern.is_some() || fc.axial_shd.is_some() || fc.radial_shd.is_some() {
                // Complex fill (Pattern/gradient) inside a nested pattern cell — skip (use "none")
                "none".to_string()
            } else if !fc.value.is_empty() {
                parse_color(&fc.value)
            } else if path_obj.fill {
                "#000".to_string()
            } else {
                "none".to_string()
            }
        } else if path_obj.fill {
            "#000".to_string()
        } else {
            "none".to_string()
        };

        let mut attrs = vec![
            format!(r#"d="{}""#, path_d),
            format!(r#"fill="{}""#, fill_color),
        ];
        if path_obj.rule.eq_ignore_ascii_case("Even-Odd") {
            attrs.push(r#"fill-rule="evenodd""#.to_string());
        }

        let mut has_stroke = false;
        if let Some(ref sc) = path_obj.stroke_color {
            if !sc.value.is_empty() {
                attrs.push(format!(r#"stroke="{}""#, parse_color(&sc.value)));
                has_stroke = true;
            }
        }
        if !has_stroke && (path_obj.stroke || path_obj.line_width > 0.0) {
            attrs.push("stroke=\"#000\"".to_string());
            has_stroke = true;
        }
        if has_stroke {
            let lw = if path_obj.line_width > 0.0 {
                path_obj.line_width
            } else {
                0.353
            };
            attrs.push(format!(r#"stroke-width="{:.4}""#, lw * scale));
            if !path_obj.join.is_empty() {
                attrs.push(format!(
                    r#"stroke-linejoin="{}""#,
                    path_obj.join.to_lowercase()
                ));
            }
            if !path_obj.cap.is_empty() {
                attrs.push(format!(
                    r#"stroke-linecap="{}""#,
                    path_obj.cap.to_lowercase()
                ));
            }
        } else {
            attrs.push("stroke=\"none\"".to_string());
        }

        Some(format!("<path {}/>", attrs.join(" ")))
    }

    fn build_pattern_fill_defs(
        &mut self,
        pattern: &Pattern,
        scale: f64,
        pattern_id: &str,
        bx: f64,
        by: f64,
    ) -> Option<String> {
        let step_x_mm = if pattern.x_step > 0.0 {
            pattern.x_step
        } else {
            pattern.width
        };
        let step_y_mm = if pattern.y_step > 0.0 {
            pattern.y_step
        } else {
            pattern.height
        };
        if step_x_mm <= 0.0 || step_y_mm <= 0.0 {
            return None;
        }

        let p_ctm = if !pattern.ctm.is_empty() {
            parse_ctm(&pattern.ctm)
        } else {
            Vec::new()
        };
        let (a, b, c, d, e, f) = if p_ctm.len() >= 6 {
            (p_ctm[0], p_ctm[1], p_ctm[2], p_ctm[3], p_ctm[4], p_ctm[5])
        } else if p_ctm.len() >= 4 {
            (p_ctm[0], p_ctm[1], p_ctm[2], p_ctm[3], 0.0, 0.0)
        } else {
            (1.0, 0.0, 0.0, 1.0, 0.0, 0.0)
        };

        let pattern_w = step_x_mm * scale;
        let pattern_h = step_y_mm * scale;
        if pattern_w <= 0.0 || pattern_h <= 0.0 {
            return None;
        }

        let mut cell_parts: Vec<String> = Vec::new();

        for img in &pattern.cell_content.image_objects {
            let img_bytes = match self.load_image_lazy(&img.resource_id) {
                Some(v) => v,
                None => continue,
            };
            if img_bytes.is_empty() {
                continue;
            }

            let mime_type = if img_bytes.len() > 2 && img_bytes[0] == 0xFF && img_bytes[1] == 0xD8 {
                "image/jpeg"
            } else {
                "image/png"
            };
            let data_url = format!("data:{};base64,{}", mime_type, BASE64.encode(&img_bytes));

            let (mut ix, mut iy, mut iw, mut ih) = parse_boundary(&img.boundary);
            if !img.ctm.is_empty() {
                let ictm = parse_ctm(&img.ctm);
                if ictm.len() >= 4 {
                    iw = ictm[0].abs();
                    ih = ictm[3].abs();
                }
                if ictm.len() >= 6 {
                    ix += ictm[4];
                    iy += ictm[5];
                }
            }
            if iw <= 0.0 || ih <= 0.0 {
                continue;
            }

            let opacity_attr = if img.alpha > 0 && img.alpha < 255 {
                format!(r#" opacity="{:.4}""#, img.alpha as f64 / 255.0)
            } else {
                String::new()
            };

            cell_parts.push(format!(
                r#"<image href="{}" x="{:.4}" y="{:.4}" width="{:.4}" height="{:.4}" preserveAspectRatio="none"{}/>"#,
                data_url,
                ix * scale,
                iy * scale,
                iw * scale,
                ih * scale,
                opacity_attr
            ));
        }

        for path_obj in &pattern.cell_content.path_objects {
            if let Some(s) = self.pattern_cell_path_to_svg(path_obj, scale) {
                cell_parts.push(s);
            }
        }

        // 渲染 Pattern 内的文本对象（水印文字）
        for text_obj in &pattern.cell_content.text_objects {
            let (text_svgs, _overlays, _html) = self.render_text_svg(text_obj, scale, &None);
            cell_parts.extend(text_svgs);
        }

        if cell_parts.is_empty() {
            return None;
        }

        let pattern_transform_attr = if p_ctm.len() >= 4 {
            // Pattern 的 CTM 平移部分需要加上 Pattern 的起始位置（bx, by）
            // 因为 Pattern 的 CTM 是相对于页面坐标系的，而 Pattern 内容是相对于 bx, by 的
            let tx = (bx + e) * scale;
            let ty = (by + f) * scale;
            format!(
                r#" patternTransform="matrix({:.6},{:.6},{:.6},{:.6},{:.6},{:.6})""#,
                a, b, c, d, tx, ty
            )
        } else {
            String::new()
        };

        Some(format!(
            r#"<defs><pattern id="{}" patternUnits="userSpaceOnUse" x="0" y="0" width="{:.4}" height="{:.4}"{}>{}</pattern></defs>"#,
            pattern_id,
            pattern_w,
            pattern_h,
            pattern_transform_attr,
            cell_parts.join("\n")
        ))
    }

    /// 将 PathObject 转换为 SVG path 元素（支持 DrawParam 继承）
    fn path_object_to_svg_with_dp(
        &mut self,
        path_obj: &PathObject,
        scale: f64,
        page_w: f64,
        page_h: f64,
        layer_dp: &Option<DrawParam>,
    ) -> Option<String> {
        if path_obj.abbreviated_data.is_empty() {
            return None;
        }

        // 解析对象级别的 DrawParam，合并 Layer 级别的
        let obj_dp = if !path_obj.draw_param.is_empty() {
            self.draw_params.get(&path_obj.draw_param).cloned()
        } else {
            None
        };
        // 优先级: 对象属性 > 对象DrawParam > Layer DrawParam
        let effective_dp = obj_dp.as_ref().or(layer_dp.as_ref());

        let (bx, by, bw, bh) = parse_boundary(&path_obj.boundary);

        let ctm = if !path_obj.ctm.is_empty() {
            parse_ctm(&path_obj.ctm)
        } else {
            Vec::new()
        };

        // 使用已有的路径转换函数获取 JSON 命令
        let cmd_json = convert_ofd_path_to_canvas(
            &path_obj.abbreviated_data,
            scale,
            bx,
            by,
            &ctm,
            page_w * scale,
            page_h * scale,
        );

        // 解析 JSON 命令数组，转换为 SVG path d 属性
        let cmds: Vec<serde_json::Value> = match serde_json::from_str(&cmd_json) {
            Ok(c) => c,
            Err(_) => return None,
        };
        let is_line_only_path = {
            let mut has_line = false;
            let mut ok = true;
            for c in &cmds {
                let cmd_type = c.get("cmd").and_then(|v| v.as_str()).unwrap_or("");
                match cmd_type {
                    "M" | "Z" => {}
                    "L" => has_line = true,
                    _ => {
                        ok = false;
                        break;
                    }
                }
            }
            ok && has_line
        };

        let mut d = String::new();
        for c in &cmds {
            let cmd_type = c.get("cmd").and_then(|v| v.as_str()).unwrap_or("");
            match cmd_type {
                "M" => {
                    let x = c.get("x").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let y = c.get("y").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let _ = write!(d, "M{:.4},{:.4} ", x, y);
                }
                "L" => {
                    let x = c.get("x").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let y = c.get("y").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let _ = write!(d, "L{:.4},{:.4} ", x, y);
                }
                "C" => {
                    let x1 = c.get("x1").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let y1 = c.get("y1").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let x2 = c.get("x2").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let y2 = c.get("y2").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let x = c.get("x").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let y = c.get("y").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let _ = write!(
                        d,
                        "C{:.4},{:.4} {:.4},{:.4} {:.4},{:.4} ",
                        x1, y1, x2, y2, x, y
                    );
                }
                "Q" => {
                    let x1 = c.get("x1").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let y1 = c.get("y1").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let x = c.get("x").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let y = c.get("y").and_then(|v| v.as_f64()).unwrap_or(0.0);
                    let _ = write!(d, "Q{:.4},{:.4} {:.4},{:.4} ", x1, y1, x, y);
                }
                "Z" => {
                    d.push_str("Z ");
                }
                _ => {}
            }
        }

        let path_d = d.trim();
        if path_d.is_empty() {
            return None;
        }

        let mut attrs = Vec::new();
        attrs.push(format!(r#"d="{}""#, path_d));

        // 填充处理
        let mut fill_color = "none".to_string();
        let mut defs_svg = String::new();

        if let Some(ref fc) = path_obj.fill_color {
            // 优先级: Pattern/渐变 > 纯色值
            // 当 FillColor 同时有 Value 和 Pattern/渐变子元素时，子元素优先
            if let Some(ref pat) = fc.pattern {
                let pat_id = next_pattern_id();
                if let Some(pat_defs) = self.build_pattern_fill_defs(pat, scale, &pat_id, bx, by) {
                    defs_svg = pat_defs;
                    fill_color = format!("url(#{})", pat_id);
                } else {
                    // Pattern exists but failed to render (e.g. images not loaded)
                    // Use transparent fill instead of Value fallback to avoid solid black rectangles
                    fill_color = "none".to_string();
                }
            } else if let Some(ref axial) = fc.axial_shd {
                let grad = self.parse_axial_shd(axial, &ctm, scale);
                let grad_id = next_gradient_id();
                let gx0 = (grad.x0 + bx) * scale;
                let gy0 = (grad.y0 + by) * scale;
                let gx1 = (grad.x1 + bx) * scale;
                let gy1 = (grad.y1 + by) * scale;
                let mut stops = String::new();
                for s in &grad.stops {
                    let _ = write!(
                        stops,
                        r#"<stop offset="{:.2}" stop-color="{}"/>"#,
                        s.position, s.color
                    );
                }
                defs_svg = format!(
                    r#"<defs><linearGradient id="{}" x1="{:.4}" y1="{:.4}" x2="{:.4}" y2="{:.4}" gradientUnits="userSpaceOnUse">{}</linearGradient></defs>"#,
                    grad_id, gx0, gy0, gx1, gy1, stops
                );
                fill_color = format!("url(#{})", grad_id);
            } else if let Some(ref radial) = fc.radial_shd {
                let grad = self.parse_radial_shd(radial, &ctm, scale);
                let grad_id = next_gradient_id();
                let gx1 = (grad.x1 + bx) * scale;
                let gy1 = (grad.y1 + by) * scale;
                let r1 = grad.r1.unwrap_or(0.0) * scale;
                let mut stops = String::new();
                for s in &grad.stops {
                    let _ = write!(
                        stops,
                        r#"<stop offset="{:.2}" stop-color="{}"/>"#,
                        s.position, s.color
                    );
                }
                defs_svg = format!(
                    r#"<defs><radialGradient id="{}" cx="{:.4}" cy="{:.4}" r="{:.4}" gradientUnits="userSpaceOnUse">{}</radialGradient></defs>"#,
                    grad_id, gx1, gy1, r1, stops
                );
                fill_color = format!("url(#{})", grad_id);
            } else if !fc.value.is_empty() {
                fill_color = parse_color(&fc.value);
            }
        } else if path_obj.fill {
            // 对象没有 FillColor 但 Fill=true，从 DrawParam 继承
            if let Some(dp) = effective_dp {
                if let Some(ref fc) = dp.fill_color {
                    if !fc.value.is_empty() {
                        fill_color = parse_color(&fc.value);
                    }
                }
            }
        }
        attrs.push(format!(r#"fill="{}""#, fill_color));
        if path_obj.rule.eq_ignore_ascii_case("Even-Odd") {
            attrs.push(r#"fill-rule="evenodd""#.to_string());
        }

        // 描边
        let mut has_stroke = false;
        if let Some(ref sc) = path_obj.stroke_color {
            if !sc.value.is_empty() {
                attrs.push(format!(r#"stroke="{}""#, parse_color(&sc.value)));
                has_stroke = true;
            }
        }
        // 从 DrawParam 继承描边颜色
        if !has_stroke {
            if let Some(dp) = effective_dp {
                if let Some(ref sc) = dp.stroke_color {
                    if !sc.value.is_empty() {
                        attrs.push(format!(r#"stroke="{}""#, parse_color(&sc.value)));
                        has_stroke = true;
                    }
                }
            }
        }
        if !has_stroke && (path_obj.stroke || path_obj.line_width > 0.0) {
            attrs.push("stroke=\"#000\"".to_string());
            has_stroke = true;
        }

        let mut stroke_width_px = 0.0_f64;
        let mut thin_line_candidate = false;
        if has_stroke {
            let mut lw = path_obj.line_width;
            if lw == 0.0 {
                if let Some(dp) = effective_dp {
                    if dp.line_width > 0.0 {
                        lw = dp.line_width;
                    }
                }
            }
            if lw == 0.0 {
                // For LineWidth=0 paths, keep line decorations visible while avoiding
                // overly bold glyph-like path strokes.
                lw = if is_line_only_path { 0.353 } else { 0.265 };
            } else if !ctm.is_empty() && ctm[0] > 0.0 {
                lw *= ctm[0];
            }
            let mut sw = lw * scale;

            // Hairline decorations (e.g. strikethrough PathObject) can become invisible.
            // Keep a minimum visible width only for thin, stroke-only line decorations.
            let is_stroke_only = fill_color == "none";
            let thin_boundary_mm = bw.min(bh);
            let css_px_per_mm = 3.78_f64;
            let thin_boundary_px = thin_boundary_mm * css_px_per_mm;
            let declared_lw = path_obj.line_width;
            let very_thin_declared = (declared_lw > 0.0 && declared_lw <= 0.06)
                || (declared_lw == 0.0 && thin_boundary_px <= 0.6);
            thin_line_candidate = is_stroke_only
                && is_line_only_path
                && very_thin_declared
                && thin_boundary_px <= 1.5;
            if thin_line_candidate {
                let hi_dpi = (scale / css_px_per_mm).max(1.0);
                let min_css_px = 1.0_f64;
                let min_viewbox_px = min_css_px * hi_dpi;
                if sw < min_viewbox_px {
                    sw = min_viewbox_px;
                }
                attrs.push(r#"shape-rendering="crispEdges""#.to_string());
            }
            stroke_width_px = sw;
            attrs.push(format!(r#"stroke-width="{:.4}""#, stroke_width_px));
            // Join/Cap: 对象属性 > DrawParam
            let join = if !path_obj.join.is_empty() {
                path_obj.join.clone()
            } else if let Some(dp) = effective_dp {
                dp.join.clone()
            } else {
                String::new()
            };
            if !join.is_empty() {
                attrs.push(format!(r#"stroke-linejoin="{}""#, join.to_lowercase()));
            }
            let cap = if !path_obj.cap.is_empty() {
                path_obj.cap.clone()
            } else if let Some(dp) = effective_dp {
                dp.cap.clone()
            } else {
                String::new()
            };
            if !cap.is_empty() {
                attrs.push(format!(r#"stroke-linecap="{}""#, cap.to_lowercase()));
            }
        } else {
            attrs.push("stroke=\"none\"".to_string());
        }

        // Alpha → opacity
        if path_obj.alpha < 255 && path_obj.alpha >= 0 {
            let opacity = path_obj.alpha as f64 / 255.0;
            attrs.push(format!(r#"opacity="{:.4}""#, opacity));
        }

        // BlendMode → style="mix-blend-mode: ..."
        if !path_obj.blend_mode.is_empty() {
            let css_blend = match path_obj.blend_mode.to_lowercase().as_str() {
                "darken" => "darken",
                "multiply" => "multiply",
                "lighten" => "lighten",
                "screen" => "screen",
                "overlay" => "overlay",
                "color-dodge" | "colordodge" => "color-dodge",
                "color-burn" | "colorburn" => "color-burn",
                "hard-light" | "hardlight" => "hard-light",
                "soft-light" | "softlight" => "soft-light",
                "difference" => "difference",
                "exclusion" => "exclusion",
                "hue" => "hue",
                "saturation" => "saturation",
                "color" => "color",
                "luminosity" => "luminosity",
                _ => "normal",
            };
            if css_blend != "normal" {
                attrs.push(format!(r#"style="mix-blend-mode: {}""#, css_blend));
            }
        }

        // DashPattern → stroke-dasharray
        if !path_obj.dash_pattern.is_empty() && has_stroke {
            let dash_values: Vec<String> = path_obj
                .dash_pattern
                .split_whitespace()
                .map(|v| {
                    let val: f64 = v.parse().unwrap_or(0.0);
                    format!("{:.4}", val * scale)
                })
                .collect();
            if !dash_values.is_empty() {
                attrs.push(format!(r#"stroke-dasharray="{}""#, dash_values.join(" ")));
            }
        }

        let path_elem = format!("<path {}/>", attrs.join(" "));

        // 用嵌套 <svg> 裁剪到 Boundary 范围，防止路径超出边界
        let mut clip_x = bx * scale;
        let mut clip_y = by * scale;
        let mut clip_w = bw * scale;
        let mut clip_h = bh * scale;

        // Expand clip box by stroke width to avoid clipping thin lines on boundary edges.
        if thin_line_candidate && stroke_width_px > 0.0 {
            let clip_pad = stroke_width_px * 0.6;
            clip_x -= clip_pad;
            clip_y -= clip_pad;
            clip_w += clip_pad * 2.0;
            clip_h += clip_pad * 2.0;
        }

        let inner = if defs_svg.is_empty() {
            path_elem
        } else {
            format!("{}\n{}", defs_svg, path_elem)
        };

        if bw > 0.0 && bh > 0.0 {
            Some(format!(
                r#"<svg x="{:.4}" y="{:.4}" width="{:.4}" height="{:.4}" viewBox="{:.4} {:.4} {:.4} {:.4}" overflow="hidden">{}</svg>"#,
                clip_x, clip_y, clip_w, clip_h, clip_x, clip_y, clip_w, clip_h, inner
            ))
        } else {
            Some(inner)
        }
    }

    /// 将 ImageObject 转换为 SVG image 元素
    fn image_object_to_svg(&mut self, img: &ImageObject, scale: f64) -> Option<String> {
        let img_data = self.load_image_lazy(&img.resource_id)?;
        if img_data.is_empty() {
            return None;
        }

        let (ix, iy, iw, ih) = parse_boundary(&img.boundary);

        let mime_type = if img_data.len() > 2 && img_data[0] == 0xFF && img_data[1] == 0xD8 {
            "image/jpeg"
        } else {
            "image/png"
        };
        let data_url = format!("data:{};base64,{}", mime_type, BASE64.encode(&img_data));

        // Alpha 透明度
        let opacity_attr = if img.alpha > 0 && img.alpha < 255 {
            format!(r#" opacity="{:.4}""#, img.alpha as f64 / 255.0)
        } else {
            String::new()
        };

        // 处理 CTM 变换
        if !img.ctm.is_empty() {
            let ctm = parse_ctm(&img.ctm);
            if ctm.len() >= 4 {
                let a = ctm[0] * scale;
                let b = ctm[1] * scale;
                let c = ctm[2] * scale;
                let d = ctm[3] * scale;
                let e = if ctm.len() > 4 { ctm[4] * scale } else { 0.0 };
                let f = if ctm.len() > 5 { ctm[5] * scale } else { 0.0 };
                let px = ix * scale;
                let py = iy * scale;
                return Some(format!(
                    r#"<image href="{}" x="0" y="0" width="1" height="1" transform="matrix({:.6},{:.6},{:.6},{:.6},{:.6},{:.6})" preserveAspectRatio="none"{}/>"#,
                    data_url,
                    a,
                    b,
                    c,
                    d,
                    px + e,
                    py + f,
                    opacity_attr
                ));
            }
        }

        // 普通绘制
        let px = ix * scale;
        let py = iy * scale;
        let pw = iw * scale;
        let ph = ih * scale;

        Some(format!(
            r#"<image href="{}" x="{:.2}" y="{:.2}" width="{:.2}" height="{:.2}" preserveAspectRatio="none"{}/>"#,
            data_url, px, py, pw, ph, opacity_attr
        ))
    }

    /// 渲染文本对象为 SVG text 元素
    fn render_text_svg(
        &self,
        text: &TextObject,
        scale: f64,
        layer_dp: &Option<DrawParam>,
    ) -> (Vec<String>, Vec<TextOverlayItem>, Vec<HtmlTextItem>) {
        let mut results = Vec::new();
        let mut overlays = Vec::new();
        let html_items = Vec::new();

        let (bx, by, _bw, _bh) = parse_boundary(&text.boundary);
        let font_id = &text.font;
        let font_size = text.size;

        // 解析对象级别的 DrawParam
        let obj_dp = if !text.draw_param.is_empty() {
            self.draw_params.get(&text.draw_param).cloned()
        } else {
            None
        };
        // 优先级: 对象属性 > 对象DrawParam > Layer DrawParam
        let effective_dp = obj_dp.as_ref().or(layer_dp.as_ref());

        // 解析颜色（优先级: 对象属性 > DrawParam > 默认黑色）
        let fill_color = text
            .fill_color
            .as_ref()
            .filter(|c| !c.value.is_empty())
            .map(|c| parse_color(&c.value))
            .or_else(|| {
                effective_dp
                    .and_then(|dp| dp.fill_color.as_ref())
                    .filter(|c| !c.value.is_empty())
                    .map(|c| parse_color(&c.value))
            })
            .unwrap_or_else(|| "#000".to_string());

        let stroke_color_str = text
            .stroke_color
            .as_ref()
            .filter(|c| !c.value.is_empty())
            .map(|c| parse_color(&c.value))
            .or_else(|| {
                effective_dp
                    .and_then(|dp| dp.stroke_color.as_ref())
                    .filter(|c| !c.value.is_empty())
                    .map(|c| parse_color(&c.value))
            });

        let color = if text.stroke && !text.fill && stroke_color_str.is_some() {
            stroke_color_str.clone().unwrap()
        } else {
            fill_color.clone()
        };

        // 解析 CTM
        let ctm = if !text.ctm.is_empty() {
            parse_ctm(&text.ctm)
        } else {
            Vec::new()
        };

        let has_ctm = ctm.len() >= 4;

        let h_scale = if text.h_scale > 0.0 {
            text.h_scale
        } else {
            1.0
        };

        // 当有 CTM 时，将 CTM 的缩放因子吸收到 font_size 中
        // 避免出现超大 font-size（如 9000）+ 微小 CTM 缩放（如 0.0176）的情况
        // 这种模式在 OFD 中很常见，浏览器对超大 font-size 渲染不佳
        let ctm_scale_y = if has_ctm {
            let sy = (ctm[2] * ctm[2] + ctm[3] * ctm[3]).sqrt();
            if sy > 0.0001 {
                sy
            } else {
                1.0
            }
        } else {
            1.0
        };
        let ctm_scale_x = if has_ctm {
            let sx = (ctm[0] * ctm[0] + ctm[1] * ctm[1]).sqrt();
            if sx > 0.0001 {
                sx
            } else {
                1.0
            }
        } else {
            1.0
        };

        let effective_font_size = font_size * ctm_scale_y;
        let char_space = font_size * h_scale;

        let font_size_px = effective_font_size * scale;

        // CTM 水平压缩比：当 CTM 的 X/Y 缩放不一致时（如 0.8 0 0 1），
        // 需要对字形水平方向额外压缩，否则文字会变宽变粗
        let ctm_h_ratio = if has_ctm && ctm_scale_y > 0.0001 {
            ctm_scale_x / ctm_scale_y
        } else {
            1.0
        };

        // 字体族
        let mut is_vertical_font = false;
        let font_family = if let Some(font) = self.fonts.get(font_id) {
            let has_file = self.font_files.contains_key(font_id);
            if font.font_name.starts_with('@') || font.family_name.starts_with('@') {
                is_vertical_font = true;
            }
            let mut families = Vec::new();
            if has_file {
                families.push(format!("'OFD_Font_{}'", font_id));
            }
            if !font.font_name.is_empty() {
                let (clean_name, _) = strip_vertical_prefix(&font.font_name);
                families.push(format!("'{}'", clean_name));
                for alias in get_font_aliases(&font.font_name) {
                    families.push(format!("'{}'", alias));
                }
            }
            if !font.family_name.is_empty() && font.family_name != font.font_name {
                let (clean_name, _) = strip_vertical_prefix(&font.family_name);
                families.push(format!("'{}'", clean_name));
                for alias in get_font_aliases(&font.family_name) {
                    let a = format!("'{}'", alias);
                    if !families.contains(&a) {
                        families.push(a);
                    }
                }
            }
            families.push(cjk_fallback_fonts().to_string());
            families.join(", ")
        } else {
            cjk_fallback_fonts().to_string()
        };

        // 描边属性（CTM 缩放影响描边线宽）
        let ctm_stroke_scale = if has_ctm {
            (ctm_scale_x * ctm_scale_y).sqrt()
        } else {
            1.0
        };
        let line_width_px = if text.stroke && text.line_width > 0.0 {
            text.line_width * ctm_stroke_scale * scale
        } else {
            0.0
        };

        // Alpha 透明度
        let opacity = if text.alpha > 0 && text.alpha < 255 {
            text.alpha as f64 / 255.0
        } else {
            1.0
        };
        let opacity_attr = if opacity < 1.0 {
            format!(r#" opacity="{:.4}""#, opacity)
        } else {
            String::new()
        };

        // 文字渲染策略：所有坐标和字号都预乘 scale 转为 px
        // 有 CTM 时：缩放因子已吸收到 font_size_px，CTM 只保留旋转/剪切
        // 无 CTM 时：直接输出绝对 px 坐标的 <text>
        if has_ctm {
            let a = ctm[0];
            let b = ctm[1];
            let c = ctm[2];
            let d = ctm[3];
            let e = if ctm.len() > 4 { ctm[4] } else { 0.0 };
            let f = if ctm.len() > 5 { ctm[5] } else { 0.0 };
            // 归一化 CTM：去掉缩放因子，只保留旋转/剪切
            let na = a / ctm_scale_x;
            let nb = b / ctm_scale_x;
            let nc = c / ctm_scale_y;
            let nd = d / ctm_scale_y;
            // 平移部分 (bx+e, by+f) 从 mm 转为 px
            results.push(format!(
                r#"<g transform="matrix({:.6},{:.6},{:.6},{:.6},{:.2},{:.2})"{}>"#,
                na,
                nb,
                nc,
                nd,
                (bx + e) * scale,
                (by + f) * scale,
                opacity_attr
            ));
        } else if opacity < 1.0 {
            // 无 CTM 但有透明度，用 <g> 包裹
            results.push(format!(r#"<g{}>"#, opacity_attr));
        }

        // 构建 CGTransform 字形映射：字符索引 → GlyphID
        let mut cg_glyph_map: std::collections::HashMap<usize, u16> =
            std::collections::HashMap::new();
        if !text.cg_transform.is_empty() {
            for cgt in &text.cg_transform {
                if cgt.glyphs.is_empty() {
                    continue;
                }
                let glyph_ids: Vec<u16> = cgt
                    .glyphs
                    .split_whitespace()
                    .filter_map(|s| s.parse::<u16>().ok())
                    .collect();
                let code_pos = cgt.code_position as usize;
                let code_count = if cgt.code_count > 0 {
                    cgt.code_count as usize
                } else {
                    1
                };
                let glyph_count = if cgt.glyph_count > 0 {
                    cgt.glyph_count as usize
                } else {
                    glyph_ids.len()
                };
                // 简单映射：每个 code position 对应一个 glyph
                for gi in 0..glyph_count.min(glyph_ids.len()) {
                    let char_idx = code_pos + gi.min(code_count.saturating_sub(1));
                    cg_glyph_map.insert(char_idx, glyph_ids[gi]);
                }
            }
        }

        let use_cg_transform = !cg_glyph_map.is_empty();

        for tc in &text.text_code {
            let content = &tc.content;
            if content.is_empty() && !use_cg_transform {
                continue;
            }
            let chars: Vec<char> = content.chars().collect();
            let char_count = chars.len();
            if char_count == 0 {
                continue;
            }

            let delta_x = parse_deltas(&tc.delta_x);
            let delta_y = parse_deltas(&tc.delta_y);

            // mm 空间累加位置（CTM 缩放已吸收，坐标需要同步缩放）
            let mut current_x_mm = tc.x * ctm_scale_x;
            let mut current_y_mm = tc.y * ctm_scale_y;

            for i in 0..char_count {
                let ch = chars[i];
                let is_space = ch == ' ' || ch == '\u{3000}' || ch == '\u{00A0}';
                let cg_glyph_id = cg_glyph_map.get(&i).copied();
                let should_render_char = if use_cg_transform {
                    // CGTransform 模式：有 glyph 映射就渲染，没有则按普通字符处理
                    cg_glyph_id.is_some() || !is_space
                } else {
                    !is_space
                };

                if should_render_char {
                    // 所有坐标预乘 scale 转为 SVG 内部坐标
                    let px_x = if has_ctm {
                        current_x_mm * scale
                    } else {
                        (bx + current_x_mm) * scale
                    };
                    let px_y = if has_ctm {
                        current_y_mm * scale
                    } else {
                        (by + current_y_mm) * scale
                    };

                    let fill_attr = if text.fill {
                        format!(r#"fill="{}""#, color)
                    } else {
                        r#"fill="none""#.to_string()
                    };
                    let stroke_attr = if text.stroke {
                        let sc = stroke_color_str.as_deref().unwrap_or(&color);
                        let lw = if line_width_px > 0.0 {
                            line_width_px
                        } else {
                            // 默认描边宽度：0.2 内部像素，在 10x 下约 0.02 CSS 像素
                            0.2
                        };
                        format!(r#" stroke="{}" stroke-width="{:.4}""#, sc, lw)
                    } else {
                        String::new() // 不描边时不输出 stroke 属性，避免任何干扰
                    };

                    // 尝试文字转曲：从嵌入字体提取字形轮廓
                    // 优先使用 CGTransform 的 GlyphID 映射，否则按 Unicode 查找
                    // 仅当字体没有系统别名或有 CGTransform 映射时才使用 glyph path
                    let has_system_font = self
                        .fonts
                        .get(font_id)
                        .map(|f| {
                            !get_font_aliases(&f.font_name).is_empty()
                                || !get_font_aliases(&f.family_name).is_empty()
                        })
                        .unwrap_or(false);

                    let glyph_path = if let Some(gid) = cg_glyph_id {
                        // CGTransform 指定了 GlyphID，直接按 ID 提取（优先级最高）
                        self.font_files
                            .get(font_id)
                            .and_then(|data| glyph_id_to_svg_path(data, gid))
                    } else if use_cg_transform {
                        None
                    } else if has_system_font {
                        None // 有系统字体且无 CGTransform，统一用 <text> 渲染
                    } else {
                        self.font_files
                            .get(font_id)
                            .and_then(|data| glyph_to_svg_path(data, ch))
                    };

                    if let Some((path_d, _advance)) = glyph_path {
                        // 字体坐标系：units_per_em → 需要缩放到目标字号
                        let units_per_em = self
                            .font_files
                            .get(font_id)
                            .and_then(|data| ttf_parser::Face::parse(data, 0).ok())
                            .map(|f| f.units_per_em() as f64)
                            .unwrap_or(1000.0);
                        let glyph_scale = font_size_px / units_per_em;

                        // HScale 处理 + CTM 水平压缩比
                        let sx = glyph_scale * h_scale * ctm_h_ratio;
                        let sy = glyph_scale;

                        // 字形路径的描边属性：stroke-width 需要转换到字形坐标空间
                        // 因为 <path> 上的 scale(sx,sy) 变换会同时缩放 stroke-width，
                        // 所以需要除以平均缩放因子来补偿，保持最终描边宽度正确
                        let glyph_stroke_attr = if text.stroke {
                            let sc = stroke_color_str.as_deref().unwrap_or(&color);
                            let lw = if line_width_px > 0.0 {
                                line_width_px
                            } else {
                                0.2
                            };
                            // 将像素空间的 stroke-width 转换到字形坐标空间
                            let avg_scale = ((sx.abs() + sy.abs()) / 2.0).max(0.0001);
                            let glyph_lw = lw / avg_scale;
                            format!(
                                r#" stroke="{}" stroke-width="{:.4}" stroke-linejoin="round""#,
                                sc, glyph_lw
                            )
                        } else {
                            String::new()
                        };

                        // transform: 平移到字符位置，缩放字形
                        let transform = format!(
                            r#"translate({:.4},{:.4}) scale({:.6},{:.6})"#,
                            px_x, px_y, sx, sy
                        );

                        let svg_path = format!(
                            r#"<path d="{}" {}{} transform="{}"/>"#,
                            path_d.trim(),
                            fill_attr,
                            glyph_stroke_attr,
                            transform
                        );
                        results.push(svg_path);
                    } else if cg_glyph_id.is_none() && !is_space {
                        // 回退：使用 SVG <text> 元素
                        // OFD 的 Size 是字身框高度，而 CSS/SVG font-size 是 em-box 大小
                        // 中文字体的字形通常只占 em-box 的 ~90%，所以需要缩小 font-size
                        // 以匹配 OFD 预期的视觉大小
                        let text_font_size = font_size_px * 0.90;

                        let mut weight_attr = String::new();
                        if text.weight >= 700 {
                            weight_attr = r#" font-weight="bold""#.to_string();
                        }
                        let mut italic_attr = String::new();
                        if text.italic {
                            italic_attr = r#" font-style="italic""#.to_string();
                        }

                        let escaped = xml_escape(&ch.to_string());

                        let mut char_transforms = Vec::new();
                        let combined_h_scale = h_scale * ctm_h_ratio;
                        if combined_h_scale < 1.0 - 0.001 || combined_h_scale > 1.0 + 0.001 {
                            char_transforms.push(format!(
                                "matrix({:.4},0,0,1,{:.4},0)",
                                combined_h_scale,
                                px_x * (1.0 - combined_h_scale)
                            ));
                        }
                        if is_vertical_font {
                            let cx = px_x + text_font_size * 0.5;
                            let cy = px_y - text_font_size * 0.35;
                            char_transforms.push(format!("rotate(90,{:.4},{:.4})", cx, cy));
                        }
                        let char_transform_attr = if !char_transforms.is_empty() {
                            format!(r#" transform="{}""#, char_transforms.join(" "))
                        } else {
                            String::new()
                        };

                        let svg_text = format!(
                            r#"<text x="{:.4}" y="{:.4}" font-size="{:.4}" font-family="{}"{} {}{}{}{}>{}</text>"#,
                            px_x,
                            px_y,
                            text_font_size,
                            font_family,
                            char_transform_attr,
                            fill_attr,
                            weight_attr,
                            italic_attr,
                            stroke_attr,
                            escaped
                        );
                        results.push(svg_text);
                    }
                }

                // mm 空间累加（CTM 缩放已吸收到坐标中）
                let dx = if i < delta_x.len() {
                    delta_x[i] * ctm_scale_x
                } else if i + 1 < chars.len() {
                    char_space * ctm_scale_x
                } else {
                    0.0
                };
                let dy = if i < delta_y.len() {
                    delta_y[i] * ctm_scale_y
                } else {
                    0.0
                };
                current_x_mm += dx;
                current_y_mm += dy;
            }

            // 文本蒙层
            if !chars.is_empty() {
                let txt: String = chars
                    .iter()
                    .filter(|c| **c != ' ' && **c != '\u{3000}' && **c != '\u{00A0}')
                    .collect();
                if !txt.is_empty() {
                    let first_x = tc.x;
                    let first_y = tc.y;
                    let text_w_local = current_x_mm - tc.x;
                    let font_size_ol = font_size * scale;

                    let (ox, oy) = if has_ctm {
                        let a = ctm[0];
                        let b_v = ctm[1];
                        let c_v = ctm[2];
                        let d = ctm[3];
                        let e = if ctm.len() > 4 { ctm[4] } else { 0.0 };
                        let f_v = if ctm.len() > 5 { ctm[5] } else { 0.0 };
                        (
                            (a * first_x + c_v * first_y + e + bx) * scale,
                            (b_v * first_x + d * first_y + f_v + by) * scale,
                        )
                    } else {
                        ((bx + first_x) * scale, (by + first_y) * scale)
                    };

                    let ow = if has_ctm {
                        (ctm[0] * text_w_local).abs() * scale + font_size_ol * 0.9
                    } else {
                        text_w_local * scale + font_size_ol * 0.9
                    };

                    overlays.push(TextOverlayItem {
                        text: txt,
                        x: ox,
                        y: oy - font_size_ol * 0.85,
                        width: ow,
                        height: font_size_ol * 1.2,
                    });
                }
            }
        }

        if has_ctm || opacity < 1.0 {
            results.push("</g>".to_string());
        }

        (results, overlays, html_items)
    }

    /// 渲染复合对象（CompositeObject）为 SVG
    fn render_composite_object_svg(
        &mut self,
        svg_parts: &mut Vec<String>,
        text_overlay: &mut Vec<TextOverlayItem>,
        comp: &CompositeObject,
        scale: f64,
        page_w: f64,
        page_h: f64,
    ) {
        let unit = match self.composite_units.get(&comp.resource_id) {
            Some(u) => u.clone(),
            None => return,
        };

        let (cx, cy, cw, ch) = parse_boundary(&comp.boundary);

        let page_block = match &unit.content.page_block {
            Some(pb) => pb,
            None => return,
        };

        // 计算子元素的实际包围盒，用于替代可能错误的 unit.width/height
        let (actual_w, actual_h) = self.compute_composite_bbox(page_block, scale);

        // 使用实际包围盒计算缩放，如果实际包围盒有效的话
        let (unit_w, unit_h) = if actual_w > 0.1 && actual_h > 0.1 {
            // 检查 unit 声明的宽高是否明显不合理（比如等于页面尺寸 210x297）
            let declared_seems_wrong =
                (unit.width - page_w).abs() < 1.0 && (unit.height - page_h).abs() < 1.0;
            let ratio_off = if unit.width > 0.0 && unit.height > 0.0 {
                let r1 = cw / unit.width;
                let r2 = ch / unit.height;
                // 如果声明的宽高导致缩放比极小（<0.2），说明声明值可能有误
                r1 < 0.2 || r2 < 0.2
            } else {
                true
            };
            if declared_seems_wrong || ratio_off {
                (actual_w, actual_h)
            } else {
                (unit.width, unit.height)
            }
        } else {
            (unit.width, unit.height)
        };

        let sx = if unit_w > 0.0 { cw / unit_w } else { 1.0 };
        let sy = if unit_h > 0.0 { ch / unit_h } else { 1.0 };

        // 用 SVG <g> 包裹，应用位移和缩放
        svg_parts.push(format!(
            r#"<g transform="translate({:.4},{:.4}) scale({:.6},{:.6})">"#,
            cx * scale,
            cy * scale,
            sx,
            sy
        ));

        for obj in &page_block.objects {
            self.render_composite_page_block_object_svg(
                svg_parts,
                text_overlay,
                obj,
                scale,
                page_w,
                page_h,
            );
        }

        svg_parts.push("</g>".to_string());
    }

    fn render_composite_page_block_object_svg(
        &mut self,
        svg_parts: &mut Vec<String>,
        text_overlay: &mut Vec<TextOverlayItem>,
        obj: &LayerObject,
        scale: f64,
        page_w: f64,
        page_h: f64,
    ) {
        match obj {
            LayerObject::PathObject(path_obj) => {
                if let Some(s) = self.path_object_to_svg(path_obj, scale, page_w, page_h) {
                    svg_parts.push(s);
                }
            }
            LayerObject::ImageObject(img) => {
                if let Some(s) = self.image_object_to_svg(img, scale) {
                    svg_parts.push(s);
                }
            }
            LayerObject::TextObject(text) => {
                let (text_svgs, overlays, _html_items) = self.render_text_svg(text, scale, &None);
                svg_parts.extend(text_svgs);
                text_overlay.extend(overlays);
            }
            LayerObject::PageBlock(block) => {
                for child in &block.objects {
                    self.render_composite_page_block_object_svg(
                        svg_parts,
                        text_overlay,
                        child,
                        scale,
                        page_w,
                        page_h,
                    );
                }
            }
            LayerObject::CompositeObject(_) => {
                // 不支持嵌套复合对象
            }
        }
    }

    /// 计算复合图元内所有子元素的实际包围盒（mm 空间）
    fn compute_composite_bbox(&self, page_block: &CompositePageBlock, _scale: f64) -> (f64, f64) {
        let mut min_x = f64::MAX;
        let mut min_y = f64::MAX;
        let mut max_x = f64::MIN;
        let mut max_y = f64::MIN;
        let mut has_path = false;

        // 优先使用 PathObject 的 Boundary 来确定实际内容区域
        // PathObject 定义了实际的形状（椭圆、边框等），其 Boundary 最可靠
        // TextObject 的 Boundary 在复合图元中经常被设为整个区域，不可靠
        for obj in &page_block.objects {
            let (boundary_str, is_path) = match obj {
                LayerObject::PathObject(p) => (&p.boundary, true),
                LayerObject::ImageObject(i) => (&i.boundary, false),
                _ => continue,
            };
            if boundary_str.is_empty() {
                continue;
            }
            let (bx, by, bw, bh) = parse_boundary(boundary_str);
            if bw < 0.01 || bh < 0.01 {
                continue;
            }
            if is_path {
                has_path = true;
            }
            if bx < min_x {
                min_x = bx;
            }
            if by < min_y {
                min_y = by;
            }
            if bx + bw > max_x {
                max_x = bx + bw;
            }
            if by + bh > max_y {
                max_y = by + bh;
            }
        }

        // 如果没有 PathObject，回退到所有元素（但排除明显过大的 TextObject）
        if !has_path {
            min_x = f64::MAX;
            min_y = f64::MAX;
            max_x = f64::MIN;
            max_y = f64::MIN;
            for obj in &page_block.objects {
                let boundary_str = match obj {
                    LayerObject::PathObject(p) => &p.boundary,
                    LayerObject::ImageObject(i) => &i.boundary,
                    LayerObject::TextObject(t) => &t.boundary,
                    LayerObject::CompositeObject(c) => &c.boundary,
                    LayerObject::PageBlock(_) => continue,
                };
                if boundary_str.is_empty() {
                    continue;
                }
                let (bx, by, bw, bh) = parse_boundary(boundary_str);
                if bw < 0.01 || bh < 0.01 {
                    continue;
                }
                if bx < min_x {
                    min_x = bx;
                }
                if by < min_y {
                    min_y = by;
                }
                if bx + bw > max_x {
                    max_x = bx + bw;
                }
                if by + bh > max_y {
                    max_y = by + bh;
                }
            }
        }

        if min_x < max_x && min_y < max_y {
            // 返回实际内容的宽高
            (max_x - min_x.min(0.0), max_y - min_y.min(0.0))
        } else {
            (0.0, 0.0)
        }
    }

    /// 加载页面注释为 SVG 元素
    fn load_page_annot_svg(
        &mut self,
        svg_parts: &mut Vec<String>,
        text_overlay: &mut Vec<TextOverlayItem>,
        scale: f64,
        page_index: usize,
        page_w: f64,
        page_h: f64,
    ) {
        let doc = match self.document.as_ref() {
            Some(d) => d.clone(),
            None => return,
        };
        if page_index >= doc.pages.page.len() {
            return;
        }

        let page_id = doc.pages.page[page_index].id.clone();
        let _doc_base = self
            .ofd
            .as_ref()
            .and_then(|ofd| ofd.doc_body.first())
            .map(|body| {
                let doc_root = body.doc_root.trim_start_matches('/');
                std::path::Path::new(doc_root)
                    .parent()
                    .and_then(|p| p.to_str())
                    .unwrap_or("")
                    .to_string()
            })
            .unwrap_or_default();

        // 查找 Annotations.xml
        let mut annot_index_path = String::new();
        let files_clone: Vec<String> = self.files.clone();
        for file in &files_clone {
            let lower = file.to_lowercase();
            if lower.ends_with("annotations.xml") && !lower.contains("page_") {
                annot_index_path = file.clone();
                break;
            }
        }
        if annot_index_path.is_empty() {
            return;
        }

        let index_data = match self.read_file(&annot_index_path) {
            Ok(d) => d,
            Err(_) => return,
        };

        let xml_str = Self::remove_namespace_prefix(&String::from_utf8_lossy(&index_data));

        #[derive(Debug, Default, serde::Deserialize)]
        #[serde(rename = "Annotations")]
        struct AnnotationsFile {
            #[serde(rename = "Page", default)]
            pages: Vec<AnnotPageEntry>,
        }
        #[derive(Debug, Default, serde::Deserialize)]
        struct AnnotPageEntry {
            #[serde(rename = "@PageID", default)]
            page_id: String,
            #[serde(rename = "FileLoc", default)]
            file_loc: String,
        }

        let annot_index: AnnotationsFile = match quick_xml::de::from_str(&xml_str) {
            Ok(a) => a,
            Err(_) => return,
        };

        let annot_index_dir = std::path::Path::new(&annot_index_path)
            .parent()
            .and_then(|p| p.to_str())
            .unwrap_or("");

        for ap in &annot_index.pages {
            if ap.page_id != page_id {
                continue;
            }

            let annot_file_path = format!(
                "{}/{}",
                annot_index_dir,
                ap.file_loc.trim_start_matches('/')
            );
            let annot_data = match self.read_file(&annot_file_path) {
                Ok(d) => d,
                Err(_) => continue,
            };

            let annot_xml = Self::remove_namespace_prefix(&String::from_utf8_lossy(&annot_data));
            let page_annot: PageAnnot = match quick_xml::de::from_str(&annot_xml) {
                Ok(a) => a,
                Err(_) => continue,
            };

            for annot in &page_annot.annots {
                if annot.annot_type.eq_ignore_ascii_case("Link")
                    || annot.subtype.eq_ignore_ascii_case("Link")
                {
                    continue;
                }

                let (ax, ay, _, _) = parse_boundary(&annot.appearance.boundary);

                // 用 <g> 包裹注释，偏移到注释位置
                svg_parts.push(format!(
                    r#"<g transform="translate({:.4},{:.4})">"#,
                    ax * scale,
                    ay * scale
                ));

                // 处理 PageBlock 内的对象
                for block in &annot.appearance.page_blocks {
                    for obj in &block.objects {
                        let mut ignored_html_texts = Vec::new();
                        self.render_layer_object_svg(
                            svg_parts,
                            text_overlay,
                            &mut ignored_html_texts,
                            obj,
                            scale,
                            page_w,
                            page_h,
                            &None,
                            false,
                        );
                    }
                }

                // 处理直接嵌在 Appearance 下的对象（无 PageBlock 包裹）
                for text_obj in &annot.appearance.text_objects {
                    let (text_svgs, overlays, _html) = self.render_text_svg(text_obj, scale, &None);
                    svg_parts.extend(text_svgs);
                    text_overlay.extend(overlays);
                }
                for path_obj in &annot.appearance.path_objects {
                    if let Some(s) = self.path_object_to_svg(path_obj, scale, page_w, page_h) {
                        svg_parts.push(s);
                    }
                }
                for a_img in &annot.appearance.image_objects {
                    if let Some(s) = self.image_object_to_svg(a_img, scale) {
                        svg_parts.push(s);
                    }
                }

                svg_parts.push("</g>".to_string());
            }
        }
    }

    /// 加载印章为 SVG 元素
    fn load_stamps_svg(
        &mut self,
        svg_parts: &mut Vec<String>,
        _text_overlay: &mut Vec<TextOverlayItem>,
        stamps: &[crate::stamp::ResolvedStamp],
        scale: f64,
        page_index: usize,
        _page_w: f64,
        _page_h: f64,
    ) {
        for (stamp_index, stamp) in stamps.iter().enumerate() {
            if let Some(data_url) = &stamp.image_data_url {
                let image = format!(
                    r#"<image href="{}" x="{:.2}" y="{:.2}" width="{:.2}" height="{:.2}" preserveAspectRatio="none"/>"#,
                    data_url,
                    stamp.full_rect.x * scale,
                    stamp.full_rect.y * scale,
                    stamp.full_rect.width * scale,
                    stamp.full_rect.height * scale
                );

                if stamp.has_clip {
                    let clip_id = if stamp.id.is_empty() {
                        format!("stamp_clip_{}_{}", page_index, stamp_index)
                    } else {
                        format!("stamp_clip_{}_{}", page_index, stamp.id)
                    };
                    svg_parts.push(format!(
                        r#"<clipPath id="{}"><rect x="{:.2}" y="{:.2}" width="{:.2}" height="{:.2}"/></clipPath>"#,
                        clip_id,
                        stamp.visible_rect.x * scale,
                        stamp.visible_rect.y * scale,
                        stamp.visible_rect.width * scale,
                        stamp.visible_rect.height * scale
                    ));
                    svg_parts.push(format!(
                        r#"<g class="ofd-signature-stamp" clip-path="url(#{})" style="mix-blend-mode: multiply; pointer-events: none">{}</g>"#,
                        clip_id, image
                    ));
                } else {
                    svg_parts.push(format!(
                        r#"<g class="ofd-signature-stamp" style="mix-blend-mode: multiply; pointer-events: none">{}</g>"#,
                        image
                    ));
                }
            } else {
                let x = stamp.visible_rect.x * scale;
                let y = stamp.visible_rect.y * scale;
                let w = stamp.visible_rect.width * scale;
                let h = stamp.visible_rect.height * scale;
                svg_parts.push(format!(
                    r##"<g class="ofd-signature-stamp-placeholder" style="pointer-events: none"><rect x="{:.2}" y="{:.2}" width="{:.2}" height="{:.2}" fill="none" stroke="#0000ff" stroke-width="1" vector-effect="non-scaling-stroke"/><line x1="{:.2}" y1="{:.2}" x2="{:.2}" y2="{:.2}" stroke="#0000ff" stroke-width="1" vector-effect="non-scaling-stroke"/><line x1="{:.2}" y1="{:.2}" x2="{:.2}" y2="{:.2}" stroke="#0000ff" stroke-width="1" vector-effect="non-scaling-stroke"/></g>"##,
                    x,
                    y,
                    w,
                    h,
                    x,
                    y,
                    x + w,
                    y + h,
                    x + w,
                    y,
                    x,
                    y + h
                ));
            }
        }
    }
}

/// 从二进制数据中提取图片（PNG/JPEG）
pub(crate) fn extract_image_from_binary(data: &[u8]) -> Option<Vec<u8>> {
    if let Some(img) = extract_image_without_asn1(data) {
        return Some(img);
    }
    extract_image_from_asn1(data)
}

fn extract_image_without_asn1(data: &[u8]) -> Option<Vec<u8>> {
    extract_image_from_binary_legacy(data)
        .or_else(|| extract_gif_by_magic(data))
        .or_else(|| extract_svg_by_string(data))
}

fn extract_gif_by_magic(data: &[u8]) -> Option<Vec<u8>> {
    const GIF89A_SIGNATURE: &[u8] = b"GIF89a";
    const GIF87A_SIGNATURE: &[u8] = b"GIF87a";

    for i in 0..data.len().saturating_sub(6) {
        if &data[i..i + 6] == GIF89A_SIGNATURE || &data[i..i + 6] == GIF87A_SIGNATURE {
            let mut last_3b_pos: Option<usize> = None;
            for j in (i + 6)..data.len() {
                if data[j] == 0x3B {
                    last_3b_pos = Some(j);
                }
            }
            if let Some(j) = last_3b_pos {
                return Some(data[i..j + 1].to_vec());
            }
            return Some(data[i..].to_vec());
        }
    }

    None
}

fn extract_svg_by_string(data: &[u8]) -> Option<Vec<u8>> {
    let text = String::from_utf8_lossy(data);
    let text_lower = text.to_lowercase();
    if let Some(start) = text_lower.find("<svg") {
        if let Some(end) = text_lower[start..].find("</svg>") {
            let svg_end = start + end + 6;
            return Some(text[start..svg_end].as_bytes().to_vec());
        }
        return Some(text[start..].as_bytes().to_vec());
    }
    None
}

fn extract_image_from_asn1(data: &[u8]) -> Option<Vec<u8>> {
    let mut stack: Vec<(Vec<u8>, usize)> = vec![(data.to_vec(), 0)];
    let mut visited: usize = 0;

    while let Some((node, depth)) = stack.pop() {
        visited += 1;
        if depth > 32 || visited > 2000 {
            continue;
        }

        if let Some(img) = extract_image_without_asn1(&node) {
            return Some(img);
        }

        if let Ok(payloads) = yasna::parse_der(&node, |reader| collect_asn1_payloads(reader)) {
            for payload in payloads {
                if payload.is_empty() {
                    continue;
                }
                if let Some(img) = extract_image_without_asn1(&payload) {
                    return Some(img);
                }
                stack.push((payload, depth + 1));
            }
        }
    }

    None
}

fn collect_asn1_payloads(reader: yasna::BERReader<'_, '_>) -> yasna::ASN1Result<Vec<Vec<u8>>> {
    use yasna::tags::TAG_BITSTRING;
    use yasna::tags::TAG_OCTETSTRING;

    let tagged = reader.read_tagged_der()?;
    let mut out = Vec::new();

    if tagged.tag() == TAG_OCTETSTRING {
        if let Ok(bytes) = yasna::parse_der(tagged.value(), |r| r.read_bytes()) {
            out.push(bytes);
        } else {
            out.push(tagged.value().to_vec());
        }
        return Ok(out);
    }

    if tagged.tag() == TAG_BITSTRING {
        if !tagged.value().is_empty() {
            out.push(tagged.value()[1..].to_vec());
        }
        return Ok(out);
    }

    if tagged.tag().tag_number == yasna::tags::TAG_SEQUENCE.tag_number
        || tagged.tag().tag_number == yasna::tags::TAG_SET.tag_number
    {
        if let Ok(nested) = yasna::parse_der(tagged.value(), |r| {
            r.collect_sequence_of(|rr| collect_asn1_payloads(rr))
        }) {
            for group in nested {
                for item in group {
                    out.push(item);
                }
            }
        } else {
            out.push(tagged.value().to_vec());
        }
        return Ok(out);
    }

    out.push(tagged.value().to_vec());
    Ok(out)
}

fn extract_image_from_binary_legacy(data: &[u8]) -> Option<Vec<u8>> {
    let png_sig = [0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A];
    let jpg_sig = [0xFF, 0xD8, 0xFF];

    // 查找 PNG
    for i in 0..data.len().saturating_sub(8) {
        if data[i..i + 8] == png_sig {
            // 找 IEND
            for j in i + 8..data.len().saturating_sub(8) {
                if data[j] == 0x49
                    && data[j + 1] == 0x45
                    && data[j + 2] == 0x4E
                    && data[j + 3] == 0x44
                {
                    return Some(data[i..j + 8].to_vec());
                }
            }
            return Some(data[i..].to_vec());
        }
    }

    // 查找 JPEG
    for i in 0..data.len().saturating_sub(3) {
        if data[i..i + 3] == jpg_sig {
            for j in i + 3..data.len().saturating_sub(1) {
                if data[j] == 0xFF && data[j + 1] == 0xD9 {
                    return Some(data[i..j + 2].to_vec());
                }
            }
            return Some(data[i..].to_vec());
        }
    }

    None
}

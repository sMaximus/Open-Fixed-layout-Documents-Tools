//! SVG 渲染模块
//! 将 OFD 页面渲染为 SVG 字符串，文字使用 SVG text 元素回退渲染

use base64::{engine::general_purpose::STANDARD as BASE64, Engine as _};
use std::fmt::Write;

use crate::page::*;
use crate::parser::*;
use crate::render::*;

/// SVG 渲染结果
#[derive(Debug, Default, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SVGRenderResult {
    pub page_index: usize,
    pub width: f64,
    pub height: f64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub svg: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub text_overlay: Option<Vec<TextOverlayItem>>,
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

use std::sync::atomic::{AtomicUsize, Ordering};

static SVG_GRADIENT_COUNTER: AtomicUsize = AtomicUsize::new(0);

fn next_gradient_id() -> String {
    let id = SVG_GRADIENT_COUNTER.fetch_add(1, Ordering::Relaxed);
    format!("grad_{}", id)
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
        let scale = 3.78;
        let px_w = width * scale;
        let px_h = height * scale;
        result.width = width;
        result.height = height;

        self.load_resources();

        let mut svg_parts: Vec<String> = Vec::new();
        let mut text_overlay: Vec<TextOverlayItem> = Vec::new();

        // 白色背景
        svg_parts.push(format!(
            r#"<rect width="{:.2}" height="{:.2}" fill="white"/>"#,
            px_w, px_h
        ));

        // 渲染模板层
        self.render_template_svg(&mut svg_parts, &mut text_overlay, &page, scale, width, height);

        // 获取页面层
        let layers = self.get_page_layers(&page);

        for layer in &layers {
            for path_obj in layer.path_objects() {
                if let Some(s) = self.path_object_to_svg(path_obj, scale, width, height) {
                    svg_parts.push(s);
                }
            }
            for img in layer.image_objects() {
                if let Some(s) = self.image_object_to_svg(img, scale) {
                    svg_parts.push(s);
                }
            }
            for text in layer.text_objects() {
                let (text_svgs, overlays) = self.render_text_svg(text, scale);
                svg_parts.extend(text_svgs);
                text_overlay.extend(overlays);
            }
        }

        // 注释图片
        self.load_page_annot_svg(&mut svg_parts, scale, page_index);

        // 印章
        self.load_stamps_svg(&mut svg_parts, &mut text_overlay, scale, page_index, width, height);

        let svg = format!(
            r#"<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="{:.2}" height="{:.2}" viewBox="0 0 {:.2} {:.2}">
{}
</svg>"#,
            px_w, px_h, px_w, px_h,
            svg_parts.join("\n")
        );

        result.svg = Some(svg);
        result.text_overlay = Some(text_overlay);
        result
    }

    fn get_page_layers<'a>(&self, page: &'a Page) -> Vec<&'a Layer> {
        let mut layers: Vec<&Layer> = page.content.layer.iter().collect();
        if layers.is_empty() {
            layers = page.layer.iter().collect();
        }
        layers
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

        let doc_base = self.ofd.as_ref()
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
                for path_obj in layer.path_objects() {
                    if let Some(s) = self.path_object_to_svg(path_obj, scale, page_w, page_h) {
                        svg_parts.push(s);
                    }
                }
                for img in layer.image_objects() {
                    if let Some(s) = self.image_object_to_svg(img, scale) {
                        svg_parts.push(s);
                    }
                }
                for text in layer.text_objects() {
                    let (text_svgs, overlays) = self.render_text_svg(text, scale);
                    svg_parts.extend(text_svgs);
                    text_overlay.extend(overlays);
                }
            }
        }
    }

    /// 将 PathObject 转换为 SVG path 元素
    fn path_object_to_svg(&self, path_obj: &PathObject, scale: f64, page_w: f64, page_h: f64) -> Option<String> {
        if path_obj.abbreviated_data.is_empty() {
            return None;
        }

        let (bx, by, _, _) = parse_boundary(&path_obj.boundary);

        let ctm = if !path_obj.ctm.is_empty() {
            parse_ctm(&path_obj.ctm)
        } else {
            Vec::new()
        };

        // 使用已有的路径转换函数获取 JSON 命令
        let cmd_json = convert_ofd_path_to_canvas(
            &path_obj.abbreviated_data, scale, bx, by, &ctm,
            page_w * scale, page_h * scale,
        );

        // 解析 JSON 命令数组，转换为 SVG path d 属性
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
                    let _ = write!(d, "C{:.4},{:.4} {:.4},{:.4} {:.4},{:.4} ", x1, y1, x2, y2, x, y);
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
            if !fc.value.is_empty() {
                fill_color = parse_color(&fc.value);
            } else if let Some(ref axial) = fc.axial_shd {
                let grad = self.parse_axial_shd(axial, &ctm, scale);
                let grad_id = next_gradient_id();
                let gx0 = (grad.x0 + bx) * scale;
                let gy0 = (grad.y0 + by) * scale;
                let gx1 = (grad.x1 + bx) * scale;
                let gy1 = (grad.y1 + by) * scale;
                let mut stops = String::new();
                for s in &grad.stops {
                    let _ = write!(stops, r#"<stop offset="{:.2}" stop-color="{}"/>"#, s.position, s.color);
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
                    let _ = write!(stops, r#"<stop offset="{:.2}" stop-color="{}"/>"#, s.position, s.color);
                }
                defs_svg = format!(
                    r#"<defs><radialGradient id="{}" cx="{:.4}" cy="{:.4}" r="{:.4}" gradientUnits="userSpaceOnUse">{}</radialGradient></defs>"#,
                    grad_id, gx1, gy1, r1, stops
                );
                fill_color = format!("url(#{})", grad_id);
            }
        }
        attrs.push(format!(r#"fill="{}""#, fill_color));

        // fill-rule
        if path_obj.fill_color.as_ref().map_or(false, |_| true) {
            // Check Rule attribute via abbreviated approach
        }

        // 描边
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
            let mut lw = path_obj.line_width;
            if lw == 0.0 {
                lw = 1.0 / scale;
            } else if !ctm.is_empty() && ctm[0] > 0.0 {
                lw *= ctm[0];
            }
            attrs.push(format!(r#"stroke-width="{:.4}""#, lw * scale));

            if !path_obj.join.is_empty() {
                attrs.push(format!(r#"stroke-linejoin="{}""#, path_obj.join.to_lowercase()));
            }
            if !path_obj.cap.is_empty() {
                attrs.push(format!(r#"stroke-linecap="{}""#, path_obj.cap.to_lowercase()));
            }
        } else {
            attrs.push("stroke=\"none\"".to_string());
        }

        let path_elem = format!("<path {}/>", attrs.join(" "));
        if defs_svg.is_empty() {
            Some(path_elem)
        } else {
            Some(format!("{}\n{}", defs_svg, path_elem))
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
                    r#"<image href="{}" x="0" y="0" width="1" height="1" transform="matrix({:.6},{:.6},{:.6},{:.6},{:.6},{:.6})" preserveAspectRatio="none"/>"#,
                    data_url, a, b, c, d, px + e, py + f
                ));
            }
        }

        // 普通绘制
        let px = ix * scale;
        let py = iy * scale;
        let pw = iw * scale;
        let ph = ih * scale;

        Some(format!(
            r#"<image href="{}" x="{:.2}" y="{:.2}" width="{:.2}" height="{:.2}" preserveAspectRatio="none"/>"#,
            data_url, px, py, pw, ph
        ))
    }

    /// 渲染文本对象为 SVG text 元素
    fn render_text_svg(&self, text: &TextObject, scale: f64) -> (Vec<String>, Vec<TextOverlayItem>) {
        let mut results = Vec::new();
        let mut overlays = Vec::new();

        let (bx, by, _, _) = parse_boundary(&text.boundary);
        let font_id = &text.font;
        let font_size = text.size;

        // 解析颜色
        let fill_color = text.fill_color.as_ref()
            .filter(|c| !c.value.is_empty())
            .map(|c| parse_color(&c.value))
            .unwrap_or_else(|| "#000".to_string());

        let stroke_color_str = text.stroke_color.as_ref()
            .filter(|c| !c.value.is_empty())
            .map(|c| parse_color(&c.value));

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

        // CTM 缩放
        let _scale_x = if ctm.len() >= 4 && ctm[0].abs() > 0.001 { ctm[0].abs() } else { 1.0 };
        let scale_y = if ctm.len() >= 4 && ctm[3].abs() > 0.001 { ctm[3].abs() } else { 1.0 };

        let h_scale = if text.h_scale > 0.0 { text.h_scale } else { 1.0 };
        let char_space = text.size * h_scale; // default advance if no deltaX

        let font_size_px = font_size * scale * scale_y;

        // 字体族
        let font_family = if let Some(font) = self.fonts.get(font_id) {
            format!("'{}', '{}', SimSun, serif", font.font_name, font.family_name)
        } else {
            "SimSun, serif".to_string()
        };

        // 描边属性
        let line_width_px = if text.stroke && text.line_width > 0.0 {
            text.line_width * scale
        } else {
            0.0
        };

        for tc in &text.text_code {
            let content = &tc.content;
            if content.is_empty() {
                continue;
            }
            let chars: Vec<char> = content.chars().collect();

            let delta_x = parse_deltas(&tc.delta_x);
            let delta_y = parse_deltas(&tc.delta_y);

            // 计算 TextCode 坐标
            let (mut tc_x_mm, mut tc_y_mm) = (tc.x, tc.y);
            if ctm.len() >= 4 {
                let (a, b, c, d) = (ctm[0], ctm[1], ctm[2], ctm[3]);
                tc_x_mm = a * tc.x + c * tc.y;
                tc_y_mm = b * tc.x + d * tc.y;
            }

            let mut current_x_mm = tc_x_mm;
            let mut current_y_mm = tc_y_mm;

            let mut seg_text = Vec::new();
            let mut seg_positions_x = Vec::new();
            let mut seg_start_y_mm = current_y_mm;

            let flush_segment = |seg_text: &mut Vec<char>, seg_positions_x: &mut Vec<f64>, seg_start_y_mm: f64,
                                      results: &mut Vec<String>, overlays: &mut Vec<TextOverlayItem>| {
                if seg_text.is_empty() {
                    return;
                }

                // 逐字符生成 SVG text
                for (ci, ch) in seg_text.iter().enumerate() {
                    let cpx = (bx + seg_positions_x[ci]) * scale;
                    let cpy = (by + seg_start_y_mm) * scale;

                    let mut weight_attr = String::new();
                    if text.weight >= 700 {
                        weight_attr = r#" font-weight="bold""#.to_string();
                    }
                    let mut italic_attr = String::new();
                    if text.italic {
                        italic_attr = r#" font-style="italic""#.to_string();
                    }

                    let fill_attr = format!(r#"fill="{}""#, color);
                    let mut stroke_attr = String::new();
                    if text.stroke {
                        if let Some(ref sc) = stroke_color_str {
                            stroke_attr = format!(r#" stroke="{}" stroke-width="{:.2}""#, sc, line_width_px);
                        }
                    }

                    let escaped = xml_escape(&ch.to_string());
                    results.push(format!(
                        r#"<text x="{:.2}" y="{:.2}" font-size="{:.2}" font-family="{}" {}{}{}{}>{}</text>"#,
                        cpx, cpy, font_size_px, font_family, fill_attr, weight_attr, italic_attr, stroke_attr, escaped
                    ));
                }

                // 文本蒙层
                if !seg_text.is_empty() {
                    let txt: String = seg_text.iter().collect();
                    let first_px = (bx + seg_positions_x[0]) * scale;
                    let last_px = (bx + seg_positions_x[seg_positions_x.len() - 1]) * scale;
                    let py = (by + seg_start_y_mm) * scale;
                    let text_width = last_px - first_px + font_size_px * 0.9;
                    overlays.push(TextOverlayItem {
                        text: txt,
                        x: first_px,
                        y: py - font_size_px * 0.85,
                        width: text_width,
                        height: font_size_px * 1.2,
                    });
                }

                seg_text.clear();
                seg_positions_x.clear();
            };

            for (i, ch) in chars.iter().enumerate() {
                let is_space = *ch == ' ' || *ch == '\u{3000}';

                if is_space {
                    flush_segment(&mut seg_text, &mut seg_positions_x, seg_start_y_mm, &mut results, &mut overlays);
                } else {
                    if seg_text.is_empty() {
                        seg_start_y_mm = current_y_mm;
                    }
                    seg_text.push(*ch);
                    seg_positions_x.push(current_x_mm);
                }

                // 计算下一个字符位置
                let mut dx = 0.0_f64;
                let mut dy = 0.0_f64;

                if i < delta_x.len() {
                    dx = delta_x[i];
                } else if i < chars.len() - 1 {
                    dx = char_space;
                }
                if i < delta_y.len() {
                    dy = delta_y[i];
                }

                if dx != 0.0 || dy != 0.0 {
                    if ctm.len() >= 4 {
                        let (a, b, c, d) = (ctm[0], ctm[1], ctm[2], ctm[3]);
                        current_x_mm += a * dx + c * dy;
                        current_y_mm += b * dx + d * dy;
                    } else {
                        current_x_mm += dx;
                        current_y_mm += dy;
                    }
                }
            }
            flush_segment(&mut seg_text, &mut seg_positions_x, seg_start_y_mm, &mut results, &mut overlays);
        }

        (results, overlays)
    }

    /// 加载页面注释图片为 SVG 元素
    fn load_page_annot_svg(&mut self, svg_parts: &mut Vec<String>, scale: f64, page_index: usize) {
        let doc = match self.document.as_ref() {
            Some(d) => d.clone(),
            None => return,
        };
        if page_index >= doc.pages.page.len() {
            return;
        }

        let page_id = doc.pages.page[page_index].id.clone();
        let _doc_base = self.ofd.as_ref()
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

            let annot_file_path = format!("{}/{}", annot_index_dir, ap.file_loc.trim_start_matches('/'));
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
                let (ax, ay, _, _) = parse_boundary(&annot.appearance.boundary);

                for block in &annot.appearance.page_blocks {
                    for a_img in &block.image_objects {
                        if let Some(img_data) = self.load_image_lazy(&a_img.resource_id) {
                            let (ix, iy, iw, ih) = parse_boundary(&a_img.boundary);
                            let mime_type = if img_data.len() > 2 && img_data[0] == 0xFF && img_data[1] == 0xD8 {
                                "image/jpeg"
                            } else {
                                "image/png"
                            };
                            let data_url = format!("data:{};base64,{}", mime_type, BASE64.encode(&img_data));
                            let px = (ax + ix) * scale;
                            let py = (ay + iy) * scale;
                            let pw = iw * scale;
                            let ph = ih * scale;
                            svg_parts.push(format!(
                                r#"<image href="{}" x="{:.2}" y="{:.2}" width="{:.2}" height="{:.2}" preserveAspectRatio="none"/>"#,
                                data_url, px, py, pw, ph
                            ));
                        }
                    }
                }
            }
        }
    }

    /// 加载印章为 SVG 元素
    fn load_stamps_svg(
        &mut self,
        svg_parts: &mut Vec<String>,
        text_overlay: &mut Vec<TextOverlayItem>,
        scale: f64,
        page_index: usize,
        _page_w: f64,
        _page_h: f64,
    ) {
        let doc = match self.document.as_ref() {
            Some(d) => d.clone(),
            None => return,
        };
        if page_index >= doc.pages.page.len() {
            return;
        }
        let page_id = doc.pages.page[page_index].id.clone();

        // 收集印章注释
        struct StampAnnotInfo {
            annot: StampAnnot,
            sig_dir: String,
        }
        let mut all_annots: Vec<StampAnnotInfo> = Vec::new();

        let files_clone: Vec<String> = self.files.clone();
        for file in &files_clone {
            let lower = file.to_lowercase();
            if !lower.ends_with("signatures.xml") {
                continue;
            }

            let data = match self.read_file(file) {
                Ok(d) => d,
                Err(_) => continue,
            };

            let xml_str = Self::remove_namespace_prefix(&String::from_utf8_lossy(&data));
            let sigs: Signatures = match quick_xml::de::from_str(&xml_str) {
                Ok(s) => s,
                Err(_) => continue,
            };

            let base_path = std::path::Path::new(file)
                .parent()
                .and_then(|p| p.to_str())
                .unwrap_or("")
                .to_string();

            for sig in &sigs.signature {
                let sig_loc = sig.base_loc.trim_start_matches('/');
                let candidates = vec![
                    format!("{}/{}", base_path, sig_loc),
                    sig_loc.to_string(),
                ];

                let mut sig_data = None;
                let mut sig_path = String::new();
                for candidate in &candidates {
                    if let Ok(d) = self.read_file(candidate) {
                        sig_data = Some(d);
                        sig_path = candidate.to_string();
                        break;
                    }
                }

                let sig_data = match sig_data {
                    Some(d) => d,
                    None => continue,
                };

                let sig_dir = std::path::Path::new(&sig_path)
                    .parent()
                    .and_then(|p| p.to_str())
                    .unwrap_or("")
                    .to_string();

                let sig_xml = Self::remove_namespace_prefix(&String::from_utf8_lossy(&sig_data));
                let sig_parsed: SignatureXML = match quick_xml::de::from_str(&sig_xml) {
                    Ok(s) => s,
                    Err(_) => {
                        // 尝试正则提取 StampAnnot
                        let re = regex::Regex::new(r#"(?i)<(?:\w+:)?StampAnnot[^>]*PageRef\s*=\s*"([^"]*)"[^>]*Boundary\s*=\s*"([^"]*)"[^/>]*/?>"#).ok();
                        let re2 = regex::Regex::new(r#"(?i)<(?:\w+:)?StampAnnot[^>]*Boundary\s*=\s*"([^"]*)"[^>]*PageRef\s*=\s*"([^"]*)"[^/>]*/?>"#).ok();
                        if let Some(re) = re {
                            for cap in re.captures_iter(&sig_xml) {
                                all_annots.push(StampAnnotInfo {
                                    annot: StampAnnot {
                                        page_ref: cap[1].to_string(),
                                        boundary: cap[2].to_string(),
                                        ..Default::default()
                                    },
                                    sig_dir: sig_dir.clone(),
                                });
                            }
                        }
                        if let Some(re2) = re2 {
                            for cap in re2.captures_iter(&sig_xml) {
                                all_annots.push(StampAnnotInfo {
                                    annot: StampAnnot {
                                        page_ref: cap[2].to_string(),
                                        boundary: cap[1].to_string(),
                                        ..Default::default()
                                    },
                                    sig_dir: sig_dir.clone(),
                                });
                            }
                        }
                        continue;
                    }
                };

                for annot in &sig_parsed.signed_info.stamp_annot {
                    all_annots.push(StampAnnotInfo {
                        annot: annot.clone(),
                        sig_dir: sig_dir.clone(),
                    });
                }
            }
        }

        // 匹配当前页面的印章
        for item in &all_annots {
            if item.annot.page_ref != page_id {
                continue;
            }

            let (x, y, w, h) = parse_boundary(&item.annot.boundary);
            if w == 0.0 || h == 0.0 {
                continue;
            }

            // 尝试加载印章图片
            if let Some(seal_img) = self.find_seal_image(&item.sig_dir) {
                let mime_type = if seal_img.len() > 2 && seal_img[0] == 0xFF && seal_img[1] == 0xD8 {
                    "image/jpeg"
                } else {
                    "image/png"
                };
                let data_url = format!("data:{};base64,{}", mime_type, BASE64.encode(&seal_img));
                svg_parts.push(format!(
                    r#"<image href="{}" x="{:.2}" y="{:.2}" width="{:.2}" height="{:.2}" preserveAspectRatio="none"/>"#,
                    data_url, x * scale, y * scale, w * scale, h * scale
                ));
            } else {
                // 占位框
                text_overlay.push(TextOverlayItem {
                    text: "[电子签章]".to_string(),
                    x: x * scale,
                    y: y * scale,
                    width: w * scale,
                    height: h * scale,
                });
            }
        }
    }

    /// 查找印章图片文件
    fn find_seal_image(&mut self, sig_dir: &str) -> Option<Vec<u8>> {
        let candidates = vec![
            format!("{}/Seal.esl", sig_dir),
            format!("{}/seal.esl", sig_dir),
            format!("{}/Seal.png", sig_dir),
            format!("{}/seal.png", sig_dir),
            format!("{}/SignedValue.dat", sig_dir),
        ];

        for candidate in &candidates {
            if let Ok(data) = self.read_file(candidate) {
                // 尝试提取图片
                if let Some(img) = extract_image_from_binary(&data) {
                    return Some(img);
                }
            }
        }

        // 扫描目录下的图片文件
        let sig_dir_lower = sig_dir.to_lowercase();
        let files_clone: Vec<String> = self.files.clone();
        for file in &files_clone {
            let lower = file.to_lowercase();
            if (lower.starts_with(&sig_dir_lower) || lower.contains(sig_dir)) &&
               (lower.ends_with(".png") || lower.ends_with(".jpg") || lower.ends_with(".jpeg")) {
                if let Ok(data) = self.read_file(file) {
                    return Some(data);
                }
            }
        }

        None
    }
}

/// 从二进制数据中提取图片（PNG/JPEG）
fn extract_image_from_binary(data: &[u8]) -> Option<Vec<u8>> {
    let png_sig = [0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A];
    let jpg_sig = [0xFF, 0xD8, 0xFF];

    // 查找 PNG
    for i in 0..data.len().saturating_sub(8) {
        if data[i..i + 8] == png_sig {
            // 找 IEND
            for j in i + 8..data.len().saturating_sub(8) {
                if data[j] == 0x49 && data[j + 1] == 0x45 && data[j + 2] == 0x4E && data[j + 3] == 0x44 {
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

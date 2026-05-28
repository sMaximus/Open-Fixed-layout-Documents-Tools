//! 页面渲染模块

use base64::{engine::general_purpose::STANDARD as BASE64, Engine as _};
use serde::{Deserialize, Serialize};

use crate::page::*;
use crate::parser::*;

/// 页面渲染结果
#[derive(Debug, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PageRenderResult {
    pub page_index: usize,
    pub width: f64,
    pub height: f64,
    pub canvas_data: CanvasRenderData,
    pub text_layer: Vec<TextItem>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub stamp_debug: Option<Vec<StampDebugInfo>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

/// Canvas渲染数据
#[derive(Debug, Default, Serialize, Deserialize)]
pub struct CanvasRenderData {
    pub paths: Vec<PathData>,
    pub images: Vec<ImageData>,
}

/// 路径数据
#[derive(Debug, Default, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PathData {
    pub commands: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub fill_color: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub stroke_color: Option<String>,
    pub line_width: f64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub line_join: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub line_cap: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub gradient: Option<GradientData>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pattern: Option<PatternData>,
}

/// 渐变数据
#[derive(Debug, Default, Clone, Serialize, Deserialize)]
pub struct GradientData {
    #[serde(rename = "type")]
    pub gradient_type: String,
    pub x0: f64,
    pub y0: f64,
    pub x1: f64,
    pub y1: f64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub r0: Option<f64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub r1: Option<f64>,
    pub stops: Vec<GradientStop>,
}

/// 渐变色标
#[derive(Debug, Default, Clone, Serialize, Deserialize)]
pub struct GradientStop {
    pub position: f64,
    pub color: String,
}

/// Pattern 填充数据
#[derive(Debug, Default, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PatternData {
    pub width: f64,
    pub height: f64,
    pub x_step: f64,
    pub y_step: f64,
    pub relative_to: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub ctm: Option<Vec<f64>>,
    pub cell_images: Vec<ImageData>,
    pub cell_paths: Vec<PathData>,
}

/// 调试信息
#[derive(Debug, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DebugInfo {
    pub image_ids: Vec<String>,
    pub requested_ids: Vec<String>,
    pub missing_ids: Vec<String>,
    pub stamps: Vec<String>,
    pub files: Vec<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub text_debug: Option<Vec<String>>,
}

#[derive(Debug, Default, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct StampDebugInfo {
    pub annot_id: String,
    pub page_ref: String,
    pub boundary: String,
    pub clip: String,
    pub sig_dir: String,
    pub seal_found: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub seal_source: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub mime_type: Option<String>,
    pub has_clip: bool,
    pub full_boundary: String,
    pub visible_boundary: String,
    pub source_view_box: String,
}

/// 图片数据
#[derive(Debug, Default, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ImageData {
    #[serde(rename = "dataURL")]
    pub data_url: String,
    pub x: f64,
    pub y: f64,
    pub width: f64,
    pub height: f64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub ctm: Option<Vec<f64>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub is_seal: Option<bool>,
}

/// 文本项
#[derive(Debug, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TextItem {
    pub text: String,
    pub x: f64,
    pub y: f64,
    pub font_size: f64,
    pub font_family: String,
    pub font_id: String,
    pub color: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub ctm: Option<Vec<f64>>,
    pub stroke: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub stroke_color: Option<String>,
    pub line_width: f64,
    pub fill: bool,
}

/// 字体信息
#[derive(Debug, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct FontInfo {
    pub id: String,
    pub font_name: String,
    pub family_name: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub data_url: Option<String>,
    pub has_file: bool,
}

impl Parser {
    /// 渲染指定页面
    pub fn render_page(&mut self, page_index: usize) -> PageRenderResult {
        let mut result = PageRenderResult {
            page_index,
            canvas_data: CanvasRenderData::default(),
            text_layer: Vec::new(),
            ..Default::default()
        };

        let page_count = self.get_page_count();
        if page_index >= page_count {
            result.error = Some("页面不存在".to_string());
            return result;
        }

        // 获取页面路径
        let page_path = match self.get_page_path(page_index) {
            Some(p) => p,
            None => {
                result.error = Some("无法获取页面路径".to_string());
                return result;
            }
        };

        // 读取页面XML
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

        // 获取页面尺寸
        let (width, height) = self.get_page_size_from_page(&page);

        let scale = 3.78; // mm to px
        result.width = width * scale;
        result.height = height * scale;

        // 加载资源
        self.load_resources();

        // 提取渲染数据
        let scale = 3.78; // mm to px
        self.extract_render_data(&mut result, &page, scale);
        let stamps = self.collect_page_stamps(page_index);
        if !stamps.is_empty() {
            result.stamp_debug = Some(stamps.iter().map(|stamp| stamp.debug_info()).collect());
        }
        self.extract_stamp_images(&mut result, &stamps, scale);

        result
    }

    pub(crate) fn get_page_size_from_page(&self, page: &Page) -> (f64, f64) {
        // 首先尝试从页面的 Area 获取
        if !page.area.physical_box.is_empty() {
            let (w, h) = parse_box(&page.area.physical_box);
            if w > 0.0 && h > 0.0 {
                return (w, h);
            }
        }
        // 然后尝试从文档的 CommonData 获取
        if let Some(ref doc) = self.document {
            if !doc.common_data.page_area.physical_box.is_empty() {
                let (w, h) = parse_box(&doc.common_data.page_area.physical_box);
                if w > 0.0 && h > 0.0 {
                    return (w, h);
                }
            }
        }
        // 默认 A4 尺寸
        (210.0, 297.0)
    }

    fn extract_render_data(&mut self, result: &mut PageRenderResult, page: &Page, scale: f64) {
        // 合并多种可能的 Content 来源
        let mut layers: Vec<&Layer> = page.content.layer.iter().collect();
        if layers.is_empty() {
            layers = page.layer.iter().collect();
        }

        for layer in layers {
            // Keep original object order to preserve z-order (e.g. text strikethrough paths).
            for obj in &layer.objects {
                self.extract_layer_object(result, obj, scale);
            }
        }
    }

    fn extract_layer_object(
        &mut self,
        result: &mut PageRenderResult,
        obj: &LayerObject,
        scale: f64,
    ) {
        match obj {
            LayerObject::ImageObject(img) => {
                self.extract_image(result, img, scale);
            }
            LayerObject::PathObject(path_obj) => {
                let page_w = result.width;
                let page_h = result.height;
                self.extract_path(result, path_obj, scale, page_w, page_h);
            }
            LayerObject::TextObject(text) => {
                self.extract_text(result, text, scale);
            }
            LayerObject::CompositeObject(comp) => {
                self.extract_composite_object(result, comp, scale);
            }
            LayerObject::PageBlock(block) => {
                for child in &block.objects {
                    self.extract_layer_object(result, child, scale);
                }
            }
        }
    }

    fn extract_composite_object(
        &mut self,
        result: &mut PageRenderResult,
        comp: &CompositeObject,
        scale: f64,
    ) {
        let unit = match self.composite_units.get(&comp.resource_id) {
            Some(u) => u.clone(),
            None => return,
        };

        let (cx, cy, cw, ch) = parse_boundary(&comp.boundary);
        let sx = if unit.width > 0.0 {
            cw / unit.width
        } else {
            1.0
        };
        let sy = if unit.height > 0.0 {
            ch / unit.height
        } else {
            1.0
        };

        let page_block = match &unit.content.page_block {
            Some(pb) => pb,
            None => return,
        };

        for obj in &page_block.objects {
            self.extract_composite_page_block_object(result, obj, scale, cx, cy, sx, sy);
        }
    }

    fn extract_composite_page_block_object(
        &mut self,
        result: &mut PageRenderResult,
        obj: &LayerObject,
        scale: f64,
        cx: f64,
        cy: f64,
        sx: f64,
        sy: f64,
    ) {
        match obj {
            LayerObject::PathObject(path_obj) => {
                let mut adjusted = path_obj.clone();
                let (bx, by, bw, bh) = parse_boundary(&adjusted.boundary);
                adjusted.boundary =
                    format!("{} {} {} {}", bx * sx + cx, by * sy + cy, bw * sx, bh * sy);
                let page_w = result.width;
                let page_h = result.height;
                self.extract_path(result, &adjusted, scale, page_w, page_h);
            }
            LayerObject::ImageObject(img) => {
                let mut adjusted = img.clone();
                let (bx, by, bw, bh) = parse_boundary(&adjusted.boundary);
                adjusted.boundary =
                    format!("{} {} {} {}", bx * sx + cx, by * sy + cy, bw * sx, bh * sy);
                self.extract_image(result, &adjusted, scale);
            }
            LayerObject::TextObject(text) => {
                let mut adjusted = text.clone();
                let (bx, by, bw, bh) = parse_boundary(&adjusted.boundary);
                adjusted.boundary =
                    format!("{} {} {} {}", bx * sx + cx, by * sy + cy, bw * sx, bh * sy);
                adjusted.size *= sy;
                self.extract_text(result, &adjusted, scale);
            }
            LayerObject::PageBlock(block) => {
                for child in &block.objects {
                    self.extract_composite_page_block_object(result, child, scale, cx, cy, sx, sy);
                }
            }
            LayerObject::CompositeObject(_) => {}
        }
    }

    fn extract_image(&mut self, result: &mut PageRenderResult, img: &ImageObject, scale: f64) {
        let img_data = match self.load_image_lazy(&img.resource_id) {
            Some(d) => d,
            None => return,
        };

        let (x, y, w, h) = parse_boundary(&img.boundary);

        let mime_type = if img_data.len() > 2 && img_data[0] == 0xFF && img_data[1] == 0xD8 {
            "image/jpeg"
        } else {
            "image/png"
        };

        let mut img_data_out = ImageData {
            data_url: format!("data:{};base64,{}", mime_type, BASE64.encode(&img_data)),
            x: x * scale,
            y: y * scale,
            width: w * scale,
            height: h * scale,
            ..Default::default()
        };

        // 解析 CTM 变换矩阵
        if !img.ctm.is_empty() {
            let ctm = parse_ctm(&img.ctm);
            if ctm.len() >= 4 && (ctm[1] != 0.0 || ctm[2] != 0.0) {
                let e = if ctm.len() > 4 { ctm[4] * scale } else { 0.0 };
                let f = if ctm.len() > 5 { ctm[5] * scale } else { 0.0 };
                img_data_out.ctm = Some(vec![
                    ctm[0] * scale,
                    ctm[1] * scale,
                    ctm[2] * scale,
                    ctm[3] * scale,
                    e,
                    f,
                ]);
            }
        }

        result.canvas_data.images.push(img_data_out);
    }

    fn extract_stamp_images(
        &mut self,
        result: &mut PageRenderResult,
        stamps: &[crate::stamp::ResolvedStamp],
        scale: f64,
    ) {
        for stamp in stamps {
            let data_url = match stamp.canvas_data_url() {
                Some(data_url) => data_url,
                None => {
                    continue;
                }
            };

            result.canvas_data.images.push(ImageData {
                data_url,
                x: stamp.visible_rect.x * scale,
                y: stamp.visible_rect.y * scale,
                width: stamp.visible_rect.width * scale,
                height: stamp.visible_rect.height * scale,
                ctm: None,
                is_seal: Some(true),
            });
        }
    }

    fn extract_path(
        &mut self,
        result: &mut PageRenderResult,
        path_obj: &PathObject,
        scale: f64,
        page_width: f64,
        page_height: f64,
    ) {
        if path_obj.abbreviated_data.is_empty() {
            return;
        }

        let mut stroke_color = None;
        let mut fill_color = None;
        let mut gradient = None;
        let mut pattern = None;

        // 处理描边颜色
        if let Some(ref sc) = path_obj.stroke_color {
            if !sc.value.is_empty() {
                stroke_color = Some(parse_color(&sc.value));
            }
        } else if path_obj.stroke || path_obj.line_width > 0.0 {
            stroke_color = Some("#000".to_string());
        }

        // 解析 CTM
        let ctm = if !path_obj.ctm.is_empty() {
            parse_ctm(&path_obj.ctm)
        } else {
            Vec::new()
        };

        // 处理填充颜色、渐变或图案
        if let Some(ref fc) = path_obj.fill_color {
            if !fc.value.is_empty() {
                fill_color = Some(parse_color(&fc.value));
            } else if let Some(ref axial) = fc.axial_shd {
                gradient = Some(self.parse_axial_shd(axial, &ctm, scale));
            } else if let Some(ref radial) = fc.radial_shd {
                gradient = Some(self.parse_radial_shd(radial, &ctm, scale));
            } else if let Some(ref pat) = fc.pattern {
                pattern = Some(self.parse_pattern(pat, scale));
            }
        }

        let (bx, by, _, _) = parse_boundary(&path_obj.boundary);

        let mut line_width = path_obj.line_width;
        if line_width == 0.0 {
            line_width = 1.0 / scale;
        } else if !ctm.is_empty() && ctm[0] > 0.0 {
            line_width *= ctm[0];
        }

        // 转换 Join 和 Cap
        let line_join = match path_obj.join.to_lowercase().as_str() {
            "round" => Some("round".to_string()),
            "bevel" => Some("bevel".to_string()),
            _ => Some("miter".to_string()),
        };

        let line_cap = match path_obj.cap.to_lowercase().as_str() {
            "round" => Some("round".to_string()),
            "square" => Some("square".to_string()),
            _ => Some("butt".to_string()),
        };

        // 处理渐变坐标
        if let Some(ref mut grad) = gradient {
            grad.x0 = (grad.x0 + bx) * scale;
            grad.y0 = (grad.y0 + by) * scale;
            grad.x1 = (grad.x1 + bx) * scale;
            grad.y1 = (grad.y1 + by) * scale;
            if let Some(r0) = grad.r0 {
                grad.r0 = Some(r0 * scale);
            }
            if let Some(r1) = grad.r1 {
                grad.r1 = Some(r1 * scale);
            }
        }

        result.canvas_data.paths.push(PathData {
            commands: convert_ofd_path_to_canvas(
                &path_obj.abbreviated_data,
                scale,
                bx,
                by,
                &ctm,
                page_width * scale,
                page_height * scale,
            ),
            fill_color,
            stroke_color,
            line_width: line_width * scale,
            line_join,
            line_cap,
            gradient,
            pattern,
        });
    }

    pub(crate) fn parse_axial_shd(&self, shd: &AxialShd, ctm: &[f64], _scale: f64) -> GradientData {
        let start_parts: Vec<f64> = shd
            .start_point
            .split_whitespace()
            .filter_map(|s| s.parse().ok())
            .collect();
        let end_parts: Vec<f64> = shd
            .end_point
            .split_whitespace()
            .filter_map(|s| s.parse().ok())
            .collect();

        let (mut x0, mut y0) = if start_parts.len() >= 2 {
            (start_parts[0], start_parts[1])
        } else {
            (0.0, 0.0)
        };

        let (mut x1, mut y1) = if end_parts.len() >= 2 {
            (end_parts[0], end_parts[1])
        } else {
            (0.0, 0.0)
        };

        // 应用 CTM 变换
        if ctm.len() >= 6 {
            let (ox0, oy0, ox1, oy1) = (x0, y0, x1, y1);
            x0 = ox0 * ctm[0] + oy0 * ctm[2] + ctm[4];
            y0 = ox0 * ctm[1] + oy0 * ctm[3] + ctm[5];
            x1 = ox1 * ctm[0] + oy1 * ctm[2] + ctm[4];
            y1 = ox1 * ctm[1] + oy1 * ctm[3] + ctm[5];
        }

        let stops: Vec<GradientStop> = shd
            .segment
            .iter()
            .map(|seg| GradientStop {
                position: seg.position,
                color: parse_color(&seg.color.value),
            })
            .collect();

        GradientData {
            gradient_type: "linear".to_string(),
            x0,
            y0,
            x1,
            y1,
            r0: None,
            r1: None,
            stops,
        }
    }

    pub(crate) fn parse_radial_shd(
        &self,
        shd: &RadialShd,
        ctm: &[f64],
        _scale: f64,
    ) -> GradientData {
        let start_parts: Vec<f64> = shd
            .start_point
            .split_whitespace()
            .filter_map(|s| s.parse().ok())
            .collect();
        let end_parts: Vec<f64> = shd
            .end_point
            .split_whitespace()
            .filter_map(|s| s.parse().ok())
            .collect();

        let (mut x0, mut y0) = if start_parts.len() >= 2 {
            (start_parts[0], start_parts[1])
        } else {
            (0.0, 0.0)
        };

        let (mut x1, mut y1) = if end_parts.len() >= 2 {
            (end_parts[0], end_parts[1])
        } else {
            (0.0, 0.0)
        };

        if ctm.len() >= 6 {
            let (ox0, oy0, ox1, oy1) = (x0, y0, x1, y1);
            x0 = ox0 * ctm[0] + oy0 * ctm[2] + ctm[4];
            y0 = ox0 * ctm[1] + oy0 * ctm[3] + ctm[5];
            x1 = ox1 * ctm[0] + oy1 * ctm[2] + ctm[4];
            y1 = ox1 * ctm[1] + oy1 * ctm[3] + ctm[5];
        }

        let stops: Vec<GradientStop> = shd
            .segment
            .iter()
            .map(|seg| GradientStop {
                position: seg.position,
                color: parse_color(&seg.color.value),
            })
            .collect();

        GradientData {
            gradient_type: "radial".to_string(),
            x0,
            y0,
            x1,
            y1,
            r0: Some(shd.start_radius),
            r1: Some(shd.end_radius),
            stops,
        }
    }

    /// 解析 Pattern 图案填充
    fn parse_pattern(&mut self, pattern: &Pattern, _scale: f64) -> PatternData {
        let mut pattern_data = PatternData {
            width: pattern.width,
            height: pattern.height,
            x_step: pattern.x_step,
            y_step: pattern.y_step,
            relative_to: if pattern.relative_to.is_empty() {
                "Page".to_string()
            } else {
                pattern.relative_to.clone()
            },
            ctm: None,
            cell_images: Vec::new(),
            cell_paths: Vec::new(),
        };

        // 解析 Pattern 的 CTM
        if !pattern.ctm.is_empty() {
            pattern_data.ctm = Some(parse_ctm(&pattern.ctm));
        }

        // 处理 CellContent 中的图片
        for img in &pattern.cell_content.image_objects {
            if let Some(img_bytes) = self.load_image_lazy(&img.resource_id) {
                let (img_x, img_y, mut img_w, mut img_h) = parse_boundary(&img.boundary);

                // 如果有 CTM，使用 CTM 中的尺寸
                if !img.ctm.is_empty() {
                    let img_ctm = parse_ctm(&img.ctm);
                    if img_ctm.len() >= 4 {
                        img_w = img_ctm[0];
                        img_h = img_ctm[3];
                    }
                }

                let mime_type =
                    if img_bytes.len() > 2 && img_bytes[0] == 0xFF && img_bytes[1] == 0xD8 {
                        "image/jpeg"
                    } else {
                        "image/png"
                    };

                // Pattern 内部坐标保持 mm 单位，前端会处理缩放
                pattern_data.cell_images.push(ImageData {
                    data_url: format!("data:{};base64,{}", mime_type, BASE64.encode(&img_bytes)),
                    x: img_x,
                    y: img_y,
                    width: img_w,
                    height: img_h,
                    ctm: None,
                    is_seal: None,
                });
            }
        }

        // 处理 CellContent 中的路径
        for path_obj in &pattern.cell_content.path_objects {
            if path_obj.abbreviated_data.is_empty() {
                continue;
            }

            let mut fill_color = None;
            let mut stroke_color = None;

            if let Some(ref fc) = path_obj.fill_color {
                if !fc.value.is_empty() {
                    fill_color = Some(parse_color(&fc.value));
                }
            }

            if let Some(ref sc) = path_obj.stroke_color {
                if !sc.value.is_empty() {
                    stroke_color = Some(parse_color(&sc.value));
                }
            }

            let (bx, by, _, _) = parse_boundary(&path_obj.boundary);

            let ctm = if !path_obj.ctm.is_empty() {
                parse_ctm(&path_obj.ctm)
            } else {
                Vec::new()
            };

            let mut line_width = path_obj.line_width;
            if line_width == 0.0 {
                line_width = 0.353;
            }

            // Pattern 内部路径不需要检查页面边界
            pattern_data.cell_paths.push(PathData {
                commands: convert_ofd_path_to_canvas(
                    &path_obj.abbreviated_data,
                    1.0,
                    bx,
                    by,
                    &ctm,
                    0.0,
                    0.0,
                ),
                fill_color,
                stroke_color,
                line_width,
                line_join: None,
                line_cap: None,
                gradient: None,
                pattern: None,
            });
        }

        pattern_data
    }

    fn extract_text(&self, result: &mut PageRenderResult, text: &TextObject, scale: f64) {
        let (bx, by, _, _) = parse_boundary(&text.boundary);

        let font_id = text.font.clone();
        let font_family = if let Some(font) = self.fonts.get(&text.font) {
            let has_file = self.font_files.contains_key(&text.font);
            let mut families = Vec::new();
            if has_file {
                families.push(format!("'OFD_Font_{}'", text.font));
            }
            if !font.font_name.is_empty() {
                let (clean_name, _) = crate::svg_render::strip_vertical_prefix(&font.font_name);
                families.push(format!("'{}'", clean_name));
                for alias in crate::svg_render::get_font_aliases(&font.font_name) {
                    families.push(format!("'{}'", alias));
                }
            }
            if !font.family_name.is_empty() && font.family_name != font.font_name {
                let (clean_name, _) = crate::svg_render::strip_vertical_prefix(&font.family_name);
                families.push(format!("'{}'", clean_name));
                for alias in crate::svg_render::get_font_aliases(&font.family_name) {
                    let a = format!("'{}'", alias);
                    if !families.contains(&a) {
                        families.push(a);
                    }
                }
            }
            families.push(crate::svg_render::cjk_fallback_fonts().to_string());
            families.join(", ")
        } else {
            crate::svg_render::cjk_fallback_fonts().to_string()
        };

        let font_size = text.size * scale;

        // 处理颜色
        let fill_color = text
            .fill_color
            .as_ref()
            .filter(|c| !c.value.is_empty())
            .map(|c| parse_color(&c.value))
            .unwrap_or_else(|| "#000".to_string());

        let stroke_color = text
            .stroke_color
            .as_ref()
            .filter(|c| !c.value.is_empty())
            .map(|c| parse_color(&c.value));

        let color = if text.stroke && !text.fill && stroke_color.is_some() {
            stroke_color.clone().unwrap()
        } else {
            fill_color.clone()
        };

        let should_fill = text.fill;

        let mut line_width = text.line_width * scale;
        if line_width == 0.0 && text.stroke {
            line_width = 0.5;
        }

        // 解析 CTM
        let ctm = if !text.ctm.is_empty() {
            let mut c = parse_ctm(&text.ctm);
            if c.len() > 4 {
                c[4] *= scale;
            }
            if c.len() > 5 {
                c[5] *= scale;
            }
            if !c.is_empty() && c[0] > 0.0 && line_width > 0.0 {
                line_width *= c[0];
            }
            Some(c)
        } else {
            None
        };

        for tc in &text.text_code {
            let content = &tc.content;
            let delta_x = parse_deltas(&tc.delta_x);

            let tc_x = (bx + tc.x) * scale;
            let tc_y = (by + tc.y) * scale;

            let chars: Vec<char> = content.chars().collect();
            let mut current_x = tc_x;

            for (i, ch) in chars.iter().enumerate() {
                if *ch != ' ' && *ch != '\u{3000}' && *ch != '\u{00A0}' {
                    result.text_layer.push(TextItem {
                        text: ch.to_string(),
                        x: current_x,
                        y: tc_y,
                        font_size,
                        font_family: font_family.clone(),
                        font_id: font_id.clone(),
                        color: color.clone(),
                        ctm: ctm.clone(),
                        stroke: text.stroke,
                        stroke_color: stroke_color.clone(),
                        line_width,
                        fill: should_fill,
                    });
                }

                // 计算下一个字符位置
                if i < delta_x.len() {
                    let mut delta = delta_x[i] * scale;
                    if let Some(ref c) = ctm {
                        if !c.is_empty() && c[0] != 0.0 {
                            delta *= c[0];
                        }
                    }
                    current_x += delta;
                } else if i < chars.len() - 1 {
                    let mut default_width = text.size * scale;
                    if let Some(ref c) = ctm {
                        if !c.is_empty() && c[0] != 0.0 {
                            default_width *= c[0];
                        }
                    }
                    current_x += default_width;
                }
            }
        }
    }

    /// 获取所有字体信息
    pub fn get_fonts(&mut self) -> Vec<FontInfo> {
        self.load_resources();

        self.fonts
            .iter()
            .map(|(id, font)| {
                let mut info = FontInfo {
                    id: id.clone(),
                    font_name: font.font_name.clone(),
                    family_name: font.family_name.clone(),
                    data_url: None,
                    has_file: false,
                };

                // 优先使用已加载的字体文件数据
                if let Some(font_data) = self.font_files.get(id) {
                    info.has_file = !font_data.is_empty();
                    if let Some(mime_type) = detect_font_mime(font_data) {
                        info.data_url = Some(format!(
                            "data:{};base64,{}",
                            mime_type,
                            BASE64.encode(font_data)
                        ));
                    }
                } else if !font.font_file.is_empty() {
                    if let Ok(font_data) = self.read_file(&font.font_file) {
                        info.has_file = !font_data.is_empty();
                        if let Some(mime_type) = detect_font_mime(&font_data) {
                            info.data_url = Some(format!(
                                "data:{};base64,{}",
                                mime_type,
                                BASE64.encode(&font_data)
                            ));
                        }
                    }
                }

                info
            })
            .collect()
    }
}

/// 检测字体MIME类型
pub fn detect_font_mime(data: &[u8]) -> Option<&'static str> {
    let mime = match data.get(0..4)? {
        [0x00, 0x01, 0x00, 0x00] | b"true" | b"typ1" => {
            if !has_browser_safe_sfnt_table_directory(data) {
                return None;
            }
            "font/ttf"
        }
        b"OTTO" => {
            if !has_browser_safe_sfnt_table_directory(data) {
                return None;
            }
            "font/otf"
        }
        b"wOFF" => "font/woff",
        b"wOF2" => "font/woff2",
        _ => return None,
    };

    ttf_parser::Face::parse(data, 0).ok()?;
    Some(mime)
}

fn has_browser_safe_sfnt_table_directory(data: &[u8]) -> bool {
    if data.len() < 12 {
        return false;
    }

    let num_tables = u16::from_be_bytes([data[4], data[5]]) as usize;
    let table_dir_len = match num_tables.checked_mul(16).and_then(|len| len.checked_add(12)) {
        Some(len) => len,
        None => return false,
    };
    if data.len() < table_dir_len {
        return false;
    }

    let mut previous_tag: Option<[u8; 4]> = None;
    let mut has_os2 = false;
    for i in 0..num_tables {
        let offset = 12 + i * 16;
        let tag = [
            data[offset],
            data[offset + 1],
            data[offset + 2],
            data[offset + 3],
        ];
        if previous_tag.is_some_and(|previous| previous >= tag) {
            return false;
        }
        if &tag == b"OS/2" {
            has_os2 = true;
        }
        previous_tag = Some(tag);
    }

    has_os2
}

/// 路径扫描器
struct PathScanner<'a> {
    data: &'a [u8],
    pos: usize,
}

impl<'a> PathScanner<'a> {
    fn new(s: &'a str) -> Self {
        PathScanner {
            data: s.as_bytes(),
            pos: 0,
        }
    }

    fn skip_whitespace(&mut self) {
        while self.pos < self.data.len() {
            let c = self.data[self.pos];
            if c == b' ' || c == b'\t' || c == b'\n' || c == b'\r' || c == b',' {
                self.pos += 1;
            } else {
                break;
            }
        }
    }

    fn peek_command(&mut self) -> Option<u8> {
        self.skip_whitespace();
        if self.pos >= self.data.len() {
            return None;
        }
        let c = self.data[self.pos];
        if c.is_ascii_alphabetic() {
            Some(c)
        } else {
            None
        }
    }

    fn next_command(&mut self) -> Option<u8> {
        self.skip_whitespace();
        if self.pos >= self.data.len() {
            return None;
        }
        let c = self.data[self.pos];
        if c.is_ascii_alphabetic() {
            self.pos += 1;
            Some(c)
        } else {
            None
        }
    }

    fn next_float(&mut self) -> Option<f64> {
        self.skip_whitespace();
        if self.pos >= self.data.len() {
            return None;
        }

        let c = self.data[self.pos];
        if c.is_ascii_alphabetic() {
            return None;
        }

        let start = self.pos;

        if self.pos < self.data.len()
            && (self.data[self.pos] == b'-' || self.data[self.pos] == b'+')
        {
            self.pos += 1;
        }

        while self.pos < self.data.len() && self.data[self.pos].is_ascii_digit() {
            self.pos += 1;
        }

        if self.pos < self.data.len() && self.data[self.pos] == b'.' {
            self.pos += 1;
            while self.pos < self.data.len() && self.data[self.pos].is_ascii_digit() {
                self.pos += 1;
            }
        }

        if self.pos < self.data.len()
            && (self.data[self.pos] == b'e' || self.data[self.pos] == b'E')
        {
            self.pos += 1;
            if self.pos < self.data.len()
                && (self.data[self.pos] == b'-' || self.data[self.pos] == b'+')
            {
                self.pos += 1;
            }
            while self.pos < self.data.len() && self.data[self.pos].is_ascii_digit() {
                self.pos += 1;
            }
        }

        if self.pos == start {
            return None;
        }

        std::str::from_utf8(&self.data[start..self.pos])
            .ok()
            .and_then(|s| s.parse().ok())
    }

    fn has_more(&mut self) -> bool {
        self.skip_whitespace();
        self.pos < self.data.len()
    }
}

/// 转换OFD路径命令为Canvas命令JSON
pub(crate) fn convert_ofd_path_to_canvas(
    data: &str,
    scale: f64,
    offset_x: f64,
    offset_y: f64,
    ctm: &[f64],
    _page_width: f64,
    _page_height: f64,
) -> String {
    let mut commands: Vec<serde_json::Value> = Vec::new();

    let transform_point = |x: f64, y: f64| -> (f64, f64) {
        if ctm.len() >= 6 {
            let tx = ctm[0] * x + ctm[2] * y + ctm[4] + offset_x;
            let ty = ctm[1] * x + ctm[3] * y + ctm[5] + offset_y;
            (tx * scale, ty * scale)
        } else {
            ((offset_x + x) * scale, (offset_y + y) * scale)
        }
    };

    let mut scanner = PathScanner::new(data);
    let mut current_cmd: u8 = 0;
    let mut _start_x = 0.0;
    let mut _start_y = 0.0;
    let mut _current_x = 0.0;
    let mut _current_y = 0.0;
    let mut _has_start = false;

    while scanner.has_more() {
        if let Some(cmd) = scanner.peek_command() {
            scanner.next_command();
            current_cmd = cmd;
        }

        match current_cmd {
            b'S' | b'M' | b's' | b'm' => {
                if let (Some(x), Some(y)) = (scanner.next_float(), scanner.next_float()) {
                    let (tx, ty) = transform_point(x, y);
                    commands.push(serde_json::json!({"cmd": "M", "x": tx, "y": ty}));
                    _start_x = tx;
                    _start_y = ty;
                    _current_x = tx;
                    _current_y = ty;
                    _has_start = true;
                    if current_cmd == b'M' || current_cmd == b'm' {
                        current_cmd = b'L';
                    }
                }
            }
            b'L' | b'l' => {
                if let (Some(x), Some(y)) = (scanner.next_float(), scanner.next_float()) {
                    let (tx, ty) = transform_point(x, y);
                    commands.push(serde_json::json!({"cmd": "L", "x": tx, "y": ty}));
                    _current_x = tx;
                    _current_y = ty;
                }
            }
            b'B' | b'b' => {
                if let (Some(x1), Some(y1), Some(x2), Some(y2), Some(x3), Some(y3)) = (
                    scanner.next_float(),
                    scanner.next_float(),
                    scanner.next_float(),
                    scanner.next_float(),
                    scanner.next_float(),
                    scanner.next_float(),
                ) {
                    let (tx1, ty1) = transform_point(x1, y1);
                    let (tx2, ty2) = transform_point(x2, y2);
                    let (tx3, ty3) = transform_point(x3, y3);
                    commands.push(serde_json::json!({
                        "cmd": "C",
                        "x1": tx1, "y1": ty1,
                        "x2": tx2, "y2": ty2,
                        "x": tx3, "y": ty3
                    }));
                    _current_x = tx3;
                    _current_y = ty3;
                }
            }
            b'Q' | b'q' => {
                if let (Some(x1), Some(y1), Some(x2), Some(y2)) = (
                    scanner.next_float(),
                    scanner.next_float(),
                    scanner.next_float(),
                    scanner.next_float(),
                ) {
                    let (tx1, ty1) = transform_point(x1, y1);
                    let (tx2, ty2) = transform_point(x2, y2);
                    commands.push(serde_json::json!({
                        "cmd": "Q",
                        "x1": tx1, "y1": ty1,
                        "x": tx2, "y": ty2
                    }));
                    _current_x = tx2;
                    _current_y = ty2;
                }
            }
            b'C' | b'c' | b'Z' | b'z' => {
                commands.push(serde_json::json!({"cmd": "Z"}));
                current_cmd = 0;
                _has_start = false;
            }
            b'A' | b'a' => {
                for _ in 0..7 {
                    scanner.next_float();
                }
            }
            _ => {
                scanner.next_float();
            }
        }
    }

    serde_json::to_string(&commands).unwrap_or_else(|_| "[]".to_string())
}

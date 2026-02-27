//! 页面结构定义

use serde::{Deserialize, Serialize};

/// 页面结构
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
#[serde(rename = "Page")]
pub struct Page {
    #[serde(rename = "Area", default)]
    pub area: PageAreaDef,
    #[serde(rename = "Content", default)]
    pub content: Content,
    #[serde(rename = "Layer", default)]
    pub layer: Vec<Layer>,
    #[serde(rename = "Template", default)]
    pub template: Vec<Template>,
}

/// 页面区域定义
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct PageAreaDef {
    #[serde(rename = "PhysicalBox", default)]
    pub physical_box: String,
    #[serde(rename = "ApplicationBox", default)]
    pub application_box: String,
    #[serde(rename = "ContentBox", default)]
    pub content_box: String,
}

/// 模板引用
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct Template {
    #[serde(rename = "@TemplateID", default)]
    pub template_id: String,
    #[serde(rename = "@ZOrder", default)]
    pub z_order: String,
}

/// 页面内容
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct Content {
    #[serde(rename = "Layer", default)]
    pub layer: Vec<Layer>,
}

/// 图层
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct Layer {
    #[serde(rename = "@ID", default)]
    pub id: String,
    #[serde(rename = "@Type", default)]
    pub layer_type: String,
    #[serde(rename = "@DrawParam", default)]
    pub draw_param: String,
    #[serde(rename = "$value", default)]
    pub objects: Vec<LayerObject>,
}

/// 图层对象（可以是文本、路径或图片）
#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(rename_all = "PascalCase")]
pub enum LayerObject {
    TextObject(TextObject),
    PathObject(PathObject),
    ImageObject(ImageObject),
    CompositeObject(CompositeObject),
}

/// 复合对象引用
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct CompositeObject {
    #[serde(rename = "@ID", default)]
    pub id: String,
    #[serde(rename = "@Boundary", default)]
    pub boundary: String,
    #[serde(rename = "@ResourceID", default)]
    pub resource_id: String,
    #[serde(rename = "@CTM", default)]
    pub ctm: String,
}

impl Layer {
    pub fn text_objects(&self) -> impl Iterator<Item = &TextObject> {
        self.objects.iter().filter_map(|o| match o {
            LayerObject::TextObject(t) => Some(t),
            _ => None,
        })
    }

    pub fn path_objects(&self) -> impl Iterator<Item = &PathObject> {
        self.objects.iter().filter_map(|o| match o {
            LayerObject::PathObject(p) => Some(p),
            _ => None,
        })
    }

    pub fn image_objects(&self) -> impl Iterator<Item = &ImageObject> {
        self.objects.iter().filter_map(|o| match o {
            LayerObject::ImageObject(i) => Some(i),
            _ => None,
        })
    }

    pub fn composite_objects(&self) -> impl Iterator<Item = &CompositeObject> {
        self.objects.iter().filter_map(|o| match o {
            LayerObject::CompositeObject(c) => Some(c),
            _ => None,
        })
    }
}

/// 文本对象
#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct TextObject {
    #[serde(rename = "@ID", default)]
    pub id: String,
    #[serde(rename = "@Boundary", default)]
    pub boundary: String,
    #[serde(rename = "@Font", default)]
    pub font: String,
    #[serde(rename = "@Size", default)]
    pub size: f64,
    #[serde(rename = "@HScale", default)]
    pub h_scale: f64,
    #[serde(rename = "@Weight", default)]
    pub weight: i32,
    #[serde(rename = "@Italic", default)]
    pub italic: bool,
    #[serde(rename = "@Stroke", default)]
    pub stroke: bool,
    #[serde(rename = "@Fill", default = "default_true")]
    pub fill: bool,
    #[serde(rename = "@LineWidth", default = "default_line_width")]
    pub line_width: f64,
    #[serde(rename = "@CTM", default)]
    pub ctm: String,
    #[serde(rename = "@DrawParam", default)]
    pub draw_param: String,
    #[serde(rename = "@Alpha", default = "default_alpha")]
    pub alpha: i32,
    #[serde(rename = "FillColor", default)]
    pub fill_color: Option<Color>,
    #[serde(rename = "StrokeColor", default)]
    pub stroke_color: Option<Color>,
    #[serde(rename = "TextCode", default)]
    pub text_code: Vec<TextCode>,
    #[serde(rename = "CGTransform", default)]
    pub cg_transform: Vec<CGTransform>,
}

impl Default for TextObject {
    fn default() -> Self {
        TextObject {
            id: String::new(),
            boundary: String::new(),
            font: String::new(),
            size: 0.0,
            h_scale: 0.0,
            weight: 0,
            italic: false,
            stroke: false,
            fill: true,
            line_width: 0.353,
            ctm: String::new(),
            draw_param: String::new(),
            alpha: 255,
            fill_color: None,
            stroke_color: None,
            text_code: Vec::new(),
            cg_transform: Vec::new(),
        }
    }
}

fn default_true() -> bool { true }
fn default_line_width() -> f64 { 0.353 }
fn default_alpha() -> i32 { 255 }

/// 文本内容
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct TextCode {
    #[serde(rename = "@X", default)]
    pub x: f64,
    #[serde(rename = "@Y", default)]
    pub y: f64,
    #[serde(rename = "@DeltaX", default)]
    pub delta_x: String,
    #[serde(rename = "@DeltaY", default)]
    pub delta_y: String,
    #[serde(rename = "$text", default)]
    pub content: String,
}

/// 字形变换
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct CGTransform {
    #[serde(rename = "@CodePosition", default)]
    pub code_position: i32,
    #[serde(rename = "@CodeCount", default)]
    pub code_count: i32,
    #[serde(rename = "@GlyphCount", default)]
    pub glyph_count: i32,
    #[serde(rename = "Glyphs", default)]
    pub glyphs: String,
}

/// 路径对象
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct PathObject {
    #[serde(rename = "@ID", default)]
    pub id: String,
    #[serde(rename = "@Boundary", default)]
    pub boundary: String,
    #[serde(rename = "@CTM", default)]
    pub ctm: String,
    #[serde(rename = "@LineWidth", default)]
    pub line_width: f64,
    #[serde(rename = "@Join", default)]
    pub join: String,
    #[serde(rename = "@Cap", default)]
    pub cap: String,
    #[serde(rename = "@Stroke", default = "default_true")]
    pub stroke: bool,
    #[serde(rename = "@Fill", default)]
    pub fill: bool,
    #[serde(rename = "@DrawParam", default)]
    pub draw_param: String,
    #[serde(rename = "FillColor", default)]
    pub fill_color: Option<ColorOrShd>,
    #[serde(rename = "StrokeColor", default)]
    pub stroke_color: Option<Color>,
    #[serde(rename = "AbbreviatedData", default)]
    pub abbreviated_data: String,
}

/// 图像对象
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct ImageObject {
    #[serde(rename = "@ID", default)]
    pub id: String,
    #[serde(rename = "@Boundary", default)]
    pub boundary: String,
    #[serde(rename = "@ResourceID", default)]
    pub resource_id: String,
    #[serde(rename = "@CTM", default)]
    pub ctm: String,
    #[serde(rename = "@Alpha", default = "default_alpha")]
    pub alpha: i32,
}

/// 颜色
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct Color {
    #[serde(rename = "@Value", default)]
    pub value: String,
    #[serde(rename = "@ColorSpace", default)]
    pub color_space: String,
}

/// 颜色或渐变
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct ColorOrShd {
    #[serde(rename = "@Value", default)]
    pub value: String,
    #[serde(rename = "AxialShd", default)]
    pub axial_shd: Option<AxialShd>,
    #[serde(rename = "RadialShd", default)]
    pub radial_shd: Option<RadialShd>,
    #[serde(rename = "Pattern", default)]
    pub pattern: Option<Pattern>,
}

/// 轴向渐变
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct AxialShd {
    #[serde(rename = "@StartPoint", default)]
    pub start_point: String,
    #[serde(rename = "@EndPoint", default)]
    pub end_point: String,
    #[serde(rename = "Segment", default)]
    pub segment: Vec<Segment>,
}

/// 径向渐变
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct RadialShd {
    #[serde(rename = "@StartPoint", default)]
    pub start_point: String,
    #[serde(rename = "@EndPoint", default)]
    pub end_point: String,
    #[serde(rename = "@StartRadius", default)]
    pub start_radius: f64,
    #[serde(rename = "@EndRadius", default)]
    pub end_radius: f64,
    #[serde(rename = "Segment", default)]
    pub segment: Vec<Segment>,
}

/// 渐变段
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct Segment {
    #[serde(rename = "@Position", default)]
    pub position: f64,
    #[serde(rename = "Color", default)]
    pub color: Color,
}

/// 资源文件
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
#[serde(rename = "Res")]
pub struct Res {
    #[serde(rename = "@BaseLoc", default)]
    pub base_loc: String,
    #[serde(rename = "Fonts", default)]
    pub fonts: Option<Fonts>,
    #[serde(rename = "MultiMedias", default)]
    pub multi_medias: Option<MultiMedias>,
    #[serde(rename = "CompositeGraphicUnits", default)]
    pub composite_graphic_units: Option<CompositeGraphicUnits>,
    /// DrawParam 直接作为 Res 的子元素（无 DrawParams 包装）
    #[serde(rename = "DrawParam", default)]
    pub draw_params: Vec<DrawParam>,
    /// DrawParams 包装元素（部分 OFD 文件使用）
    #[serde(rename = "DrawParams", default)]
    pub draw_params_wrapped: Option<DrawParamsWrapper>,
}

/// DrawParams 包装元素
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct DrawParamsWrapper {
    #[serde(rename = "DrawParam", default)]
    pub draw_param: Vec<DrawParam>,
}

/// 绘制参数（定义默认描边/填充样式）
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct DrawParam {
    #[serde(rename = "@ID", default)]
    pub id: String,
    #[serde(rename = "@Relative", default)]
    pub relative: String,
    #[serde(rename = "@LineWidth", default)]
    pub line_width: f64,
    #[serde(rename = "@Join", default)]
    pub join: String,
    #[serde(rename = "@Cap", default)]
    pub cap: String,
    #[serde(rename = "FillColor", default)]
    pub fill_color: Option<Color>,
    #[serde(rename = "StrokeColor", default)]
    pub stroke_color: Option<Color>,
}

#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct CompositeGraphicUnits {
    #[serde(rename = "CompositeGraphicUnit", default)]
    pub units: Vec<CompositeGraphicUnit>,
}

/// 复合图元定义（如印章矢量图形）
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct CompositeGraphicUnit {
    #[serde(rename = "@ID", default)]
    pub id: String,
    #[serde(rename = "@Width", default)]
    pub width: f64,
    #[serde(rename = "@Height", default)]
    pub height: f64,
    #[serde(rename = "Content", default)]
    pub content: CompositeContent,
}

/// 复合图元内容
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct CompositeContent {
    #[serde(rename = "@ID", default)]
    pub id: String,
    #[serde(rename = "@Type", default)]
    pub content_type: String,
    #[serde(rename = "PageBlock", default)]
    pub page_block: Option<CompositePageBlock>,
}

/// 复合图元页面块
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct CompositePageBlock {
    #[serde(rename = "@ID", default)]
    pub id: String,
    #[serde(rename = "$value", default)]
    pub objects: Vec<LayerObject>,
}

#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct Fonts {
    #[serde(rename = "Font", default)]
    pub font: Vec<Font>,
}

#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct MultiMedias {
    #[serde(rename = "MultiMedia", default)]
    pub multi_media: Vec<MultiMedia>,
}

/// 字体
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct Font {
    #[serde(rename = "@ID", default)]
    pub id: String,
    #[serde(rename = "@FontName", default)]
    pub font_name: String,
    #[serde(rename = "@FamilyName", default)]
    pub family_name: String,
    #[serde(rename = "FontFile", default)]
    pub font_file: String,
}

/// 多媒体资源
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct MultiMedia {
    #[serde(rename = "@ID", default)]
    pub id: String,
    #[serde(rename = "@Type", default)]
    pub media_type: String,
    #[serde(rename = "MediaFile", default)]
    pub media_file: String,
}

/// 签章结构
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
#[serde(rename = "Signatures")]
pub struct Signatures {
    #[serde(rename = "Signature", default)]
    pub signature: Vec<Signature>,
}

/// 签章
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct Signature {
    #[serde(rename = "@ID", default)]
    pub id: String,
    #[serde(rename = "@Type", default)]
    pub sig_type: String,
    #[serde(rename = "@BaseLoc", default)]
    pub base_loc: String,
}

/// 签章XML
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
#[serde(rename = "Signature")]
pub struct SignatureXML {
    #[serde(rename = "SignedInfo", default)]
    pub signed_info: SignedInfo,
    #[serde(rename = "SignedValue", default)]
    pub signed_value: String,
}

/// 签章信息
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct SignedInfo {
    #[serde(rename = "StampAnnot", default)]
    pub stamp_annot: Vec<StampAnnot>,
    #[serde(rename = "Provider", default)]
    pub provider: String,
    #[serde(rename = "SignatureMethod", default)]
    pub signature_method: String,
}

/// 印章注释
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct StampAnnot {
    #[serde(rename = "@ID", default)]
    pub id: String,
    #[serde(rename = "@PageRef", default)]
    pub page_ref: String,
    #[serde(rename = "@Boundary", default)]
    pub boundary: String,
}

/// Pattern 图案填充
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct Pattern {
    #[serde(rename = "@Width", default)]
    pub width: f64,
    #[serde(rename = "@Height", default)]
    pub height: f64,
    #[serde(rename = "@XStep", default)]
    pub x_step: f64,
    #[serde(rename = "@YStep", default)]
    pub y_step: f64,
    #[serde(rename = "@RelativeTo", default)]
    pub relative_to: String,
    #[serde(rename = "@CTM", default)]
    pub ctm: String,
    #[serde(rename = "CellContent", default)]
    pub cell_content: CellContent,
}

/// CellContent Pattern 单元格内容
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct CellContent {
    #[serde(rename = "ImageObject", default)]
    pub image_objects: Vec<ImageObject>,
    #[serde(rename = "PathObject", default)]
    pub path_objects: Vec<PathObject>,
    #[serde(rename = "TextObject", default)]
    pub text_objects: Vec<TextObject>,
}

/// 页面注释列表
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
#[serde(rename = "PageAnnot")]
pub struct PageAnnot {
    #[serde(rename = "Annot", default)]
    pub annots: Vec<AnnotElement>,
}

/// 注释元素
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct AnnotElement {
    #[serde(rename = "@ID", default)]
    pub id: String,
    #[serde(rename = "@Type", default)]
    pub annot_type: String,
    #[serde(rename = "@Creator", default)]
    pub creator: String,
    #[serde(rename = "@Subtype", default)]
    pub subtype: String,
    #[serde(rename = "Appearance", default)]
    pub appearance: AnnotAppearance,
}

/// 注释外观
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct AnnotAppearance {
    #[serde(rename = "@Boundary", default)]
    pub boundary: String,
    #[serde(rename = "PageBlock", default)]
    pub page_blocks: Vec<AnnotPageBlock>,
    /// 直接嵌在 Appearance 下的对象（无 PageBlock 包裹）
    #[serde(rename = "TextObject", default)]
    pub text_objects: Vec<TextObject>,
    #[serde(rename = "PathObject", default)]
    pub path_objects: Vec<PathObject>,
    #[serde(rename = "ImageObject", default)]
    pub image_objects: Vec<ImageObject>,
}

/// 注释页面块
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct AnnotPageBlock {
    #[serde(rename = "@ID", default)]
    pub id: String,
    #[serde(rename = "$value", default)]
    pub objects: Vec<LayerObject>,
}

/// 印章数据 (Seal.xml)
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
#[serde(rename = "Seal")]
pub struct SealXML {
    #[serde(rename = "SealID", default)]
    pub seal_id: String,
    #[serde(rename = "Type", default)]
    pub seal_type: String,
    #[serde(rename = "Picture", default)]
    pub picture: SealPicture,
    #[serde(rename = "SealData", default)]
    pub seal_data: String,
}

/// 印章图片
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct SealPicture {
    #[serde(rename = "@Type", default)]
    pub picture_type: String,
    #[serde(rename = "@Width", default)]
    pub width: f64,
    #[serde(rename = "@Height", default)]
    pub height: f64,
    #[serde(rename = "$text", default)]
    pub data: String,
}


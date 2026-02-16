//! OFD 解析器

use std::collections::HashMap;
use std::io::Read;
use zip::ZipArchive;
use regex::Regex;

use crate::types::*;
use crate::page::*;

/// 解析结果
#[derive(Debug, Default)]
pub struct ParseResult {
    pub ofd: Option<OFDDocument>,
    pub document: Option<Document>,
    pub files: Vec<String>,
    pub page_count: usize,
    pub error: Option<String>,
}

/// OFD 解析器
pub struct Parser {
    pub(crate) files: Vec<String>,
    file_contents: HashMap<String, Vec<u8>>,
    pub ofd: Option<OFDDocument>,
    pub document: Option<Document>,
    pub fonts: HashMap<String, Font>,
    pub images: HashMap<String, Vec<u8>>,
    pub font_files: HashMap<String, Vec<u8>>,
    pub composite_units: HashMap<String, CompositeGraphicUnit>,
}

impl Parser {
    /// 创建解析器
    pub fn new(data: Vec<u8>) -> Result<Self, String> {
        let data_len = data.len();
        let cursor = std::io::Cursor::new(data.clone());
        let mut archive = ZipArchive::new(cursor)
            .map_err(|e| format!("无法打开ZIP文件: {} (size: {})", e, data_len))?;

        let mut files = Vec::new();
        let mut file_contents = HashMap::new();

        for i in 0..archive.len() {
            let mut file = archive.by_index(i)
                .map_err(|e| format!("读取ZIP条目失败: {}", e))?;
            let name = file.name().to_string();
            files.push(name.clone());

            let mut contents = Vec::new();
            file.read_to_end(&mut contents)
                .map_err(|e| format!("读取文件内容失败: {}", e))?;
            file_contents.insert(name, contents);
        }

        Ok(Parser {
            files,
            file_contents,
            ofd: None,
            document: None,
            fonts: HashMap::new(),
            images: HashMap::new(),
            font_files: HashMap::new(),
            composite_units: HashMap::new(),
        })
    }

    /// 解析OFD文件
    pub fn parse(&mut self) -> Result<ParseResult, String> {
        let mut result = ParseResult {
            files: self.files.clone(),
            ..Default::default()
        };

        // 解析 OFD.xml
        let ofd_data = match self.read_file("OFD.xml") {
            Ok(d) => d,
            Err(e) => {
                result.error = Some(format!("无法读取OFD.xml: {}", e));
                return Ok(result);
            }
        };
        
        let ofd_xml = Self::remove_namespace_prefix(&String::from_utf8_lossy(&ofd_data));
        
        self.ofd = match quick_xml::de::from_str(&ofd_xml) {
            Ok(o) => Some(o),
            Err(e) => {
                result.error = Some(format!("解析OFD.xml失败: {}", e));
                return Ok(result);
            }
        };
        result.ofd = self.ofd.clone();

        // 解析 Document.xml
        if let Some(ref ofd) = self.ofd {
            if !ofd.doc_body.is_empty() {
                let doc_root = ofd.doc_body[0].doc_root.trim_start_matches('/');
                
                let doc_data = self.read_file(doc_root)
                    .or_else(|_| self.read_file("Doc_0/Document.xml"));

                if let Ok(data) = doc_data {
                    let doc_xml = Self::remove_namespace_prefix(&String::from_utf8_lossy(&data));
                    let doc_xml = Self::dedup_xml_elements(&doc_xml);
                    self.document = match quick_xml::de::from_str(&doc_xml) {
                        Ok(d) => Some(d),
                        Err(e) => {
                            result.error = Some(format!("解析Document.xml失败: {}", e));
                            return Ok(result);
                        }
                    };
                    
                    if let Some(ref doc) = self.document {
                        result.page_count = doc.pages.page.len();
                        // 调试输出
                        web_sys::console::log_1(&format!(
                            "Document parsed: page_count={}, physical_box='{}'",
                            doc.pages.page.len(),
                            doc.common_data.page_area.physical_box
                        ).into());
                    }
                    result.document = self.document.clone();
                } else {
                    result.error = Some("无法读取Document.xml".to_string());
                }
            }
        }

        Ok(result)
    }

    /// 读取ZIP中的文件
    pub fn read_file(&self, name: &str) -> Result<Vec<u8>, String> {
        let name = name.trim_start_matches('/');
        let name_lower = name.to_lowercase();

        // 精确匹配
        if let Some(data) = self.file_contents.get(name) {
            return Ok(data.clone());
        }

        // 大小写不敏感匹配
        for (key, data) in &self.file_contents {
            if key.eq_ignore_ascii_case(name) {
                return Ok(data.clone());
            }
        }

        // 后缀匹配
        for (key, data) in &self.file_contents {
            let key_lower = key.to_lowercase();
            if key_lower.ends_with(&name_lower) || key_lower.ends_with(&format!("/{}", name_lower)) {
                return Ok(data.clone());
            }
        }

        // 基础文件名匹配
        let base_name = std::path::Path::new(&name_lower)
            .file_name()
            .and_then(|s| s.to_str())
            .unwrap_or(&name_lower);

        for (key, data) in &self.file_contents {
            let key_base = std::path::Path::new(key)
                .file_name()
                .and_then(|s| s.to_str())
                .unwrap_or(key);
            if key_base.eq_ignore_ascii_case(base_name) {
                return Ok(data.clone());
            }
        }

        Err(format!("文件不存在: {}", name))
    }

    /// 移除XML命名空间前缀
    pub fn remove_namespace_prefix(xml_str: &str) -> String {
        let re1 = Regex::new(r"<(/?)ofd:").unwrap();
        let result = re1.replace_all(xml_str, "<$1");
        
        let re2 = Regex::new(r#"\s+xmlns:[^=]+="[^"]*""#).unwrap();
        let result = re2.replace_all(&result, "").to_string();

        // 保护 TextCode 中的文本内容不被 quick_xml trim
        // 将 <TextCode ...>text</TextCode> 中的文本空格替换为 \u{00A0}(NBSP)
        // 这样 quick_xml 的 $text 不会 trim 掉它们
        Self::preserve_textcode_spaces(&result)
    }

    /// 保护 TextCode 元素中的前导/尾部空格
    fn preserve_textcode_spaces(xml_str: &str) -> String {
        let re = Regex::new(r"(?s)(<TextCode[^>]*>)(.*?)(</TextCode>)").unwrap();
        re.replace_all(xml_str, |caps: &regex::Captures| {
            let open_tag = &caps[1];
            let text = &caps[2];
            let close_tag = &caps[3];

            // 如果文本中包含子元素（如 CGTransform），不处理
            if text.contains('<') {
                return format!("{}{}{}", open_tag, text, close_tag);
            }

            // 将前导和尾部的普通空格替换为 NBSP(\u{00A0})
            let chars: Vec<char> = text.chars().collect();
            let mut result_chars = chars.clone();

            // 前导空格
            for i in 0..result_chars.len() {
                if result_chars[i] == ' ' {
                    result_chars[i] = '\u{00A0}';
                } else {
                    break;
                }
            }
            // 尾部空格
            for i in (0..result_chars.len()).rev() {
                if result_chars[i] == ' ' {
                    result_chars[i] = '\u{00A0}';
                } else {
                    break;
                }
            }

            let preserved: String = result_chars.into_iter().collect();
            format!("{}{}{}", open_tag, preserved, close_tag)
        }).to_string()
    }

    /// 合并重复的XML元素（如多个 PublicRes、DocumentRes）
    /// quick-xml 反序列化不支持重复同名元素，去除重复只保留第一个
    pub fn dedup_xml_elements(xml_str: &str) -> String {
        let mut result = xml_str.to_string();
        let tags = ["PublicRes", "DocumentRes"];
        for tag in &tags {
            let pattern = format!(r"<{0}>[^<]*</{0}>", tag);
            let re = match Regex::new(&pattern) {
                Ok(r) => r,
                Err(_) => continue,
            };
            let matches: Vec<_> = re.find_iter(&result).map(|m| (m.start(), m.end())).collect();
            if matches.len() > 1 {
                for &(start, end) in matches[1..].iter().rev() {
                    result.replace_range(start..end, "");
                }
            }
        }
        result
    }

    /// 获取文件列表
    pub fn get_files(&self) -> &[String] {
        &self.files
    }

    /// 获取文档信息
    pub fn get_doc_info(&self) -> Option<&DocInfo> {
        self.ofd.as_ref()
            .and_then(|ofd| ofd.doc_body.first())
            .map(|body| &body.doc_info)
    }

    /// 获取页数
    pub fn get_page_count(&self) -> usize {
        self.document.as_ref()
            .map(|doc| doc.pages.page.len())
            .unwrap_or(0)
    }

    /// 获取页面文件路径
    pub fn get_page_path(&self, index: usize) -> Option<String> {
        let doc = self.document.as_ref()?;
        let page_ref = doc.pages.page.get(index)?;
        let page_loc = page_ref.base_loc.trim_start_matches('/');

        // 获取文档根目录
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

        // 尝试多种路径组合
        let candidates = vec![
            format!("{}/{}", doc_base, page_loc),
            page_loc.to_string(),
            format!("{}/Pages/{}", doc_base, page_loc),
        ];

        for candidate in &candidates {
            for file in &self.files {
                if file.eq_ignore_ascii_case(candidate) || 
                   file.to_lowercase().ends_with(&format!("/{}", std::path::Path::new(candidate).file_name()?.to_str()?)) {
                    return Some(file.clone());
                }
            }
        }

        // 按页面索引查找
        let page_pattern = format!("page_{}/content.xml", index);
        for file in &self.files {
            let lower = file.to_lowercase();
            if lower.contains(&page_pattern) || lower.contains(&format!("page_{}.xml", index)) {
                return Some(file.clone());
            }
        }

        Some(format!("{}/{}", doc_base, page_loc))
    }

    /// 获取页面尺寸
    pub fn get_page_size(&self, index: usize) -> (f64, f64) {
        let page_path = match self.get_page_path(index) {
            Some(p) => p,
            None => return (210.0, 297.0),
        };

        let page_data = match self.read_file(&page_path) {
            Ok(d) => d,
            Err(_) => return (210.0, 297.0),
        };

        let page_xml = Self::remove_namespace_prefix(&String::from_utf8_lossy(&page_data));
        let page: Page = match quick_xml::de::from_str(&page_xml) {
            Ok(p) => p,
            Err(_) => return (210.0, 297.0),
        };

        if !page.area.physical_box.is_empty() {
            return parse_box(&page.area.physical_box);
        }

        if let Some(ref doc) = self.document {
            if !doc.common_data.page_area.physical_box.is_empty() {
                return parse_box(&doc.common_data.page_area.physical_box);
            }
        }

        (210.0, 297.0)
    }

    /// 加载资源
    pub fn load_resources(&mut self) {
        if !self.fonts.is_empty() {
            return;
        }

        // 获取文档根目录
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

        let files_clone: Vec<String> = self.files.clone();
        
        for file in &files_clone {
            let lower = file.to_lowercase();
            if !lower.ends_with("publicres.xml") && 
               !lower.ends_with("documentres.xml") &&
               !(lower.contains("res") && lower.ends_with(".xml")) {
                continue;
            }

            let data = match self.read_file(file) {
                Ok(d) => d,
                Err(_) => continue,
            };

            let xml_str = Self::remove_namespace_prefix(&String::from_utf8_lossy(&data));
            let res: Res = match quick_xml::de::from_str(&xml_str) {
                Ok(r) => r,
                Err(_) => continue,
            };

            let base_path = std::path::Path::new(file)
                .parent()
                .and_then(|p| p.to_str())
                .unwrap_or("");

            // 加载字体
            if let Some(fonts) = &res.fonts {
                for font in &fonts.font {
                    let mut font_clone = font.clone();
                    if !font.font_file.is_empty() {
                        font_clone.font_file = format!("{}/{}/{}", base_path, res.base_loc, font.font_file);
                        // 预加载字体文件数据，标记该字体有嵌入文件
                        match self.read_file(&font_clone.font_file) {
                            Ok(font_data) if !font_data.is_empty() => {
                                web_sys::console::log_1(&format!(
                                    "[资源] 字体加载成功: id={}, path='{}', size={}bytes",
                                    font.id, font_clone.font_file, font_data.len()
                                ).into());
                                self.font_files.insert(font.id.clone(), font_data);
                            }
                            Ok(_) => {
                                web_sys::console::warn_1(&format!(
                                    "[资源] 字体文件为空: id={}, path='{}'",
                                    font.id, font_clone.font_file
                                ).into());
                            }
                            Err(e) => {
                                web_sys::console::warn_1(&format!(
                                    "[资源] 字体文件读取失败: id={}, path='{}', err={}",
                                    font.id, font_clone.font_file, e
                                ).into());
                            }
                        }
                    }
                    self.fonts.insert(font.id.clone(), font_clone);
                }
            }

            // 加载图片路径
            if let Some(medias) = &res.multi_medias {
                for media in &medias.multi_media {
                    if media.media_type == "Image" {
                        let img_path = format!("{}/{}/{}", base_path, res.base_loc, media.media_file);
                        self.images.insert(media.id.clone(), img_path.into_bytes());
                    }
                }
            }

            // 加载复合图元
            if let Some(cgu) = &res.composite_graphic_units {
                for unit in &cgu.units {
                    self.composite_units.insert(unit.id.clone(), unit.clone());
                }
            }
        }

        let _ = doc_base;

        // 调试：输出已加载的字体信息
        web_sys::console::log_1(&format!(
            "[资源] load_resources 完成: fonts={}, font_files={}, images={}, composites={}",
            self.fonts.len(), self.font_files.len(), self.images.len(), self.composite_units.len()
        ).into());
        for (id, font) in &self.fonts {
            web_sys::console::log_1(&format!(
                "[资源] font id={}, name='{}', family='{}'",
                id, font.font_name, font.family_name
            ).into());
        }
    }

    /// 懒加载图片
    pub fn load_image_lazy(&mut self, resource_id: &str) -> Option<Vec<u8>> {
        if let Some(data) = self.images.get(resource_id) {
            if data.len() > 500 {
                return Some(data.clone());
            }
            // 是路径，尝试加载
            let img_path = String::from_utf8_lossy(data).to_string();
            if let Ok(img_data) = self.read_file(&img_path) {
                self.images.insert(resource_id.to_string(), img_data.clone());
                return Some(img_data);
            }
        }

        // 尝试直接用 ID 作为文件名查找
        let files_clone: Vec<String> = self.files.clone();
        for file in &files_clone {
            let lower = file.to_lowercase();
            if lower.contains(&resource_id.to_lowercase()) {
                if lower.ends_with(".png") || lower.ends_with(".jpg") || 
                   lower.ends_with(".jpeg") || lower.ends_with(".gif") {
                    if let Ok(img_data) = self.read_file(file) {
                        self.images.insert(resource_id.to_string(), img_data.clone());
                        return Some(img_data);
                    }
                }
            }
        }

        None
    }
}

/// 解析边界框
pub fn parse_box(box_str: &str) -> (f64, f64) {
    let mut scanner = NumberScanner::new(box_str);
    let x = scanner.next_float(); // skip x
    let y = scanner.next_float(); // skip y
    let w = scanner.next_float().unwrap_or(0.0);
    let h = scanner.next_float().unwrap_or(0.0);
    
    // 调试输出
    web_sys::console::log_1(&format!(
        "parse_box: input='{}', x={:?}, y={:?}, w={}, h={}",
        box_str, x, y, w, h
    ).into());
    
    if w == 0.0 && h == 0.0 {
        return (210.0, 297.0);
    }
    (w, h)
}

/// 解析边界
pub fn parse_boundary(boundary: &str) -> (f64, f64, f64, f64) {
    let mut scanner = NumberScanner::new(boundary);
    let x = scanner.next_float().unwrap_or(0.0);
    let y = scanner.next_float().unwrap_or(0.0);
    let w = scanner.next_float().unwrap_or(0.0);
    let h = scanner.next_float().unwrap_or(0.0);
    (x, y, w, h)
}

/// 解析颜色
pub fn parse_color(value: &str) -> String {
    let mut scanner = NumberScanner::new(value);
    if let (Some(r), Some(g), Some(b)) = (scanner.next_float(), scanner.next_float(), scanner.next_float()) {
        return format!("rgb({},{},{})", r as i32, g as i32, b as i32);
    }
    "#000".to_string()
}

/// 解析CTM变换矩阵
pub fn parse_ctm(ctm_str: &str) -> Vec<f64> {
    let mut result = Vec::with_capacity(6);
    let mut scanner = NumberScanner::new(ctm_str);
    while let Some(val) = scanner.next_float() {
        result.push(val);
        if result.len() >= 6 {
            break;
        }
    }
    if result.len() < 4 {
        return Vec::new();
    }
    result
}

/// 解析DeltaX/DeltaY
pub fn parse_deltas(delta_str: &str) -> Vec<f64> {
    let mut result = Vec::new();
    let mut scanner = TokenScanner::new(delta_str);

    while let Some(token) = scanner.next_token() {
        if token == "g" {
            if let (Some(count_str), Some(val_str)) = (scanner.next_token(), scanner.next_token()) {
                if let (Ok(count), Ok(val)) = (count_str.parse::<usize>(), val_str.parse::<f64>()) {
                    for _ in 0..count {
                        result.push(val);
                    }
                }
            }
        } else if let Ok(val) = token.parse::<f64>() {
            result.push(val);
        }
    }
    result
}

/// 数字扫描器
pub struct NumberScanner<'a> {
    data: &'a [u8],
    pos: usize,
}

impl<'a> NumberScanner<'a> {
    pub fn new(s: &'a str) -> Self {
        NumberScanner { data: s.as_bytes(), pos: 0 }
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

    pub fn next_float(&mut self) -> Option<f64> {
        self.skip_whitespace();
        if self.pos >= self.data.len() {
            return None;
        }

        let start = self.pos;
        
        // 符号
        if self.pos < self.data.len() && (self.data[self.pos] == b'-' || self.data[self.pos] == b'+') {
            self.pos += 1;
        }
        
        // 整数部分
        while self.pos < self.data.len() && self.data[self.pos].is_ascii_digit() {
            self.pos += 1;
        }
        
        // 小数部分
        if self.pos < self.data.len() && self.data[self.pos] == b'.' {
            self.pos += 1;
            while self.pos < self.data.len() && self.data[self.pos].is_ascii_digit() {
                self.pos += 1;
            }
        }
        
        // 科学计数法
        if self.pos < self.data.len() && (self.data[self.pos] == b'e' || self.data[self.pos] == b'E') {
            self.pos += 1;
            if self.pos < self.data.len() && (self.data[self.pos] == b'-' || self.data[self.pos] == b'+') {
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
}

/// Token扫描器
pub struct TokenScanner<'a> {
    data: &'a [u8],
    pos: usize,
}

impl<'a> TokenScanner<'a> {
    pub fn new(s: &'a str) -> Self {
        TokenScanner { data: s.as_bytes(), pos: 0 }
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

    pub fn next_token(&mut self) -> Option<String> {
        self.skip_whitespace();
        if self.pos >= self.data.len() {
            return None;
        }

        let start = self.pos;
        while self.pos < self.data.len() {
            let c = self.data[self.pos];
            if c == b' ' || c == b'\t' || c == b'\n' || c == b'\r' || c == b',' {
                break;
            }
            self.pos += 1;
        }

        if self.pos == start {
            return None;
        }

        std::str::from_utf8(&self.data[start..self.pos])
            .ok()
            .map(|s| s.to_string())
    }
}

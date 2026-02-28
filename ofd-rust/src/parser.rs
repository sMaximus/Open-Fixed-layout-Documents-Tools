//! OFD parser

use std::collections::HashMap;
use std::cell::RefCell;
use std::io::Read;
use zip::ZipArchive;
use regex::Regex;

use crate::types::*;
use crate::page::*;

/// Parse result
#[derive(Debug, Default)]
pub struct ParseResult {
    pub ofd: Option<OFDDocument>,
    pub document: Option<Document>,
    pub files: Vec<String>,
    pub page_count: usize,
    pub error: Option<String>,
}

/// OFD parser
pub struct Parser {
    pub(crate) files: Vec<String>,
    files_lower: Vec<String>,
    file_index_exact: HashMap<String, usize>,
    file_index_ci: HashMap<String, usize>,
    zip_data: Vec<u8>,
    file_contents: RefCell<HashMap<String, Vec<u8>>>,
    pub ofd: Option<OFDDocument>,
    pub document: Option<Document>,
    pub fonts: HashMap<String, Font>,
    pub images: HashMap<String, Vec<u8>>,
    pub font_files: HashMap<String, Vec<u8>>,
    pub composite_units: HashMap<String, CompositeGraphicUnit>,
    pub draw_params: HashMap<String, DrawParam>,
}

impl Parser {
    /// Create parser
    pub fn new(data: Vec<u8>) -> Result<Self, String> {
        let data_len = data.len();
        let cursor = std::io::Cursor::new(data.as_slice());
        let mut archive = ZipArchive::new(cursor)
            .map_err(|e| format!("failed to open ZIP: {} (size: {})", e, data_len))?;

        let mut files = Vec::with_capacity(archive.len());
        let mut files_lower = Vec::with_capacity(archive.len());
        let mut file_index_exact = HashMap::with_capacity(archive.len());
        let mut file_index_ci = HashMap::with_capacity(archive.len());

        // Large OFD optimization: index entries first, defer decompression to read_file().
        for i in 0..archive.len() {
            let file = archive.by_index(i)
                .map_err(|e| format!("failed to read ZIP entry: {}", e))?;
            let name = file.name().to_string();
            let lower = name.to_lowercase();
            files.push(name.clone());
            files_lower.push(lower.clone());
            file_index_exact.entry(name).or_insert(i);
            file_index_ci.entry(lower).or_insert(i);
        }

        Ok(Parser {
            files,
            files_lower,
            file_index_exact,
            file_index_ci,
            zip_data: data,
            file_contents: RefCell::new(HashMap::new()),
            ofd: None,
            document: None,
            fonts: HashMap::new(),
            images: HashMap::new(),
            font_files: HashMap::new(),
            composite_units: HashMap::new(),
            draw_params: HashMap::new(),
        })
    }
    /// Parse OFD
    pub fn parse(&mut self) -> Result<ParseResult, String> {
        const MAX_FILES_IN_PARSE_RESULT: usize = 2000;
        let files_for_result = if self.files.len() <= MAX_FILES_IN_PARSE_RESULT {
            self.files.clone()
        } else {
            Vec::new()
        };
        let mut result = ParseResult {
            files: files_for_result,
            ..Default::default()
        };

        if self.files.len() > MAX_FILES_IN_PARSE_RESULT {
            web_sys::console::log_1(
                &format!(
                    "[parse] too many files ({}), skip files in parse result; use get_files() when needed",
                    self.files.len()
                )
                .into(),
            );
        }

        // Parse OFD.xml
        let ofd_data = match self.read_file("OFD.xml") {
            Ok(d) => d,
            Err(e) => {
                result.error = Some(format!("failed to read OFD.xml: {}", e));
                return Ok(result);
            }
        };
        
        let ofd_xml = Self::remove_namespace_prefix(&String::from_utf8_lossy(&ofd_data));
        
        self.ofd = match quick_xml::de::from_str(&ofd_xml) {
            Ok(o) => Some(o),
            Err(e) => {
                result.error = Some(format!("failed to parse OFD.xml: {}", e));
                return Ok(result);
            }
        };
        result.ofd = self.ofd.clone();

        // Parse Document.xml
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
                            result.error = Some(format!("failed to parse Document.xml: {}", e));
                            return Ok(result);
                        }
                    };
                    
                    if let Some(ref doc) = self.document {
                        result.page_count = doc.pages.page.len();
                        // 璋冭瘯杈撳嚭
                        web_sys::console::log_1(&format!(
                            "Document parsed: page_count={}, physical_box='{}'",
                            doc.pages.page.len(),
                            doc.common_data.page_area.physical_box
                        ).into());
                    }
                    result.document = self.document.clone();
                } else {
                    result.error = Some("failed to read Document.xml".to_string());
                }
            }
        }

        Ok(result)
    }

    /// Read file from ZIP (lazy extraction for large OFD)
    pub fn read_file(&self, name: &str) -> Result<Vec<u8>, String> {
        let name = name.trim_start_matches('/');
        if name.is_empty() {
            return Err("file name is empty".to_string());
        }

        let name_lower = name.to_lowercase();
        let file_index = self
            .resolve_file_index(name, &name_lower)
            .ok_or_else(|| format!("file not found: {}", name))?;
        let canonical_name = &self.files[file_index];

        if let Some(data) = self.file_contents.borrow().get(canonical_name).cloned() {
            return Ok(data);
        }

        let cursor = std::io::Cursor::new(self.zip_data.as_slice());
        let mut archive = ZipArchive::new(cursor)
            .map_err(|e| format!("failed to reopen ZIP: {}", e))?;

        let mut file = archive
            .by_index(file_index)
            .map_err(|e| format!("failed to read ZIP entry: {}", e))?;

        let reserve = (file.size().min(4 * 1024 * 1024) as usize).max(1024);
        let mut contents = Vec::with_capacity(reserve);
        file.read_to_end(&mut contents)
            .map_err(|e| format!("failed to read file content: {}", e))?;

        if Self::should_cache_file(canonical_name, contents.len()) {
            self.file_contents
                .borrow_mut()
                .insert(canonical_name.clone(), contents.clone());
        }

        Ok(contents)
    }

    fn resolve_file_index(&self, name: &str, name_lower: &str) -> Option<usize> {
        if let Some(idx) = self.file_index_exact.get(name) {
            return Some(*idx);
        }
        if let Some(idx) = self.file_index_ci.get(name_lower) {
            return Some(*idx);
        }

        let suffix = format!("/{}", name_lower);
        for (idx, key_lower) in self.files_lower.iter().enumerate() {
            if key_lower.ends_with(name_lower) || key_lower.ends_with(&suffix) {
                return Some(idx);
            }
        }

        let base_name = std::path::Path::new(name_lower)
            .file_name()
            .and_then(|s| s.to_str())
            .unwrap_or(name_lower);

        for (idx, key_lower) in self.files_lower.iter().enumerate() {
            let key_base = std::path::Path::new(key_lower)
                .file_name()
                .and_then(|s| s.to_str())
                .unwrap_or(key_lower.as_str());
            if key_base == base_name {
                return Some(idx);
            }
        }

        None
    }

    fn should_cache_file(name: &str, size: usize) -> bool {
        if size <= 1024 * 1024 {
            return true;
        }
        let lower = name.to_lowercase();
        lower.ends_with(".xml") || lower.ends_with(".ofd") || lower.ends_with(".txt")
    }
    /// 绉婚櫎XML鍛藉悕绌洪棿鍓嶇紑
    pub fn remove_namespace_prefix(xml_str: &str) -> String {
        let re1 = Regex::new(r"<(/?)ofd:").unwrap();
        let result = re1.replace_all(xml_str, "<$1");
        
        let re2 = Regex::new(r#"\s+xmlns:[^=]+="[^"]*""#).unwrap();
        let result = re2.replace_all(&result, "").to_string();

        // 淇濇姢 TextCode 涓殑鏂囨湰鍐呭涓嶈 quick_xml trim
        // 灏?<TextCode ...>text</TextCode> 涓殑鏂囨湰绌烘牸鏇挎崲涓?\u{00A0}(NBSP)
        // 杩欐牱 quick_xml 鐨?$text 涓嶄細 trim 鎺夊畠浠?
        Self::preserve_textcode_spaces(&result)
    }

    /// 淇濇姢 TextCode 鍏冪礌涓殑鍓嶅/灏鹃儴绌烘牸
    fn preserve_textcode_spaces(xml_str: &str) -> String {
        let re = Regex::new(r"(?s)(<TextCode[^>]*>)(.*?)(</TextCode>)").unwrap();
        re.replace_all(xml_str, |caps: &regex::Captures| {
            let open_tag = &caps[1];
            let text = &caps[2];
            let close_tag = &caps[3];

            // 濡傛灉鏂囨湰涓寘鍚瓙鍏冪礌锛堝 CGTransform锛夛紝涓嶅鐞?
            if text.contains('<') {
                return format!("{}{}{}", open_tag, text, close_tag);
            }

            // 灏嗗墠瀵煎拰灏鹃儴鐨勬櫘閫氱┖鏍兼浛鎹负 NBSP(\u{00A0})
            let chars: Vec<char> = text.chars().collect();
            let mut result_chars = chars.clone();

            // 鍓嶅绌烘牸
            for i in 0..result_chars.len() {
                if result_chars[i] == ' ' {
                    result_chars[i] = '\u{00A0}';
                } else {
                    break;
                }
            }
            // 灏鹃儴绌烘牸
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

    /// 鍚堝苟閲嶅鐨刋ML鍏冪礌锛堝澶氫釜 PublicRes銆丏ocumentRes锛?
    /// quick-xml 鍙嶅簭鍒楀寲涓嶆敮鎸侀噸澶嶅悓鍚嶅厓绱狅紝鍘婚櫎閲嶅鍙繚鐣欑涓€涓?
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

    /// 鑾峰彇鏂囦欢鍒楄〃
    pub fn get_files(&self) -> &[String] {
        &self.files
    }

    /// 鑾峰彇鏂囨。淇℃伅
    pub fn get_doc_info(&self) -> Option<&DocInfo> {
        self.ofd.as_ref()
            .and_then(|ofd| ofd.doc_body.first())
            .map(|body| &body.doc_info)
    }

    /// 鑾峰彇椤垫暟
    pub fn get_page_count(&self) -> usize {
        self.document.as_ref()
            .map(|doc| doc.pages.page.len())
            .unwrap_or(0)
    }

    /// 鑾峰彇椤甸潰鏂囦欢璺緞
    pub fn get_page_path(&self, index: usize) -> Option<String> {
        let doc = self.document.as_ref()?;
        let page_ref = doc.pages.page.get(index)?;
        let page_loc = page_ref.base_loc.trim_start_matches('/');

        // 鑾峰彇鏂囨。鏍圭洰褰?
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

        // 灏濊瘯澶氱璺緞缁勫悎
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

        // 鎸夐〉闈㈢储寮曟煡鎵?
        let page_pattern = format!("page_{}/content.xml", index);
        for file in &self.files {
            let lower = file.to_lowercase();
            if lower.contains(&page_pattern) || lower.contains(&format!("page_{}.xml", index)) {
                return Some(file.clone());
            }
        }

        Some(format!("{}/{}", doc_base, page_loc))
    }

    /// 鑾峰彇椤甸潰灏哄
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

    /// 鍔犺浇璧勬簮
    pub fn load_resources(&mut self) {
        if !self.fonts.is_empty() {
            return;
        }

        // 鑾峰彇鏂囨。鏍圭洰褰?
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

            // 鍔犺浇瀛椾綋
            if let Some(fonts) = &res.fonts {
                for font in &fonts.font {
                    let mut font_clone = font.clone();
                    if !font.font_file.is_empty() {
                        font_clone.font_file = format!("{}/{}/{}", base_path, res.base_loc, font.font_file);
                        // 棰勫姞杞藉瓧浣撴枃浠舵暟鎹紝鏍囪璇ュ瓧浣撴湁宓屽叆鏂囦欢
                        match self.read_file(&font_clone.font_file) {
                            Ok(font_data) if !font_data.is_empty() => {
                                web_sys::console::log_1(&format!(
                                    "[璧勬簮] 瀛椾綋鍔犺浇鎴愬姛: id={}, path='{}', size={}bytes",
                                    font.id, font_clone.font_file, font_data.len()
                                ).into());
                                self.font_files.insert(font.id.clone(), font_data);
                            }
                            Ok(_) => {
                                web_sys::console::warn_1(&format!(
                                    "[璧勬簮] 瀛椾綋鏂囦欢涓虹┖: id={}, path='{}'",
                                    font.id, font_clone.font_file
                                ).into());
                            }
                            Err(e) => {
                                web_sys::console::warn_1(&format!(
                                    "[璧勬簮] 瀛椾綋鏂囦欢璇诲彇澶辫触: id={}, path='{}', err={}",
                                    font.id, font_clone.font_file, e
                                ).into());
                            }
                        }
                    }
                    self.fonts.insert(font.id.clone(), font_clone);
                }
            }

            // 鍔犺浇鍥剧墖璺緞
            if let Some(medias) = &res.multi_medias {
                for media in &medias.multi_media {
                    if media.media_type == "Image" {
                        let img_path = format!("{}/{}/{}", base_path, res.base_loc, media.media_file);
                        self.images.insert(media.id.clone(), img_path.into_bytes());
                    }
                }
            }

            // 鍔犺浇澶嶅悎鍥惧厓
            if let Some(cgu) = &res.composite_graphic_units {
                for unit in &cgu.units {
                    self.composite_units.insert(unit.id.clone(), unit.clone());
                }
            }

            // 鍔犺浇缁樺埗鍙傛暟锛圖rawParam 鐩存帴鍦?Res 涓嬶級
            for dp in &res.draw_params {
                self.draw_params.insert(dp.id.clone(), dp.clone());
            }
            // 鍔犺浇缁樺埗鍙傛暟锛圖rawParams 鍖呰鍏冪礌涓嬶級
            if let Some(ref dps) = res.draw_params_wrapped {
                for dp in &dps.draw_param {
                    self.draw_params.insert(dp.id.clone(), dp.clone());
                }
            }
        }

        // 澶勭悊 DrawParam 鐨?Relative 缁ф壙
        let dp_ids: Vec<String> = self.draw_params.keys().cloned().collect();
        for id in dp_ids {
            let relative_id = self.draw_params.get(&id).map(|dp| dp.relative.clone()).unwrap_or_default();
            if !relative_id.is_empty() {
                if let Some(parent) = self.draw_params.get(&relative_id).cloned() {
                    let child = self.draw_params.get_mut(&id).unwrap();
                    if child.stroke_color.is_none() {
                        child.stroke_color = parent.stroke_color.clone();
                    }
                    if child.fill_color.is_none() {
                        child.fill_color = parent.fill_color.clone();
                    }
                    if child.line_width == 0.0 && parent.line_width > 0.0 {
                        child.line_width = parent.line_width;
                    }
                    if child.join.is_empty() {
                        child.join = parent.join.clone();
                    }
                    if child.cap.is_empty() {
                        child.cap = parent.cap.clone();
                    }
                }
            }
        }

        let _ = doc_base;

        // 璋冭瘯锛氳緭鍑哄凡鍔犺浇鐨勮祫婧愪俊鎭?
        web_sys::console::log_1(&format!(
            "[璧勬簮] load_resources 瀹屾垚: fonts={}, font_files={}, images={}, composites={}, draw_params={}",
            self.fonts.len(), self.font_files.len(), self.images.len(), self.composite_units.len(), self.draw_params.len()
        ).into());
        for (id, font) in &self.fonts {
            web_sys::console::log_1(&format!(
                "[璧勬簮] font id={}, name='{}', family='{}'",
                id, font.font_name, font.family_name
            ).into());
        }
    }

    /// 鎳掑姞杞藉浘鐗?
    pub fn load_image_lazy(&mut self, resource_id: &str) -> Option<Vec<u8>> {
        if let Some(data) = self.images.get(resource_id) {
            if data.len() > 500 {
                return Some(data.clone());
            }
            // 鏄矾寰勶紝灏濊瘯鍔犺浇
            let img_path = String::from_utf8_lossy(data).to_string();
            if let Ok(img_data) = self.read_file(&img_path) {
                self.images.insert(resource_id.to_string(), img_data.clone());
                return Some(img_data);
            }
        }

        // 灏濊瘯鐩存帴鐢?ID 浣滀负鏂囦欢鍚嶆煡鎵?
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

/// 瑙ｆ瀽杈圭晫妗?
pub fn parse_box(box_str: &str) -> (f64, f64) {
    let mut scanner = NumberScanner::new(box_str);
    let x = scanner.next_float(); // skip x
    let y = scanner.next_float(); // skip y
    let w = scanner.next_float().unwrap_or(0.0);
    let h = scanner.next_float().unwrap_or(0.0);
    
    // 璋冭瘯杈撳嚭
    web_sys::console::log_1(&format!(
        "parse_box: input='{}', x={:?}, y={:?}, w={}, h={}",
        box_str, x, y, w, h
    ).into());
    
    if w == 0.0 && h == 0.0 {
        return (210.0, 297.0);
    }
    (w, h)
}

/// 瑙ｆ瀽杈圭晫
pub fn parse_boundary(boundary: &str) -> (f64, f64, f64, f64) {
    let mut scanner = NumberScanner::new(boundary);
    let x = scanner.next_float().unwrap_or(0.0);
    let y = scanner.next_float().unwrap_or(0.0);
    let w = scanner.next_float().unwrap_or(0.0);
    let h = scanner.next_float().unwrap_or(0.0);
    (x, y, w, h)
}

/// 瑙ｆ瀽棰滆壊
pub fn parse_color(value: &str) -> String {
    let mut scanner = NumberScanner::new(value);
    if let (Some(r), Some(g), Some(b)) = (scanner.next_float(), scanner.next_float(), scanner.next_float()) {
        return format!("rgb({},{},{})", r as i32, g as i32, b as i32);
    }
    "#000".to_string()
}

/// 瑙ｆ瀽CTM鍙樻崲鐭╅樀
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

/// 瑙ｆ瀽DeltaX/DeltaY
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

/// 鏁板瓧鎵弿鍣?
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
        
        // 绗﹀彿
        if self.pos < self.data.len() && (self.data[self.pos] == b'-' || self.data[self.pos] == b'+') {
            self.pos += 1;
        }
        
        // 鏁存暟閮ㄥ垎
        while self.pos < self.data.len() && self.data[self.pos].is_ascii_digit() {
            self.pos += 1;
        }
        
        // 灏忔暟閮ㄥ垎
        if self.pos < self.data.len() && self.data[self.pos] == b'.' {
            self.pos += 1;
            while self.pos < self.data.len() && self.data[self.pos].is_ascii_digit() {
                self.pos += 1;
            }
        }
        
        // 绉戝璁℃暟娉?
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

/// Token鎵弿鍣?
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



use base64::{engine::general_purpose::STANDARD as BASE64, Engine as _};
use regex::Regex;
use std::collections::HashSet;

use crate::page::{SignatureXML, Signatures, StampAnnot};
use crate::parser::{parse_boundary, Parser};
use crate::render::StampDebugInfo;

#[derive(Debug, Clone, Copy, Default)]
pub(crate) struct StampRect {
    pub x: f64,
    pub y: f64,
    pub width: f64,
    pub height: f64,
}

impl StampRect {
    fn is_empty(self) -> bool {
        self.width <= 0.0 || self.height <= 0.0
    }
}

#[derive(Debug, Clone)]
pub(crate) struct ResolvedStamp {
    pub id: String,
    pub page_ref: String,
    pub boundary: String,
    pub clip: String,
    pub sig_dir: String,
    pub image_data_url: Option<String>,
    pub seal_source: Option<String>,
    pub mime_type: Option<String>,
    pub full_rect: StampRect,
    pub visible_rect: StampRect,
    pub source_rect: StampRect,
    pub has_clip: bool,
}

impl ResolvedStamp {
    pub(crate) fn canvas_data_url(&self) -> Option<String> {
        let data_url = self.image_data_url.as_ref()?;
        if !self.has_clip {
            return Some(data_url.clone());
        }

        let svg = format!(
            r#"<svg xmlns="http://www.w3.org/2000/svg" width="{:.4}" height="{:.4}" viewBox="0 0 {:.4} {:.4}"><image href="{}" x="{:.4}" y="{:.4}" width="{:.4}" height="{:.4}" preserveAspectRatio="none"/></svg>"#,
            self.visible_rect.width,
            self.visible_rect.height,
            self.visible_rect.width,
            self.visible_rect.height,
            data_url,
            -(self.source_rect.x * self.full_rect.width),
            -(self.source_rect.y * self.full_rect.height),
            self.full_rect.width,
            self.full_rect.height,
        );

        Some(format!(
            "data:image/svg+xml;base64,{}",
            BASE64.encode(svg.as_bytes())
        ))
    }

    pub(crate) fn debug_info(&self) -> StampDebugInfo {
        StampDebugInfo {
            annot_id: self.id.clone(),
            page_ref: self.page_ref.clone(),
            boundary: self.boundary.clone(),
            clip: self.clip.clone(),
            sig_dir: self.sig_dir.clone(),
            seal_found: self.image_data_url.is_some(),
            seal_source: self.seal_source.clone(),
            mime_type: self.mime_type.clone(),
            has_clip: self.has_clip,
            full_boundary: format_rect(self.full_rect),
            visible_boundary: format_rect(self.visible_rect),
            source_view_box: format_rect(self.source_rect),
        }
    }
}

#[derive(Debug, Clone)]
struct StampAnnotInfo {
    annot: StampAnnot,
    sig_dir: String,
}

#[derive(Debug, Clone)]
struct SignatureSource {
    path: String,
    sig_dir: String,
}

impl Parser {
    pub(crate) fn collect_page_stamps(&mut self, page_index: usize) -> Vec<ResolvedStamp> {
        let doc = match self.document.as_ref() {
            Some(d) => d,
            None => return Vec::new(),
        };
        if page_index >= doc.pages.page.len() {
            return Vec::new();
        }

        let page_id = doc.pages.page[page_index].id.clone();
        let annots = self.collect_stamp_annots();
        let mut resolved = Vec::new();

        for item in annots {
            if item.annot.page_ref != page_id {
                continue;
            }

            let geometry = match compute_stamp_geometry(&item.annot) {
                Some(geometry) => geometry,
                None => continue,
            };

            let seal_image = self.find_seal_image(&item.sig_dir);
            let image_data_url = seal_image.as_ref().map(|seal_img| {
                format!(
                    "data:{};base64,{}",
                    seal_img.mime_type,
                    BASE64.encode(&seal_img.bytes)
                )
            });
            let seal_source = seal_image.as_ref().map(|seal_img| seal_img.source.clone());
            let mime_type = seal_image
                .as_ref()
                .map(|seal_img| seal_img.mime_type.to_string());

            let resolved_stamp = ResolvedStamp {
                id: item.annot.id.clone(),
                page_ref: item.annot.page_ref.clone(),
                boundary: item.annot.boundary.clone(),
                clip: item.annot.clip.clone(),
                sig_dir: item.sig_dir.clone(),
                image_data_url,
                seal_source,
                mime_type,
                full_rect: geometry.full_rect,
                visible_rect: geometry.visible_rect,
                source_rect: geometry.source_rect,
                has_clip: geometry.has_clip,
            };

            resolved.push(resolved_stamp);
        }

        resolved
    }

    fn collect_stamp_annots(&mut self) -> Vec<StampAnnotInfo> {
        let mut all_annots = Vec::new();
        let signature_sources = self.collect_signature_sources();

        for source in signature_sources {
            let sig_data = match self.read_file(&source.path) {
                Ok(data) => data,
                Err(_) => {
                    continue;
                }
            };

            let annots = parse_signature_stamp_annots(&String::from_utf8_lossy(&sig_data));

            for annot in annots {
                all_annots.push(StampAnnotInfo {
                    annot,
                    sig_dir: source.sig_dir.clone(),
                });
            }
        }

        all_annots
    }

    fn find_seal_image(&mut self, sig_dir: &str) -> Option<SealImage> {
        let candidates = [
            format!("{}/Seal.esl", sig_dir),
            format!("{}/seal.esl", sig_dir),
            format!("{}/Seal.png", sig_dir),
            format!("{}/seal.png", sig_dir),
            format!("{}/SignedValue.dat", sig_dir),
        ];

        for candidate in &candidates {
            if !self.package_contains_exact_path(candidate) {
                continue;
            }
            if let Ok(data) = self.read_file(candidate) {
                if let Some(img) = crate::svg_render::extract_image_from_binary(&data) {
                    return Some(SealImage {
                        mime_type: detect_stamp_mime_type(&img),
                        bytes: img,
                        source: candidate.to_string(),
                    });
                }
            }
        }

        let sig_dir_lower = sig_dir.to_lowercase();
        let files_clone: Vec<String> = self.files.clone();
        for file in &files_clone {
            let lower = file.to_lowercase();
            if (lower.starts_with(&sig_dir_lower) || lower.contains(sig_dir))
                && (lower.ends_with(".png")
                    || lower.ends_with(".jpg")
                    || lower.ends_with(".jpeg")
                    || lower.ends_with(".gif")
                    || lower.ends_with(".svg"))
            {
                if let Ok(data) = self.read_file(file) {
                    return Some(SealImage {
                        mime_type: detect_stamp_mime_type(&data),
                        bytes: data,
                        source: file.clone(),
                    });
                }
            }
        }

        None
    }

    fn package_contains_exact_path(&self, path: &str) -> bool {
        self.resolve_package_exact_path(path).is_some()
    }

    fn resolve_package_exact_path(&self, path: &str) -> Option<String> {
        let normalized = normalize_package_path(path.trim_start_matches('/')).to_lowercase();
        self.files
            .iter()
            .find(|file| {
                normalize_package_path(file.trim_start_matches('/')).to_lowercase() == normalized
            })
            .cloned()
    }
}

impl Parser {
    fn collect_signature_sources(&mut self) -> Vec<SignatureSource> {
        let files_clone: Vec<String> = self.files.clone();
        let mut sources = Vec::new();
        let mut seen = HashSet::new();
        for file in &files_clone {
            if !file.to_lowercase().ends_with("signatures.xml") {
                continue;
            }

            let data = match self.read_file(file) {
                Ok(d) => d,
                Err(_) => {
                    continue;
                }
            };

            let xml_str = Self::remove_namespace_prefix(&String::from_utf8_lossy(&data));
            let sigs: Signatures = match quick_xml::de::from_str(&xml_str) {
                Ok(sigs) => sigs,
                Err(_) => {
                    continue;
                }
            };

            let base_path = std::path::Path::new(file)
                .parent()
                .and_then(|p| p.to_str())
                .unwrap_or("")
                .to_string();

            for sig in &sigs.signature {
                let sig_loc = normalize_package_path(sig.base_loc.trim_start_matches('/'));
                if sig_loc.is_empty() {
                    continue;
                }

                let mut candidates = vec![sig_loc.clone()];
                if !base_path.is_empty() {
                    candidates.push(format!("{}/{}", base_path, sig_loc));
                }

                let sig_path = match candidates
                    .iter()
                    .find_map(|candidate| self.resolve_package_exact_path(candidate))
                {
                    Some(path) => path,
                    None => {
                        continue;
                    }
                };

                push_signature_source(&mut sources, &mut seen, sig_path);
            }
        }

        for file in &files_clone {
            let lower = file.to_lowercase();
            if lower.ends_with("signature.xml") && !lower.ends_with("signatures.xml") {
                push_signature_source(&mut sources, &mut seen, file.clone());
            }
        }

        sources
    }
}

#[derive(Debug, Clone, Copy)]
struct StampGeometry {
    full_rect: StampRect,
    visible_rect: StampRect,
    source_rect: StampRect,
    has_clip: bool,
}

#[derive(Debug, Clone)]
struct SealImage {
    mime_type: &'static str,
    bytes: Vec<u8>,
    source: String,
}

fn compute_stamp_geometry(annot: &StampAnnot) -> Option<StampGeometry> {
    let (x, y, width, height) = parse_boundary(&annot.boundary);
    let full_rect = StampRect {
        x,
        y,
        width,
        height,
    };
    if full_rect.is_empty() {
        return None;
    }

    if annot.clip.trim().is_empty() {
        return Some(StampGeometry {
            full_rect,
            visible_rect: full_rect,
            source_rect: StampRect {
                x: 0.0,
                y: 0.0,
                width: 1.0,
                height: 1.0,
            },
            has_clip: false,
        });
    }

    let (clip_x, clip_y, clip_width, clip_height) = parse_boundary(&annot.clip);
    if clip_width <= 0.0 || clip_height <= 0.0 {
        return Some(StampGeometry {
            full_rect,
            visible_rect: full_rect,
            source_rect: StampRect {
                x: 0.0,
                y: 0.0,
                width: 1.0,
                height: 1.0,
            },
            has_clip: false,
        });
    }

    let start_x = clip_x.clamp(0.0, full_rect.width);
    let start_y = clip_y.clamp(0.0, full_rect.height);
    let end_x = (clip_x + clip_width).clamp(0.0, full_rect.width);
    let end_y = (clip_y + clip_height).clamp(0.0, full_rect.height);
    let visible_width = end_x - start_x;
    let visible_height = end_y - start_y;

    if visible_width <= 0.0 || visible_height <= 0.0 {
        return Some(StampGeometry {
            full_rect,
            visible_rect: full_rect,
            source_rect: StampRect {
                x: 0.0,
                y: 0.0,
                width: 1.0,
                height: 1.0,
            },
            has_clip: false,
        });
    }

    Some(StampGeometry {
        full_rect,
        visible_rect: StampRect {
            x: full_rect.x + start_x,
            y: full_rect.y + start_y,
            width: visible_width,
            height: visible_height,
        },
        source_rect: StampRect {
            x: start_x / full_rect.width,
            y: start_y / full_rect.height,
            width: visible_width / full_rect.width,
            height: visible_height / full_rect.height,
        },
        has_clip: true,
    })
}

fn parse_signature_stamp_annots(sig_xml: &str) -> Vec<StampAnnot> {
    let sig_xml = Parser::remove_namespace_prefix(sig_xml);
    match quick_xml::de::from_str::<SignatureXML>(&sig_xml) {
        Ok(parsed) => parsed.signed_info.stamp_annot,
        Err(_) => extract_stamp_annots_by_regex(&sig_xml),
    }
}

fn extract_stamp_annots_by_regex(sig_xml: &str) -> Vec<StampAnnot> {
    let tag_re = Regex::new(r#"(?is)<(?:\w+:)?StampAnnot\b([^>]*)/?>"#).unwrap();
    let attr_re = Regex::new(r#"(?i)([\w:]+)\s*=\s*"([^"]*)""#).unwrap();
    let mut annots = Vec::new();

    for tag_caps in tag_re.captures_iter(sig_xml) {
        let attrs = tag_caps.get(1).map(|m| m.as_str()).unwrap_or_default();
        let mut annot = StampAnnot::default();

        for attr_caps in attr_re.captures_iter(attrs) {
            let key = attr_caps[1]
                .split(':')
                .last()
                .unwrap_or_default()
                .to_ascii_lowercase();
            let value = attr_caps[2].to_string();
            match key.as_str() {
                "id" => annot.id = value,
                "pageref" => annot.page_ref = value,
                "boundary" => annot.boundary = value,
                "clip" => annot.clip = value,
                _ => {}
            }
        }

        if !annot.page_ref.is_empty() && !annot.boundary.is_empty() {
            annots.push(annot);
        }
    }

    annots
}

fn detect_stamp_mime_type(data: &[u8]) -> &'static str {
    if data.starts_with(&[0xFF, 0xD8, 0xFF]) {
        "image/jpeg"
    } else if data.starts_with(b"GIF87a") || data.starts_with(b"GIF89a") {
        "image/gif"
    } else if data.starts_with(b"<svg") || data.windows(4).any(|window| window == b"<svg") {
        "image/svg+xml"
    } else {
        "image/png"
    }
}

fn format_rect(rect: StampRect) -> String {
    format!(
        "{:.4} {:.4} {:.4} {:.4}",
        rect.x, rect.y, rect.width, rect.height
    )
}

fn push_signature_source(
    sources: &mut Vec<SignatureSource>,
    seen: &mut HashSet<String>,
    sig_path: String,
) {
    let key = sig_path.to_lowercase();
    if !seen.insert(key) {
        return;
    }

    let sig_dir = std::path::Path::new(&sig_path)
        .parent()
        .and_then(|p| p.to_str())
        .unwrap_or("")
        .to_string();

    sources.push(SignatureSource {
        path: sig_path,
        sig_dir,
    });
}

fn normalize_package_path(path: &str) -> String {
    path.replace('\\', "/").trim_start_matches('/').to_string()
}

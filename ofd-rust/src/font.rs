//! Embedded font normalization helpers.
//!
//! OFD producers sometimes store a stand-alone CFF program in `FontFile`.
//! Browsers and `ttf-parser::Face` expect the CFF program to be contained in an
//! OpenType (SFNT) file, so build the small set of required OpenType tables
//! around it when the resource is loaded.

use std::collections::BTreeMap;

const CHECKSUM_MAGIC: u32 = 0xB1B0_AFBA;

/// Additional transform selected by a CID CFF font's FDSelect/Font DICT.
///
/// `ttf-parser` outlines CFF charstrings in their raw coordinate space and
/// currently exposes only the Top DICT FontMatrix. Some OFD subset fonts put a
/// different FontMatrix in each Font DICT, so retain the transform relative to
/// the top-level matrix and apply it while collecting the outline.
#[derive(Debug, Clone, Copy)]
pub(crate) struct CffGlyphTransform {
    xx: f64,
    xy: f64,
    yx: f64,
    yy: f64,
    dx: f64,
    dy: f64,
}

impl CffGlyphTransform {
    fn identity() -> Self {
        Self {
            xx: 1.0,
            xy: 0.0,
            yx: 0.0,
            yy: 1.0,
            dx: 0.0,
            dy: 0.0,
        }
    }

    pub(crate) fn apply(self, x: f32, y: f32) -> (f64, f64) {
        let x = f64::from(x);
        let y = f64::from(y);
        (
            self.xx * x + self.xy * y + self.dx,
            self.yx * x + self.yy * y + self.dy,
        )
    }
}

impl Default for CffGlyphTransform {
    fn default() -> Self {
        Self::identity()
    }
}

/// Returns the per-GID Font DICT transforms for a stand-alone CFF font.
pub(crate) fn cff_glyph_transforms(data: &[u8]) -> Vec<CffGlyphTransform> {
    let Some(cff) = ttf_parser::cff::Table::parse(data) else {
        return Vec::new();
    };
    let glyph_count = usize::from(cff.number_of_glyphs());
    let identity = vec![CffGlyphTransform::identity(); glyph_count];
    let Some((font_dicts, selectors)) = parse_cid_font_dicts(data, glyph_count) else {
        return identity;
    };

    let top = cff.matrix();
    let top_matrix = [
        f64::from(top.sx),
        f64::from(top.kx),
        f64::from(top.ky),
        f64::from(top.sy),
        f64::from(top.tx),
        f64::from(top.ty),
    ];
    let transforms: Vec<CffGlyphTransform> = font_dicts
        .iter()
        .map(|dict| {
            dict_matrix(dict)
                .and_then(|matrix| relative_matrix(top_matrix, matrix))
                .unwrap_or_default()
        })
        .collect();

    selectors
        .into_iter()
        .map(|fd| transforms.get(fd).copied().unwrap_or_default())
        .collect()
}

fn parse_cid_font_dicts<'a>(
    data: &'a [u8],
    glyph_count: usize,
) -> Option<(Vec<&'a [u8]>, Vec<usize>)> {
    let header_size = usize::from(*data.get(2)?);
    let (_, top_offset) = parse_cff_index(data, header_size)?;
    let (top_dicts, _) = parse_cff_index(data, top_offset)?;
    let top_dict = *top_dicts.first()?;
    let fd_array_offset = dict_offset(top_dict, 1236)?;
    let fd_select_offset = dict_offset(top_dict, 1237)?;
    let (font_dicts, _) = parse_cff_index(data, fd_array_offset)?;
    if font_dicts.is_empty() {
        return None;
    }
    let selectors = parse_fd_select(data, fd_select_offset, glyph_count)?;
    Some((font_dicts, selectors))
}

fn parse_cff_index<'a>(data: &'a [u8], offset: usize) -> Option<(Vec<&'a [u8]>, usize)> {
    let count = read_be_offset(data, offset, 2)?;
    if count == 0 {
        return Some((Vec::new(), offset.checked_add(2)?));
    }

    let off_size = usize::from(*data.get(offset.checked_add(2)?)?);
    if !(1..=4).contains(&off_size) {
        return None;
    }
    let offsets_start = offset.checked_add(3)?;
    let offsets_len = count.checked_add(1)?.checked_mul(off_size)?;
    let objects_start = offsets_start.checked_add(offsets_len)?;
    let mut objects = Vec::with_capacity(count);
    for index in 0..count {
        let current = offsets_start.checked_add(index.checked_mul(off_size)?)?;
        let next = current.checked_add(off_size)?;
        let start = read_be_offset(data, current, off_size)?.checked_sub(1)?;
        let end = read_be_offset(data, next, off_size)?.checked_sub(1)?;
        if end < start {
            return None;
        }
        objects.push(data.get(objects_start.checked_add(start)?..objects_start.checked_add(end)?)?);
    }
    let final_offset = offsets_start.checked_add(count.checked_mul(off_size)?)?;
    let end = read_be_offset(data, final_offset, off_size)?.checked_sub(1)?;
    Some((objects, objects_start.checked_add(end)?))
}

fn read_be_offset(data: &[u8], offset: usize, size: usize) -> Option<usize> {
    let mut value = 0usize;
    for byte in data.get(offset..offset.checked_add(size)?)? {
        value = value.checked_mul(256)?.checked_add(usize::from(*byte))?;
    }
    Some(value)
}

fn dict_offset(data: &[u8], wanted_operator: u16) -> Option<usize> {
    let operands = dict_operands(data, wanted_operator)?;
    let value = *operands.last()?;
    if !value.is_finite() || value < 0.0 || value.fract() != 0.0 {
        return None;
    }
    usize::try_from(value as u64).ok()
}

fn dict_matrix(data: &[u8]) -> Option<[f64; 6]> {
    let operands = dict_operands(data, 1207)?;
    let matrix: [f64; 6] = operands.try_into().ok()?;
    // CFF stores [a b c d tx ty], where x' = ax + cy + tx and
    // y' = bx + dy + ty. Internally keep the two matrix rows together.
    Some([
        matrix[0], matrix[2], matrix[1], matrix[3], matrix[4], matrix[5],
    ])
}

fn dict_operands(data: &[u8], wanted_operator: u16) -> Option<Vec<f64>> {
    let mut operands = Vec::new();
    let mut offset = 0usize;
    while offset < data.len() {
        let byte = *data.get(offset)?;
        offset += 1;
        match byte {
            0..=21 => {
                let operator = if byte == 12 {
                    let escaped = *data.get(offset)?;
                    offset += 1;
                    1200 + u16::from(escaped)
                } else {
                    u16::from(byte)
                };
                if operator == wanted_operator {
                    return Some(operands);
                }
                operands.clear();
            }
            28 => {
                let bytes: [u8; 2] = data.get(offset..offset.checked_add(2)?)?.try_into().ok()?;
                offset += 2;
                operands.push(f64::from(i16::from_be_bytes(bytes)));
            }
            29 => {
                let bytes: [u8; 4] = data.get(offset..offset.checked_add(4)?)?.try_into().ok()?;
                offset += 4;
                operands.push(f64::from(i32::from_be_bytes(bytes)));
            }
            30 => operands.push(parse_dict_real(data, &mut offset)?),
            32..=246 => operands.push(f64::from(byte) - 139.0),
            247..=250 => {
                let next = u16::from(*data.get(offset)?);
                offset += 1;
                operands.push(f64::from((u16::from(byte) - 247) * 256 + next + 108));
            }
            251..=254 => {
                let next = u16::from(*data.get(offset)?);
                offset += 1;
                operands.push(-f64::from((u16::from(byte) - 251) * 256 + next + 108));
            }
            _ => return None,
        }
    }
    None
}

fn parse_dict_real(data: &[u8], offset: &mut usize) -> Option<f64> {
    let mut value = String::new();
    loop {
        let byte = *data.get(*offset)?;
        *offset += 1;
        for nibble in [byte >> 4, byte & 0x0F] {
            match nibble {
                0..=9 => value.push(char::from(b'0' + nibble)),
                10 => value.push('.'),
                11 => value.push('E'),
                12 => value.push_str("E-"),
                13 => return None,
                14 => value.push('-'),
                15 => return value.parse().ok(),
                _ => return None,
            }
        }
    }
}

fn parse_fd_select(data: &[u8], offset: usize, glyph_count: usize) -> Option<Vec<usize>> {
    let format = *data.get(offset)?;
    match format {
        0 => data
            .get(offset.checked_add(1)?..offset.checked_add(1 + glyph_count)?)?
            .iter()
            .map(|value| Some(usize::from(*value)))
            .collect(),
        3 => {
            let range_count = read_be_offset(data, offset.checked_add(1)?, 2)?;
            if range_count == 0 {
                return None;
            }
            let records_start = offset.checked_add(3)?;
            let sentinel_offset = records_start.checked_add(range_count.checked_mul(3)?)?;
            let sentinel = read_be_offset(data, sentinel_offset, 2)?;
            if sentinel != glyph_count || read_be_offset(data, records_start, 2)? != 0 {
                return None;
            }

            let mut selectors = vec![0usize; glyph_count];
            for index in 0..range_count {
                let record = records_start.checked_add(index.checked_mul(3)?)?;
                let start = read_be_offset(data, record, 2)?;
                let fd = usize::from(*data.get(record.checked_add(2)?)?);
                let end = if index + 1 < range_count {
                    read_be_offset(data, record.checked_add(3)?, 2)?
                } else {
                    sentinel
                };
                if end < start {
                    return None;
                }
                for selector in selectors.get_mut(start.min(glyph_count)..end.min(glyph_count))? {
                    *selector = fd;
                }
            }
            Some(selectors)
        }
        _ => None,
    }
}

fn relative_matrix(top: [f64; 6], font_dict: [f64; 6]) -> Option<CffGlyphTransform> {
    let determinant = top[0] * top[3] - top[1] * top[2];
    if !determinant.is_finite() || determinant.abs() < f64::EPSILON {
        return None;
    }
    let dx = font_dict[4] - top[4];
    let dy = font_dict[5] - top[5];
    let transform = CffGlyphTransform {
        xx: (top[3] * font_dict[0] - top[1] * font_dict[2]) / determinant,
        xy: (top[3] * font_dict[1] - top[1] * font_dict[3]) / determinant,
        yx: (-top[2] * font_dict[0] + top[0] * font_dict[2]) / determinant,
        yy: (-top[2] * font_dict[1] + top[0] * font_dict[3]) / determinant,
        dx: (top[3] * dx - top[1] * dy) / determinant,
        dy: (-top[2] * dx + top[0] * dy) / determinant,
    };
    [
        transform.xx,
        transform.xy,
        transform.yx,
        transform.yy,
        transform.dx,
        transform.dy,
    ]
    .iter()
    .all(|value| value.is_finite() && value.abs() <= 1_000_000.0)
    .then_some(transform)
}

/// Converts a stand-alone CFF 1 font program to an OpenType/CFF font.
///
/// Returns `None` when `data` is not a valid stand-alone CFF 1 program. Existing
/// OpenType fonts therefore pass through the resource loader unchanged.
pub fn convert_cff_to_otf(data: &[u8], font_name: &str) -> Option<Vec<u8>> {
    let cff = ttf_parser::cff::Table::parse(data)?;
    let glyph_count = cff.number_of_glyphs();
    let units_per_em = cff_units_per_em(&cff);
    let cmap = build_cmap(&cff);
    let advances = glyph_advances(&cff, units_per_em);
    let ascender = scale_metric(units_per_em, 0.8);
    let descender = -scale_metric(units_per_em, 0.2);

    let mut tables = vec![
        TableData::new(*b"CFF ", data.to_vec()),
        TableData::new(*b"OS/2", build_os2(&cmap, &advances, ascender, descender)),
        TableData::new(*b"cmap", build_cmap_table(&cmap)),
        TableData::new(*b"head", build_head(units_per_em, ascender, descender)),
        TableData::new(
            *b"hhea",
            build_hhea(glyph_count, &advances, ascender, descender),
        ),
        TableData::new(*b"hmtx", build_hmtx(&advances)),
        TableData::new(*b"maxp", build_maxp(glyph_count)),
        TableData::new(*b"name", build_name(font_name)),
        TableData::new(*b"post", build_post(units_per_em)),
    ];
    tables.sort_unstable_by_key(|table| table.tag);

    build_sfnt(tables)
}

fn cff_units_per_em(cff: &ttf_parser::cff::Table<'_>) -> u16 {
    let matrix = cff.matrix();
    let scale = if matrix.sy != 0.0 {
        matrix.sy.abs()
    } else {
        matrix.sx.abs()
    };
    if scale.is_finite() && scale > 0.0 {
        (1.0 / scale).round().clamp(16.0, 16_384.0) as u16
    } else {
        1000
    }
}

fn build_cmap(cff: &ttf_parser::cff::Table<'_>) -> BTreeMap<u32, u16> {
    let mut mappings = BTreeMap::new();

    // Prefer explicit Unicode glyph names when they are present in a SID font.
    for glyph_id in 1..cff.number_of_glyphs() {
        let gid = ttf_parser::GlyphId(glyph_id);
        if let Some(code_point) = cff.glyph_name(gid).and_then(glyph_name_code_point) {
            insert_mapping(&mut mappings, code_point, glyph_id);
        }
    }

    // Stand-alone non-CID CFF fonts carry an 8-bit Encoding table.
    for code_point in 0u16..=u8::MAX as u16 {
        if let Some(gid) = cff.glyph_index(code_point as u8) {
            if gid.0 != 0 {
                insert_mapping(&mut mappings, u32::from(code_point), gid.0);
            }
        }
    }

    // CID-keyed CFF fonts do not contain a Unicode cmap. Preserve CID-to-GID
    // mappings because OFD CGTransform values may refer to CIDs rather than the
    // subset's sequential glyph IDs.
    for glyph_id in 1..cff.number_of_glyphs() {
        let gid = ttf_parser::GlyphId(glyph_id);
        if let Some(cid) = cff.glyph_cid(gid) {
            insert_mapping(&mut mappings, u32::from(cid), glyph_id);
        }
    }

    // Retain an identity fallback for subset fonts that omit both Encoding and
    // usable charset metadata. Do not replace any stronger mapping above.
    for glyph_id in 1..cff.number_of_glyphs() {
        insert_mapping(&mut mappings, u32::from(glyph_id), glyph_id);
    }

    // A format 12 cmap with zero groups is rejected by browser font sanitizers.
    // Keep `.notdef`-only subset fonts loadable by mapping the replacement
    // character to glyph zero.
    if mappings.is_empty() {
        mappings.insert(0xFFFD, 0);
    }

    mappings
}

fn glyph_name_code_point(name: &str) -> Option<u32> {
    let value = if name == "space" {
        0x20
    } else if name.len() == 1 && name.as_bytes()[0].is_ascii_graphic() {
        u32::from(name.as_bytes()[0])
    } else if name.len() == 7 && name.starts_with("uni") {
        u32::from_str_radix(&name[3..], 16).ok()?
    } else if (5..=7).contains(&name.len()) && name.starts_with('u') {
        u32::from_str_radix(&name[1..], 16).ok()?
    } else {
        return None;
    };

    is_unicode_scalar(value).then_some(value)
}

fn insert_mapping(mappings: &mut BTreeMap<u32, u16>, code_point: u32, glyph_id: u16) {
    if glyph_id != 0 && is_unicode_scalar(code_point) {
        mappings.entry(code_point).or_insert(glyph_id);
    }
}

fn is_unicode_scalar(value: u32) -> bool {
    value <= 0x10_FFFF && !(0xD800..=0xDFFF).contains(&value)
}

fn glyph_advances(cff: &ttf_parser::cff::Table<'_>, units_per_em: u16) -> Vec<u16> {
    (0..cff.number_of_glyphs())
        .map(|glyph_id| {
            cff.glyph_width(ttf_parser::GlyphId(glyph_id))
                .filter(|width| *width != 0)
                .unwrap_or(units_per_em)
        })
        .collect()
}

fn build_cmap_table(mappings: &BTreeMap<u32, u16>) -> Vec<u8> {
    let groups = cmap_groups(mappings);
    let subtable_len = 16usize.saturating_add(groups.len().saturating_mul(12));
    let mut table = Vec::with_capacity(20 + subtable_len);

    push_u16(&mut table, 0); // version
    push_u16(&mut table, 2); // encoding records
    push_u16(&mut table, 0); // Unicode
    push_u16(&mut table, 4); // Unicode 2.0 and later, full repertoire
    push_u32(&mut table, 20);
    push_u16(&mut table, 3); // Windows
    push_u16(&mut table, 10); // Unicode full repertoire
    push_u32(&mut table, 20);

    push_u16(&mut table, 12); // format
    push_u16(&mut table, 0); // reserved
    push_u32(&mut table, subtable_len as u32);
    push_u32(&mut table, 0); // language
    push_u32(&mut table, groups.len() as u32);
    for group in groups {
        push_u32(&mut table, group.start_char);
        push_u32(&mut table, group.end_char);
        push_u32(&mut table, group.start_glyph_id);
    }

    table
}

#[derive(Debug, Clone, Copy)]
struct CmapGroup {
    start_char: u32,
    end_char: u32,
    start_glyph_id: u32,
}

fn cmap_groups(mappings: &BTreeMap<u32, u16>) -> Vec<CmapGroup> {
    let mut groups: Vec<CmapGroup> = Vec::new();
    for (&code_point, &glyph_id) in mappings {
        if let Some(previous) = groups.last_mut() {
            let expected_glyph = previous
                .start_glyph_id
                .saturating_add(previous.end_char - previous.start_char)
                .saturating_add(1);
            if code_point == previous.end_char.saturating_add(1)
                && u32::from(glyph_id) == expected_glyph
            {
                previous.end_char = code_point;
                continue;
            }
        }
        groups.push(CmapGroup {
            start_char: code_point,
            end_char: code_point,
            start_glyph_id: u32::from(glyph_id),
        });
    }
    groups
}

fn build_head(units_per_em: u16, ascender: i16, descender: i16) -> Vec<u8> {
    let mut table = Vec::with_capacity(54);
    push_u32(&mut table, 0x0001_0000); // version
    push_u32(&mut table, 0x0001_0000); // fontRevision
    push_u32(&mut table, 0); // checksumAdjustment (filled after assembly)
    push_u32(&mut table, 0x5F0F_3CF5); // magicNumber
    push_u16(&mut table, 0x0003); // flags
    push_u16(&mut table, units_per_em);
    table.extend_from_slice(&[0; 16]); // created + modified
    push_i16(&mut table, -i16::try_from(units_per_em).unwrap_or(i16::MAX));
    push_i16(&mut table, descender);
    push_i16(&mut table, i16::try_from(units_per_em).unwrap_or(i16::MAX));
    push_i16(&mut table, ascender);
    push_u16(&mut table, 0); // macStyle
    push_u16(&mut table, 8); // lowestRecPPEM
    push_i16(&mut table, 2); // fontDirectionHint
    push_i16(&mut table, 0); // indexToLocFormat (unused for CFF)
    push_i16(&mut table, 0); // glyphDataFormat
    table
}

fn build_hhea(glyph_count: u16, advances: &[u16], ascender: i16, descender: i16) -> Vec<u8> {
    let mut table = Vec::with_capacity(36);
    push_u32(&mut table, 0x0001_0000);
    push_i16(&mut table, ascender);
    push_i16(&mut table, descender);
    push_i16(&mut table, 0); // lineGap
    push_u16(&mut table, advances.iter().copied().max().unwrap_or(1000));
    push_i16(&mut table, 0); // minLeftSideBearing
    push_i16(&mut table, 0); // minRightSideBearing
    push_i16(
        &mut table,
        advances
            .iter()
            .copied()
            .max()
            .unwrap_or(1000)
            .min(i16::MAX as u16) as i16,
    );
    push_i16(&mut table, 1); // caretSlopeRise
    push_i16(&mut table, 0); // caretSlopeRun
    push_i16(&mut table, 0); // caretOffset
    table.extend_from_slice(&[0; 8]); // reserved
    push_i16(&mut table, 0); // metricDataFormat
    push_u16(&mut table, glyph_count); // numberOfHMetrics
    table
}

fn build_hmtx(advances: &[u16]) -> Vec<u8> {
    let mut table = Vec::with_capacity(advances.len() * 4);
    for advance in advances {
        push_u16(&mut table, *advance);
        push_i16(&mut table, 0); // left side bearing
    }
    table
}

fn build_maxp(glyph_count: u16) -> Vec<u8> {
    let mut table = Vec::with_capacity(6);
    push_u32(&mut table, 0x0000_5000); // CFF maxp version 0.5
    push_u16(&mut table, glyph_count);
    table
}

fn build_name(font_name: &str) -> Vec<u8> {
    let family_source = if font_name.trim().is_empty() {
        "OFD Embedded CFF".to_string()
    } else {
        font_name.trim().to_string()
    };
    let family: String = family_source.chars().take(255).collect();
    let postscript_name = postscript_font_name(&family);
    let values = [
        (1u16, family.as_str()),
        (2u16, "Regular"),
        (4u16, family.as_str()),
        (6u16, postscript_name.as_str()),
    ];

    let count = values.len() as u16;
    let string_offset = 6 + usize::from(count) * 12;
    let mut table = Vec::with_capacity(string_offset + family.len() * 4);
    push_u16(&mut table, 0); // format
    push_u16(&mut table, count);
    push_u16(&mut table, string_offset as u16);

    let mut strings = Vec::new();
    for (name_id, value) in values {
        let encoded = utf16_be(value);
        push_u16(&mut table, 3); // Windows
        push_u16(&mut table, 1); // Unicode BMP
        push_u16(&mut table, 0x0409); // en-US
        push_u16(&mut table, name_id);
        push_u16(&mut table, encoded.len() as u16);
        push_u16(&mut table, strings.len() as u16);
        strings.extend_from_slice(&encoded);
    }
    table.extend_from_slice(&strings);
    table
}

fn postscript_font_name(name: &str) -> String {
    let mut result: String = name
        .chars()
        .filter(|ch| ch.is_ascii_alphanumeric() || matches!(ch, '-' | '_'))
        .take(63)
        .collect();
    if result.is_empty() {
        result.push_str("OFD-Embedded-CFF");
    }
    result
}

fn utf16_be(value: &str) -> Vec<u8> {
    value
        .encode_utf16()
        .flat_map(u16::to_be_bytes)
        .collect::<Vec<_>>()
}

fn build_os2(
    cmap: &BTreeMap<u32, u16>,
    advances: &[u16],
    ascender: i16,
    descender: i16,
) -> Vec<u8> {
    let average_width = if advances.is_empty() {
        1000
    } else {
        (advances.iter().map(|value| u64::from(*value)).sum::<u64>() / advances.len() as u64)
            .min(i16::MAX as u64) as i16
    };
    let first_char = cmap.keys().next().copied().unwrap_or(0).min(0xFFFF) as u16;
    let last_char = cmap.keys().next_back().copied().unwrap_or(0).min(0xFFFF) as u16;
    let units_per_em = ascender.saturating_sub(descender).max(16);

    let mut table = Vec::with_capacity(78);
    push_u16(&mut table, 0); // version
    push_i16(&mut table, average_width);
    push_u16(&mut table, 400); // usWeightClass
    push_u16(&mut table, 5); // usWidthClass
    push_u16(&mut table, 0); // fsType
    push_i16(&mut table, scale_i16(units_per_em, 0.65));
    push_i16(&mut table, scale_i16(units_per_em, 0.60));
    push_i16(&mut table, 0);
    push_i16(&mut table, scale_i16(units_per_em, 0.075));
    push_i16(&mut table, scale_i16(units_per_em, 0.65));
    push_i16(&mut table, scale_i16(units_per_em, 0.60));
    push_i16(&mut table, 0);
    push_i16(&mut table, scale_i16(units_per_em, 0.35));
    push_i16(&mut table, scale_i16(units_per_em, 0.05));
    push_i16(&mut table, scale_i16(units_per_em, 0.25));
    push_i16(&mut table, 0); // sFamilyClass
    table.extend_from_slice(&[0; 10]); // panose
    table.extend_from_slice(&[0; 16]); // Unicode ranges
    table.extend_from_slice(b"OFD ");
    push_u16(&mut table, 0x0040); // REGULAR
    push_u16(&mut table, first_char);
    push_u16(&mut table, last_char);
    push_i16(&mut table, ascender);
    push_i16(&mut table, descender);
    push_i16(&mut table, 0); // line gap
    push_u16(&mut table, ascender.max(0) as u16);
    push_u16(&mut table, descender.unsigned_abs());
    table
}

fn build_post(units_per_em: u16) -> Vec<u8> {
    let mut table = Vec::with_capacity(32);
    push_u32(&mut table, 0x0003_0000); // format 3 for CFF fonts
    push_u32(&mut table, 0); // italicAngle
    push_i16(&mut table, -scale_metric(units_per_em, 0.1));
    push_i16(&mut table, scale_metric(units_per_em, 0.05));
    table.extend_from_slice(&[0; 20]);
    table
}

fn scale_metric(units_per_em: u16, factor: f64) -> i16 {
    (f64::from(units_per_em) * factor)
        .round()
        .clamp(0.0, f64::from(i16::MAX)) as i16
}

fn scale_i16(value: i16, factor: f64) -> i16 {
    (f64::from(value) * factor)
        .round()
        .clamp(f64::from(i16::MIN), f64::from(i16::MAX)) as i16
}

struct TableData {
    tag: [u8; 4],
    data: Vec<u8>,
}

impl TableData {
    fn new(tag: [u8; 4], data: Vec<u8>) -> Self {
        Self { tag, data }
    }
}

fn build_sfnt(tables: Vec<TableData>) -> Option<Vec<u8>> {
    let num_tables = u16::try_from(tables.len()).ok()?;
    let directory_len = tables.len().checked_mul(16)?.checked_add(12)?;
    let mut font = vec![0; directory_len];

    font[0..4].copy_from_slice(b"OTTO");
    write_u16_at(&mut font, 4, num_tables)?;
    let max_power = 1u16.checked_shl(num_tables.ilog2())?;
    let search_range = max_power.checked_mul(16)?;
    write_u16_at(&mut font, 6, search_range)?;
    write_u16_at(&mut font, 8, num_tables.ilog2() as u16)?;
    write_u16_at(
        &mut font,
        10,
        num_tables.checked_mul(16)?.checked_sub(search_range)?,
    )?;

    let mut head_offset = None;
    for (index, table) in tables.iter().enumerate() {
        while font.len() % 4 != 0 {
            font.push(0);
        }
        let offset = font.len();
        let length = table.data.len();
        let record_offset = 12 + index * 16;
        font[record_offset..record_offset + 4].copy_from_slice(&table.tag);
        write_u32_at(&mut font, record_offset + 4, table_checksum(&table.data))?;
        write_u32_at(&mut font, record_offset + 8, u32::try_from(offset).ok()?)?;
        write_u32_at(&mut font, record_offset + 12, u32::try_from(length).ok()?)?;
        font.extend_from_slice(&table.data);
        if &table.tag == b"head" {
            head_offset = Some(offset);
        }
    }
    while font.len() % 4 != 0 {
        font.push(0);
    }

    let checksum_adjustment = CHECKSUM_MAGIC.wrapping_sub(table_checksum(&font));
    write_u32_at(&mut font, head_offset?.checked_add(8)?, checksum_adjustment)?;
    Some(font)
}

fn table_checksum(data: &[u8]) -> u32 {
    data.chunks(4).fold(0u32, |sum, chunk| {
        let mut word = [0u8; 4];
        word[..chunk.len()].copy_from_slice(chunk);
        sum.wrapping_add(u32::from_be_bytes(word))
    })
}

fn push_u16(output: &mut Vec<u8>, value: u16) {
    output.extend_from_slice(&value.to_be_bytes());
}

fn push_i16(output: &mut Vec<u8>, value: i16) {
    output.extend_from_slice(&value.to_be_bytes());
}

fn push_u32(output: &mut Vec<u8>, value: u32) {
    output.extend_from_slice(&value.to_be_bytes());
}

fn write_u16_at(output: &mut [u8], offset: usize, value: u16) -> Option<()> {
    output
        .get_mut(offset..offset.checked_add(2)?)?
        .copy_from_slice(&value.to_be_bytes());
    Some(())
}

fn write_u32_at(output: &mut [u8], offset: usize, value: u32) -> Option<()> {
    output
        .get_mut(offset..offset.checked_add(4)?)?
        .copy_from_slice(&value.to_be_bytes());
    Some(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn wraps_standalone_cff_in_parseable_opentype_font() {
        let cff = minimal_cff();

        let otf = convert_cff_to_otf(&cff, "Test CFF").expect("valid CFF should convert");
        let face = ttf_parser::Face::parse(&otf, 0).expect("generated OTF should parse");

        assert_eq!(&otf[..4], b"OTTO");
        assert_eq!(face.number_of_glyphs(), 2);
        assert_eq!(face.glyph_index(' '), Some(ttf_parser::GlyphId(1)));
        assert!(face.tables().cff.is_some());
        assert_eq!(table_checksum(&otf), CHECKSUM_MAGIC);
    }

    #[test]
    fn rejects_non_cff_data() {
        assert!(convert_cff_to_otf(b"not a font", "Invalid").is_none());
    }

    #[test]
    fn applies_cid_font_dict_matrix_relative_to_top_dict() {
        let cff = minimal_cid_cff();
        let parsed = ttf_parser::cff::Table::parse(&cff).unwrap();

        assert_eq!(parsed.glyph_cid(ttf_parser::GlyphId(1)), Some(4699));
        let transforms = cff_glyph_transforms(&cff);
        let (x, y) = transforms[1].apply(10.0, 20.0);
        assert!((x - 20.0).abs() < 0.0001);
        assert!((y - 40.0).abs() < 0.0001);
    }

    fn minimal_cff() -> Vec<u8> {
        vec![
            1, 0, 4, 4, // header
            0, 1, 1, 1, 5, b'T', b'e', b's', b't', // Name INDEX
            0, 1, 1, 1, 3, // Top DICT INDEX header and offsets
            163, 17, // CharStrings offset = 24, operator = CharStrings
            0, 0, // empty String INDEX
            0, 0, // empty Global Subrs INDEX
            0, 2, 1, 1, 4, 7, // CharStrings INDEX header and offsets
            149, 22, 14, 149, 22, 14, // .notdef and space charstrings
        ]
    }

    fn minimal_cid_cff() -> Vec<u8> {
        vec![
            1, 0, 4, 4, // header
            0, 1, 1, 1, 5, b'T', b'e', b's', b't', // Name INDEX
            0, 1, 1, 1, 32, // Top DICT INDEX header and offsets
            139, 139, 139, 12, 30, // ROS 0 0 0
            29, 0, 0, 0, 53, 15, // charset offset
            29, 0, 0, 0, 56, 17, // CharStrings offset
            29, 0, 0, 0, 76, 12, 36, // FDArray offset
            29, 0, 0, 0, 68, 12, 37, // FDSelect offset
            0, 0, // empty String INDEX
            0, 0, // empty Global Subrs INDEX
            0, 0x12, 0x5B, // charset format 0: GID 1 => CID 4699
            0, 2, 1, 1, 4, 7, // CharStrings INDEX header and offsets
            149, 22, 14, 149, 22, 14, // .notdef and CID 4699 charstrings
            3, 0, 1, 0, 0, 0, 0, 2, // FDSelect format 3: GIDs 0..2 use FD 0
            0, 1, 1, 1, 15, // Font DICT INDEX header and offsets
            30, 0xA0, 0x02, 0xFF, 139, 139, // 0.002 0 0
            30, 0xA0, 0x02, 0xFF, 139, 139, 12, 7, // 0.002 0 0 FontMatrix
        ]
    }
}

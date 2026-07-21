use ofd_rust::{convert_cff_to_otf, detect_font_mime};
use std::io::{Cursor, Write};
use zip::write::FileOptions;
use zip::ZipWriter;

#[test]
fn detect_font_mime_rejects_non_font_bytes() {
    assert_eq!(detect_font_mime(b"not a font"), None);
}

#[test]
fn detect_font_mime_rejects_sfnt_without_required_os2_table() {
    let font = sfnt_with_table_tags(&[*b"cmap", *b"head"]);

    assert_eq!(detect_font_mime(&font), None);
}

#[test]
fn detect_font_mime_rejects_sfnt_with_unordered_table_directory() {
    let font = sfnt_with_table_tags(&[*b"head", *b"OS/2"]);

    assert_eq!(detect_font_mime(&font), None);
}

#[test]
fn standalone_cff_converts_to_detectable_opentype_font() {
    let otf = convert_cff_to_otf(&minimal_cff(), "Embedded CFF").unwrap();

    assert_eq!(detect_font_mime(&otf), Some("font/otf"));
    assert!(ttf_parser::Face::parse(&otf, 0).is_ok());
}

#[test]
fn text_object_parses_font_and_size_attributes() {
    let text: ofd_rust::TextObject = quick_xml::de::from_str(
        r#"<TextObject ID="19" CTM="14.75 -0.00 -0.00 14.75 0.00 0.00" Boundary="81.18 120.08 6.10 5.03" Font="20" Size="0.35">
          <CGTransform CodePosition="0" CodeCount="1" GlyphCount="1"><Glyphs>4699</Glyphs></CGTransform>
          <TextCode X="-0.02" Y="0.28">鲁</TextCode>
        </TextObject>"#,
    )
    .unwrap();

    assert_eq!(text.font, "20");
    assert!((text.size - 0.35).abs() < f64::EPSILON);
    assert_eq!(text.ctm, "14.75 -0.00 -0.00 14.75 0.00 0.00");
    assert_eq!(text.cg_transform[0].glyphs, "4699");
    assert_eq!(text.text_code[0].content, "鲁");
}

#[test]
fn parser_loads_cff_font_file_as_opentype() {
    let mut parser = ofd_rust::Parser::new(ofd_with_cff_font()).unwrap();
    parser.parse().unwrap();

    let fonts = parser.get_fonts();
    let font = fonts.iter().find(|font| font.id == "20").unwrap();

    assert!(font.has_file);
    assert!(font.data_url.is_none());
    assert_eq!(&parser.font_files["20"][..4], b"OTTO");
}

#[test]
fn svg_renderer_uses_cff_glyph_outline() {
    let mut parser = ofd_rust::Parser::new(ofd_with_cff_font()).unwrap();
    parser.parse().unwrap();

    let result = parser.render_page_svg(0);
    let svg = result.svg.expect("page should render");

    assert!(result.error.is_none());
    assert!(
        svg.contains("<path d=\"M20.0000,"),
        "expected embedded CFF glyph path in SVG: {svg}"
    );
    assert!(
        svg.contains("scale(0.195142,0.195142)"),
        "expected Size and CTM to determine the CFF glyph scale: {svg}"
    );
    assert!(!svg.contains("@font-face"));
}

fn sfnt_with_table_tags(tags: &[[u8; 4]]) -> Vec<u8> {
    let mut data = Vec::new();
    data.extend_from_slice(&[0x00, 0x01, 0x00, 0x00]);
    data.extend_from_slice(&(tags.len() as u16).to_be_bytes());
    data.extend_from_slice(&[0, 0, 0, 0, 0, 0]);

    for tag in tags {
        data.extend_from_slice(tag);
        data.extend_from_slice(&[0; 12]);
    }

    data
}

fn minimal_cff() -> Vec<u8> {
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

fn ofd_with_cff_font() -> Vec<u8> {
    const OFD_XML: &str = r#"<?xml version="1.0" encoding="UTF-8"?>
<ofd:OFD xmlns:ofd="http://www.ofdspec.org/2016">
  <ofd:DocBody><ofd:DocRoot>Doc_0/Document.xml</ofd:DocRoot></ofd:DocBody>
</ofd:OFD>"#;
    const DOCUMENT_XML: &str = r#"<?xml version="1.0" encoding="UTF-8"?>
<ofd:Document xmlns:ofd="http://www.ofdspec.org/2016">
  <ofd:CommonData>
    <ofd:PageArea><ofd:PhysicalBox>0 0 210 297</ofd:PhysicalBox></ofd:PageArea>
    <ofd:PublicRes>PublicRes.xml</ofd:PublicRes>
  </ofd:CommonData>
  <ofd:Pages>
    <ofd:Page ID="1" BaseLoc="Pages/Page_0/Content.xml"/>
  </ofd:Pages>
</ofd:Document>"#;
    const PUBLIC_RES_XML: &str = r#"<?xml version="1.0" encoding="UTF-8"?>
<ofd:Res xmlns:ofd="http://www.ofdspec.org/2016" BaseLoc="Res">
  <ofd:Fonts>
    <ofd:Font ID="20" FontName="Test CFF" FamilyName="Test CFF">
      <ofd:FontFile>Embedded.cff</ofd:FontFile>
    </ofd:Font>
  </ofd:Fonts>
</ofd:Res>"#;
    const PAGE_XML: &str = r#"<?xml version="1.0" encoding="UTF-8"?>
<ofd:Page xmlns:ofd="http://www.ofdspec.org/2016">
  <ofd:Area><ofd:PhysicalBox>0 0 210 297</ofd:PhysicalBox></ofd:Area>
  <ofd:Content>
    <ofd:Layer ID="1">
      <ofd:TextObject ID="2" CTM="14.75 -0.00 -0.00 14.75 0.00 0.00" Boundary="81.18 120.08 6.10 5.03" Font="20" Size="0.35">
        <ofd:FillColor Value="0 0 0"/>
        <ofd:TextCode X="-0.02" Y="0.28">鲁</ofd:TextCode>
        <ofd:CGTransform CodePosition="0" CodeCount="1" GlyphCount="1">
          <ofd:Glyphs>4699</ofd:Glyphs>
        </ofd:CGTransform>
      </ofd:TextObject>
    </ofd:Layer>
  </ofd:Content>
</ofd:Page>"#;

    let cursor = Cursor::new(Vec::<u8>::new());
    let mut zip = ZipWriter::new(cursor);
    let options = FileOptions::default();
    for (path, contents) in [
        ("OFD.xml", OFD_XML.as_bytes()),
        ("Doc_0/Document.xml", DOCUMENT_XML.as_bytes()),
        ("Doc_0/PublicRes.xml", PUBLIC_RES_XML.as_bytes()),
        ("Doc_0/Pages/Page_0/Content.xml", PAGE_XML.as_bytes()),
        ("Doc_0/Res/Embedded.cff", minimal_cff().as_slice()),
    ] {
        zip.start_file(path, options).unwrap();
        zip.write_all(contents).unwrap();
    }
    zip.finish().unwrap().into_inner()
}

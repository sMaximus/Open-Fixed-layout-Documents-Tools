use ofd_rust::Parser;
use serde_json::Value;
use std::io::{Cursor, Write};
use zip::write::FileOptions;
use zip::ZipWriter;

const TINY_PNG: &[u8] = &[
    0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48,
    0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00,
    0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x44, 0x41, 0x54, 0x78,
    0x9C, 0x63, 0xF8, 0xCF, 0xC0, 0xF0, 0x1F, 0x00, 0x05, 0x00, 0x01, 0xFF, 0x89, 0x99,
    0x3D, 0x1D, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
];

fn build_test_ofd(signature_xml: &str) -> Vec<u8> {
    build_test_ofd_with_index(
        r#"<?xml version="1.0" encoding="UTF-8"?>
<Signatures>
  <Signature ID="sig1" BaseLoc="Sign_0/Signature.xml"/>
</Signatures>
"#,
        signature_xml,
    )
}

fn build_test_ofd_with_files(
    signatures_xml: &str,
    signature_xml_files: &[(&str, &str)],
    extra_files: &[(&str, &[u8])],
) -> Vec<u8> {
    const PAGE_XML: &str = r#"<?xml version="1.0" encoding="UTF-8"?>
<Page>
  <Area>
    <PhysicalBox>0 0 210 297</PhysicalBox>
  </Area>
</Page>
"#;

    build_test_ofd_with_files_and_page(
        signatures_xml,
        signature_xml_files,
        extra_files,
        PAGE_XML,
    )
}

fn build_test_ofd_with_files_and_page(
    signatures_xml: &str,
    signature_xml_files: &[(&str, &str)],
    extra_files: &[(&str, &[u8])],
    page_xml: &str,
) -> Vec<u8> {
    const OFD_XML: &str = r#"<?xml version="1.0" encoding="UTF-8"?>
<OFD>
  <DocBody>
    <DocRoot>Doc_0/Document.xml</DocRoot>
  </DocBody>
</OFD>
"#;

    const DOCUMENT_XML: &str = r#"<?xml version="1.0" encoding="UTF-8"?>
<Document>
  <CommonData>
    <PageArea>
      <PhysicalBox>0 0 210 297</PhysicalBox>
    </PageArea>
  </CommonData>
  <Pages>
    <Page ID="1" BaseLoc="Pages/Page_0/Content.xml"/>
  </Pages>
</Document>
"#;

    let cursor = Cursor::new(Vec::<u8>::new());
    let mut zip = ZipWriter::new(cursor);
    let options = FileOptions::default();

    for (path, contents) in [
        ("OFD.xml", OFD_XML.as_bytes()),
        ("Doc_0/Document.xml", DOCUMENT_XML.as_bytes()),
        ("Doc_0/Pages/Page_0/Content.xml", page_xml.as_bytes()),
        ("Doc_0/Signs/Signatures.xml", signatures_xml.as_bytes()),
    ] {
        zip.start_file(path, options).unwrap();
        zip.write_all(contents).unwrap();
    }

    for &(path, contents) in signature_xml_files {
        zip.start_file(path, options).unwrap();
        zip.write_all(contents.as_bytes()).unwrap();
    }

    for &(path, contents) in extra_files {
        zip.start_file(path, options).unwrap();
        zip.write_all(contents).unwrap();
    }

    zip.finish().unwrap().into_inner()
}

fn build_test_ofd_with_index(signatures_xml: &str, signature_xml: &str) -> Vec<u8> {
    let extra_files: Vec<(&str, &[u8])> = vec![("Doc_0/Signs/Sign_0/Seal.png", TINY_PNG)];
    build_test_ofd_with_files(
        signatures_xml,
        &[("Doc_0/Signs/Sign_0/Signature.xml", signature_xml)],
        &extra_files,
    )
}

#[test]
fn render_page_svg_draws_signature_stamp_after_page_text_for_blending() {
    let signature_xml = r#"<?xml version="1.0" encoding="UTF-8"?>
<Signature>
  <SignedInfo>
    <StampAnnot ID="s001" PageRef="1" Boundary="10 20 40 40"/>
  </SignedInfo>
</Signature>
"#;
    let signatures_xml = r#"<?xml version="1.0" encoding="UTF-8"?>
<Signatures>
  <Signature ID="sig1" BaseLoc="Sign_0/Signature.xml"/>
</Signatures>
"#;
    let page_xml = r#"<?xml version="1.0" encoding="UTF-8"?>
<Page>
  <Area>
    <PhysicalBox>0 0 210 297</PhysicalBox>
  </Area>
  <Content>
    <Layer ID="L1">
      <TextObject ID="t001" Boundary="10 20 40 10" Font="F1" Size="5">
        <FillColor Value="0 0 0"/>
        <TextCode X="0" Y="5">000001</TextCode>
      </TextObject>
    </Layer>
  </Content>
</Page>
"#;

    let mut parser = Parser::new(build_test_ofd_with_files_and_page(
        signatures_xml,
        &[("Doc_0/Signs/Sign_0/Signature.xml", signature_xml)],
        &[("Doc_0/Signs/Sign_0/Seal.png", TINY_PNG)],
        page_xml,
    ))
    .unwrap();
    parser.parse().unwrap();

    let result = parser.render_page_svg(0);
    let svg = result.svg.expect("expected svg output");

    let text_index = svg
        .find("<text ")
        .unwrap_or_else(|| panic!("expected page text in svg, got: {svg}"));
    let stamp_index = svg
        .find("data:image/png;base64")
        .unwrap_or_else(|| panic!("expected signature stamp in svg, got: {svg}"));

    assert!(
        stamp_index > text_index,
        "expected signature stamp to render after page text so overlap can blend, got: {svg}"
    );
    assert!(
        svg.contains("mix-blend-mode: multiply"),
        "expected signature stamp to use multiply blend mode, got: {svg}"
    );
    assert!(
        svg.contains("pointer-events: none"),
        "expected signature stamp to ignore pointer events, got: {svg}"
    );
}

#[test]
fn render_page_svg_clips_seam_stamp_slice() {
    let signature_xml = r#"<?xml version="1.0" encoding="UTF-8"?>
<Signature>
  <SignedInfo>
    <StampAnnot ID="s001" PageRef="1" Boundary="10 20 40 40" Clip="0 0 8 40"/>
  </SignedInfo>
</Signature>
"#;

    let mut parser = Parser::new(build_test_ofd(signature_xml)).unwrap();
    parser.parse().unwrap();

    let result = parser.render_page_svg(0);
    let svg = result.svg.expect("expected svg output");

    assert!(
        svg.contains("clipPath"),
        "expected seam stamp SVG output to define a clipPath, got: {}",
        svg
    );
    assert!(
        svg.contains(r#"clip-path="url(#"#),
        "expected seam stamp image to reference a clip path, got: {}",
        svg
    );
    assert!(
        svg.contains(r#"width="302.40""#),
        "expected seam clip width of 8mm at hi-dpi scale, got: {}",
        svg
    );
}

#[test]
fn render_page_includes_clipped_stamp_image_for_canvas_output() {
    let signature_xml = r#"<?xml version="1.0" encoding="UTF-8"?>
<Signature>
  <SignedInfo>
    <StampAnnot ID="s001" PageRef="1" Boundary="10 20 40 40" Clip="0 0 8 40"/>
  </SignedInfo>
</Signature>
"#;

    let mut parser = Parser::new(build_test_ofd(signature_xml)).unwrap();
    parser.parse().unwrap();

    let result = parser.render_page(0);

    assert_eq!(
        result.canvas_data.images.len(),
        1,
        "expected seam stamp to be emitted as a canvas image"
    );

    let stamp = &result.canvas_data.images[0];
    assert!(
        (stamp.x - 37.8).abs() < 0.01,
        "expected seam stamp x to match the visible slice, got {}",
        stamp.x
    );
    assert!(
        (stamp.width - 30.24).abs() < 0.01,
        "expected seam stamp width to match the 8mm visible slice, got {}",
        stamp.width
    );
}

#[test]
fn render_outputs_include_stamp_debug_information() {
    let signature_xml = r#"<?xml version="1.0" encoding="UTF-8"?>
<Signature>
  <SignedInfo>
    <StampAnnot ID="s001" PageRef="1" Boundary="10 20 40 40" Clip="0 0 8 40"/>
  </SignedInfo>
</Signature>
"#;

    let mut parser = Parser::new(build_test_ofd(signature_xml)).unwrap();
    parser.parse().unwrap();

    let svg_json = serde_json::to_value(parser.render_page_svg(0)).unwrap();
    let canvas_json = serde_json::to_value(parser.render_page(0)).unwrap();

    assert_stamp_debug(&svg_json, "render_page_svg");
    assert_stamp_debug(&canvas_json, "render_page");
}

fn assert_stamp_debug(json: &Value, label: &str) {
    let entries = json["stampDebug"]
        .as_array()
        .unwrap_or_else(|| panic!("expected {label} to include stampDebug entries, got: {json}"));
    assert_eq!(
        entries.len(),
        1,
        "expected exactly one stampDebug entry in {label}, got: {json}"
    );

    let entry = &entries[0];
    assert_eq!(
        entry["annotId"], "s001",
        "unexpected annotId in {label}: {entry}"
    );
    assert_eq!(
        entry["pageRef"], "1",
        "unexpected pageRef in {label}: {entry}"
    );
    assert_eq!(
        entry["clip"], "0 0 8 40",
        "unexpected clip in {label}: {entry}"
    );
    assert_eq!(
        entry["sealFound"], true,
        "expected sealFound=true in {label}: {entry}"
    );
    assert_eq!(
        entry["hasClip"], true,
        "expected hasClip=true in {label}: {entry}"
    );
    assert_eq!(
        entry["visibleBoundary"], "10.0000 20.0000 8.0000 40.0000",
        "unexpected visibleBoundary in {label}: {entry}"
    );
}

#[test]
fn render_page_svg_falls_back_to_scanning_signature_xml_when_index_has_no_entries() {
    let signature_xml = r#"<?xml version="1.0" encoding="UTF-8"?>
<Signature>
  <SignedInfo>
    <StampAnnot ID="s001" PageRef="1" Boundary="10 20 40 40" Clip="0 0 8 40"/>
  </SignedInfo>
</Signature>
"#;
    let signatures_xml = r#"<?xml version="1.0" encoding="UTF-8"?>
<Signatures>
</Signatures>
"#;

    let mut parser = Parser::new(build_test_ofd_with_index(signatures_xml, signature_xml)).unwrap();
    parser.parse().unwrap();

    let result = parser.render_page_svg(0);
    let svg = result.svg.expect("expected svg output");

    assert!(
        svg.contains("data:image/png;base64"),
        "expected fallback signature scan to render stamp image, got: {svg}"
    );
}

#[test]
fn render_page_svg_does_not_reuse_another_signature_seal_for_placeholder_locator() {
    let signatures_xml = r#"<?xml version="1.0" encoding="UTF-8"?>
<Signatures>
  <Signature ID="0" Type="Seal" BaseLoc="Sign_0/Signature.xml"/>
  <Signature ID="6" Type="Seal" BaseLoc="Sign_1/Signature.xml"/>
</Signatures>
"#;
    let real_signature_xml = r#"<?xml version="1.0" encoding="UTF-8"?>
<Signature>
  <SignedInfo>
    <StampAnnot ID="s001" PageRef="1" Boundary="10 20 40 40"/>
    <Seal><BaseLoc>Seal.esl</BaseLoc></Seal>
  </SignedInfo>
  <SignedValue>SignedValue.dat</SignedValue>
</Signature>
"#;
    let locator_signature_xml = r#"<?xml version="1.0" encoding="UTF-8"?>
<Signature>
  <SignedInfo>
    <StampAnnot ID="7" PageRef="1" Boundary="68.7917 27.7813 38 38"/>
    <Seal><BaseLoc>Seal.esl</BaseLoc></Seal>
  </SignedInfo>
  <SignedValue>SignedValue.dat</SignedValue>
</Signature>
"#;

    let mut parser = Parser::new(build_test_ofd_with_files(
        signatures_xml,
        &[
            ("Doc_0/Signs/Sign_0/Signature.xml", real_signature_xml),
            ("Doc_0/Signs/Sign_1/Signature.xml", locator_signature_xml),
        ],
        &[("Doc_0/Signs/Sign_0/Seal.png", TINY_PNG)],
    ))
    .unwrap();
    parser.parse().unwrap();

    let result = parser.render_page_svg(0);
    let entries = result.stamp_debug.expect("expected stamp debug entries");

    assert_eq!(entries.len(), 2);
    assert_eq!(entries[0].annot_id, "s001");
    assert!(entries[0].seal_found);
    assert_eq!(entries[1].annot_id, "7");
    assert!(!entries[1].seal_found, "locator without local seal data must not reuse Sign_0 seal");
}

#[test]
fn render_page_svg_draws_blue_cross_placeholder_when_stamp_image_is_missing() {
    let signatures_xml = r#"<?xml version="1.0" encoding="UTF-8"?>
<Signatures>
  <Signature ID="sig1" Type="Seal" BaseLoc="Sign_0/Signature.xml"/>
</Signatures>
"#;
    let signature_xml = r#"<?xml version="1.0" encoding="UTF-8"?>
<Signature>
  <SignedInfo>
    <StampAnnot ID="s001" PageRef="1" Boundary="68.7917 27.7813 38 38"/>
    <Seal><BaseLoc>Seal.esl</BaseLoc></Seal>
  </SignedInfo>
  <SignedValue>SignedValue.dat</SignedValue>
</Signature>
"#;

    let mut parser = Parser::new(build_test_ofd_with_files(
        signatures_xml,
        &[("Doc_0/Signs/Sign_0/Signature.xml", signature_xml)],
        &[],
    ))
    .unwrap();
    parser.parse().unwrap();

    let result = parser.render_page_svg(0);
    let entries = result
        .stamp_debug
        .as_ref()
        .expect("expected stamp debug entries");
    assert_eq!(entries.len(), 1);
    assert_eq!(entries[0].annot_id, "s001");
    assert!(
        !entries[0].seal_found,
        "missing stamp image should be recorded in debug info"
    );

    let svg = result.svg.expect("expected svg output");
    assert!(
        svg.contains(r#"class="ofd-signature-stamp-placeholder""#),
        "expected missing stamp image to render a placeholder, got: {svg}"
    );
    assert!(
        svg.contains(r##"stroke="#0000ff""##),
        "expected placeholder to use the blue fallback stroke, got: {svg}"
    );
    assert!(
        svg.contains("<rect "),
        "expected placeholder to include a boundary rectangle, got: {svg}"
    );
    assert_eq!(
        svg.matches("<line ").count(),
        2,
        "expected placeholder to include two diagonal lines, got: {svg}"
    );
}

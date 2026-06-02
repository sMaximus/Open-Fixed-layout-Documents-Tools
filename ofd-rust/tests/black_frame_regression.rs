use ofd_rust::Parser;
use std::io::{Cursor, Write};
use zip::write::FileOptions;
use zip::ZipWriter;

fn build_test_ofd(page_xml: &str) -> Vec<u8> {
    build_test_ofd_with_extra_files(page_xml, &[])
}

fn build_test_ofd_with_extra_files(page_xml: &str, extra_files: &[(&str, &str)]) -> Vec<u8> {
    const OFD_XML: &str = r#"<?xml version="1.0" encoding="UTF-8"?>
<ofd:OFD xmlns:ofd="http://www.ofdspec.org/2016">
  <ofd:DocBody>
    <ofd:DocRoot>Doc_0/Document.xml</ofd:DocRoot>
  </ofd:DocBody>
</ofd:OFD>
"#;

    const DOCUMENT_XML: &str = r#"<?xml version="1.0" encoding="UTF-8"?>
<ofd:Document xmlns:ofd="http://www.ofdspec.org/2016">
  <ofd:CommonData>
    <ofd:PageArea>
      <ofd:PhysicalBox>0.00 0.00 181.00 256.00</ofd:PhysicalBox>
    </ofd:PageArea>
  </ofd:CommonData>
  <ofd:Pages>
    <ofd:Page ID="1" BaseLoc="Pages/Page_0/Content.xml"/>
  </ofd:Pages>
</ofd:Document>
"#;

    let cursor = Cursor::new(Vec::<u8>::new());
    let mut zip = ZipWriter::new(cursor);
    let options = FileOptions::default();

    for (path, contents) in [
        ("OFD.xml", OFD_XML.as_bytes()),
        ("Doc_0/Document.xml", DOCUMENT_XML.as_bytes()),
        ("Doc_0/Pages/Page_0/Content.xml", page_xml.as_bytes()),
    ] {
        zip.start_file(path, options).unwrap();
        zip.write_all(contents).unwrap();
    }

    for &(path, contents) in extra_files {
        zip.start_file(path, options).unwrap();
        zip.write_all(contents.as_bytes()).unwrap();
    }

    zip.finish().unwrap().into_inner()
}

#[test]
fn render_page_svg_does_not_add_black_frame_for_title_page() {
    let page_xml = r#"<?xml version="1.0" encoding="UTF-8"?>
<ofd:Page xmlns:ofd="http://www.ofdspec.org">
  <ofd:Area>
    <ofd:PhysicalBox>0.00 0.00 181.00 256.00</ofd:PhysicalBox>
  </ofd:Area>
  <ofd:Content>
    <ofd:Layer ID="2">
      <ofd:TextObject ID="3" Boundary="25.12 47.84 134.58 16.69" Font="4" Size="16.67" HScale="0.70" Weight="700">
        <ofd:FillColor Value="255 0 0" ColorSpace="5"/>
        <ofd:TextCode X="1.87" Y="14.34" DeltaX="12.70 12.70 12.70 12.70 12.70 12.96 12.70 12.70 12.70">北京书生电子公司文件</ofd:TextCode>
      </ofd:TextObject>
      <ofd:PathObject ID="23" Boundary="31.68 93.05 156.14 0.81" Stroke="false" Fill="true">
        <ofd:FillColor Value="255 0 0" ColorSpace="5"/>
        <ofd:AbbreviatedData>M 0.00 0.00 L 156.14 0.02 L 156.14 0.02 L 156.14 0.81 L 156.14 0.81 L 0.00 0.79 L 0.00 0.79 L 0.00 0.00 C </ofd:AbbreviatedData>
      </ofd:PathObject>
    </ofd:Layer>
  </ofd:Content>
</ofd:Page>
"#;

    let mut parser = Parser::new(build_test_ofd(page_xml)).unwrap();
    parser.parse().unwrap();

    let result = parser.render_page_svg(0);

    assert!(
        result.error.is_none(),
        "expected title page to render without error, got {:?}",
        result.error
    );
    let svg = result.svg.expect("expected svg output");
    assert!(
        !svg.contains(r#"x="0.0000" y="0.0000" width="263"#),
        "expected no synthetic black frame at page origin, got: {svg}"
    );
    assert!(
        !svg.contains(r##"stroke="#000""##),
        "expected Stroke=false red rule not to gain a black stroke, got: {svg}"
    );
}

#[test]
fn render_page_svg_does_not_render_link_annotation_hit_area_as_black_frame() {
    let page_xml = r#"<?xml version="1.0" encoding="UTF-8"?>
<ofd:Page xmlns:ofd="http://www.ofdspec.org">
  <ofd:Area>
    <ofd:PhysicalBox>0.00 0.00 181.00 256.00</ofd:PhysicalBox>
  </ofd:Area>
</ofd:Page>
"#;
    let annotations_xml = r#"<?xml version="1.0" encoding="UTF-8"?>
<ofd:Annotations xmlns:ofd="http://www.ofdspec.org">
  <ofd:Page PageID="1">
    <ofd:FileLoc>Annots/Page_0.xml</ofd:FileLoc>
  </ofd:Page>
</ofd:Annotations>
"#;
    let page_annot_xml = r#"<?xml version="1.0" encoding="UTF-8"?>
<ofd:PageAnnot xmlns:ofd="http://www.ofdspec.org">
  <ofd:Annot ID="link-1" Type="Link" Subtype="Link">
    <ofd:Appearance Boundary="0 0 70 10">
      <ofd:PathObject ID="border" Boundary="0 0 70 10">
        <ofd:AbbreviatedData>M 0 0 L 70 0 L 70 10 L 0 10 C</ofd:AbbreviatedData>
      </ofd:PathObject>
    </ofd:Appearance>
  </ofd:Annot>
</ofd:PageAnnot>
"#;

    let mut parser = Parser::new(build_test_ofd_with_extra_files(
        page_xml,
        &[
            ("Doc_0/Annotations.xml", annotations_xml),
            ("Doc_0/Annots/Page_0.xml", page_annot_xml),
        ],
    ))
    .unwrap();
    parser.parse().unwrap();

    let result = parser.render_page_svg(0);

    assert!(
        result.error.is_none(),
        "expected page with link annotation to render without error, got {:?}",
        result.error
    );
    let svg = result.svg.expect("expected svg output");
    assert!(
        !svg.contains(r##"stroke="#000""##),
        "expected link annotation hit area not to render as a black frame, got: {svg}"
    );
}

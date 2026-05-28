use ofd_rust::Parser;
use std::io::{Cursor, Write};
use zip::write::FileOptions;
use zip::ZipWriter;

fn build_test_ofd(page_xml: &str) -> Vec<u8> {
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
      <ofd:PhysicalBox>0 0 210 297</ofd:PhysicalBox>
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

    zip.finish().unwrap().into_inner()
}

#[test]
fn render_page_svg_accepts_page_block_inside_layer() {
    let page_xml = r#"<?xml version="1.0" encoding="UTF-8"?>
<ofd:Page xmlns:ofd="http://www.ofdspec.org/2016">
  <ofd:Area>
    <ofd:PhysicalBox>0 0 210 297</ofd:PhysicalBox>
  </ofd:Area>
  <ofd:Content>
    <ofd:Layer ID="1">
      <ofd:PageBlock ID="1974">
        <ofd:PathObject ID="1975" Boundary="0 0 210 297" Fill="true">
          <ofd:AbbreviatedData>M 0 0 L 210 0 L 210 297 L 0 297 C</ofd:AbbreviatedData>
        </ofd:PathObject>
      </ofd:PageBlock>
    </ofd:Layer>
  </ofd:Content>
</ofd:Page>
"#;

    let mut parser = Parser::new(build_test_ofd(page_xml)).unwrap();
    parser.parse().unwrap();

    let result = parser.render_page_svg(0);

    assert!(
        result.error.is_none(),
        "expected PageBlock page content to render without parse error, got {:?}",
        result.error
    );
    let svg = result.svg.expect("expected svg output");
    assert!(
        svg.contains("<path "),
        "expected PathObject inside PageBlock to be rendered, got: {svg}"
    );
}

#[test]
fn render_page_svg_walks_nested_page_blocks_depth_first() {
    let page_xml = r#"<?xml version="1.0" encoding="UTF-8"?>
<ofd:Page xmlns:ofd="http://www.ofdspec.org/2016">
  <ofd:Area>
    <ofd:PhysicalBox>0 0 210 297</ofd:PhysicalBox>
  </ofd:Area>
  <ofd:Content>
    <ofd:Layer ID="1">
      <ofd:PageBlock ID="outer">
        <ofd:PageBlock ID="inner">
          <ofd:PathObject ID="deep-path" Boundary="24.042 27.976 161.928 0.1" Stroke="true">
            <ofd:AbbreviatedData>M 24.042 27.976 L 185.970 27.976</ofd:AbbreviatedData>
          </ofd:PathObject>
        </ofd:PageBlock>
      </ofd:PageBlock>
    </ofd:Layer>
  </ofd:Content>
</ofd:Page>
"#;

    let mut parser = Parser::new(build_test_ofd(page_xml)).unwrap();
    parser.parse().unwrap();

    let result = parser.render_page_svg(0);

    assert!(
        result.error.is_none(),
        "expected nested PageBlock content to render without parse error, got {:?}",
        result.error
    );
    let svg = result.svg.expect("expected svg output");
    assert!(
        svg.contains("<path "),
        "expected deepest PathObject inside nested PageBlock to be rendered, got: {svg}"
    );
}

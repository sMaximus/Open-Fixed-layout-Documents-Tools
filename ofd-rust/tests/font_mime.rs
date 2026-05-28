use ofd_rust::detect_font_mime;

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

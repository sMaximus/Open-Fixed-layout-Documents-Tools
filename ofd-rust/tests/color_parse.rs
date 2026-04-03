use ofd_rust::parse_color;

#[test]
fn parse_color_supports_fragmented_hex_triplets() {
    assert_eq!(parse_color("#ee #20 #25"), "rgb(238,32,37)");
}

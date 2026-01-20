package ofd

import "encoding/xml"

// Page 页面结构
type Page struct {
	XMLName   xml.Name   `xml:"Page"`
	Area      PageArea   `xml:"Area"`
	Content   Content    `xml:"Content"`
	ContentNS Content    `xml:"http://www.ofdspec.org/2016 Content"`
	Template  []Template `xml:"Template"`
	// 直接包含 Layer（某些 OFD 文档可能没有 Content 包装）
	Layer []Layer `xml:"Layer"`
}

// Template 模板引用
type Template struct {
	TemplateID string `xml:"TemplateID,attr"`
	ZOrder     string `xml:"ZOrder,attr"`
}

// Content 页面内容
type Content struct {
	Layer []Layer `xml:"Layer"`
}

// Layer 图层
type Layer struct {
	ID           string        `xml:"ID,attr"`
	Type         string        `xml:"Type,attr"`
	DrawParam    string        `xml:"DrawParam,attr"`
	TextObjects  []TextObject  `xml:"TextObject"`
	PathObjects  []PathObject  `xml:"PathObject"`
	ImageObjects []ImageObject `xml:"ImageObject"`
}

// TextObject 文本对象
type TextObject struct {
	ID            string        `xml:"ID,attr"`
	Boundary      string        `xml:"Boundary,attr"`
	Font          string        `xml:"Font,attr"`
	Size          float64       `xml:"Size,attr"`
	HScale        float64       `xml:"HScale,attr"`
	CharSpace     float64       `xml:"CharSpace,attr"`
	ReadDirection int           `xml:"ReadDirection,attr"`
	Weight        int           `xml:"Weight,attr"`
	Italic        bool          `xml:"Italic,attr"`
	Stroke        bool          `xml:"Stroke,attr"`
	Fill          bool          `xml:"Fill,attr"`
	LineWidth     float64       `xml:"LineWidth,attr"`
	CTM           string        `xml:"CTM,attr"`
	FillColor     *Color        `xml:"FillColor"`
	StrokeColor   *Color        `xml:"StrokeColor"`
	TextCode      []TextCode    `xml:"TextCode"`
	CGTransform   []CGTransform `xml:"CGTransform"`
}

// TextCode 文本内容
type TextCode struct {
	X       float64 `xml:"X,attr"`
	Y       float64 `xml:"Y,attr"`
	DeltaX  string  `xml:"DeltaX,attr"`
	DeltaY  string  `xml:"DeltaY,attr"`
	Content string  `xml:",chardata"`
}

// CGTransform 变换矩阵
type CGTransform struct {
	CodePosition int    `xml:"CodePosition,attr"`
	CodeCount    int    `xml:"CodeCount,attr"`
	Glyphs       string `xml:"Glyphs,attr"`
}

// PathObject 路径对象
type PathObject struct {
	ID              string       `xml:"ID,attr"`
	Boundary        string       `xml:"Boundary,attr"`
	CTM             string       `xml:"CTM,attr"`
	LineWidth       float64      `xml:"LineWidth,attr"`
	Stroke          bool         `xml:"Stroke,attr"`
	Fill            bool         `xml:"Fill,attr"`
	FillColor       *ColorOrShd  `xml:"FillColor"`
	StrokeColor     *Color       `xml:"StrokeColor"`
	AbbreviatedData string       `xml:"AbbreviatedData"`
}

// ImageObject 图像对象
type ImageObject struct {
	ID         string `xml:"ID,attr"`
	Boundary   string `xml:"Boundary,attr"`
	ResourceID string `xml:"ResourceID,attr"`
	CTM        string `xml:"CTM,attr"`
}

// Color 颜色
type Color struct {
	Value      string `xml:"Value,attr"`
	ColorSpace string `xml:"ColorSpace,attr"`
}

// ColorOrShd 颜色或渐变
type ColorOrShd struct {
	Value      string   `xml:"Value,attr"`
	ColorSpace string   `xml:"ColorSpace,attr"`
	AxialShd   *AxialShd `xml:"AxialShd"`
	RadialShd  *RadialShd `xml:"RadialShd"`
}

// AxialShd 轴向渐变（线性渐变）
type AxialShd struct {
	StartPoint string    `xml:"StartPoint,attr"`
	EndPoint   string    `xml:"EndPoint,attr"`
	Extend     string    `xml:"Extend,attr"`
	Segment    []Segment `xml:"Segment"`
}

// RadialShd 径向渐变
type RadialShd struct {
	StartPoint string    `xml:"StartPoint,attr"`
	EndPoint   string    `xml:"EndPoint,attr"`
	StartRadius float64  `xml:"StartRadius,attr"`
	EndRadius   float64  `xml:"EndRadius,attr"`
	Extend      string   `xml:"Extend,attr"`
	Segment     []Segment `xml:"Segment"`
}

// Segment 渐变段
type Segment struct {
	Position float64 `xml:"Position,attr"`
	Color    Color   `xml:"Color"`
}

// Res 资源文件
type Res struct {
	XMLName     xml.Name     `xml:"Res"`
	BaseLoc     string       `xml:"BaseLoc,attr"`
	Fonts       []Font       `xml:"Fonts>Font"`
	MultiMedias []MultiMedia `xml:"MultiMedias>MultiMedia"`
	ColorSpaces []ColorSpace `xml:"ColorSpaces>ColorSpace"`
}

// Font 字体
type Font struct {
	ID         string `xml:"ID,attr"`
	FontName   string `xml:"FontName,attr"`
	FamilyName string `xml:"FamilyName,attr"`
	FontFile   string `xml:"FontFile"` // 子元素，不是属性
}

// MultiMedia 多媒体资源
type MultiMedia struct {
	ID        string `xml:"ID,attr"`
	Type      string `xml:"Type,attr"`
	MediaFile string `xml:"MediaFile"`
}

// ColorSpace 颜色空间
type ColorSpace struct {
	ID   string `xml:"ID,attr"`
	Type string `xml:"Type,attr"`
}

// Signatures 签章结构
type Signatures struct {
	XMLName   xml.Name    `xml:"Signatures"`
	Signature []Signature `xml:"Signature"`
}

// Signature 签章
type Signature struct {
	ID      string `xml:"ID,attr"`
	Type    string `xml:"Type,attr"`
	BaseLoc string `xml:"BaseLoc,attr"` // BaseLoc 是属性，不是子元素
}

// SignatureXML 签章XML - 支持多种格式和命名空间
type SignatureXML struct {
	XMLName     xml.Name   `xml:"Signature"`
	SignedInfo  SignedInfo `xml:"SignedInfo"`
	SignedValue string     `xml:"SignedValue"`
}

// SignatureXMLNS 带命名空间的签章XML
type SignatureXMLNS struct {
	XMLName     xml.Name   `xml:"http://www.ofdspec.org/2016 Signature"`
	SignedInfo  SignedInfo `xml:"http://www.ofdspec.org/2016 SignedInfo"`
	SignedValue string     `xml:"http://www.ofdspec.org/2016 SignedValue"`
}

// SignedInfo 签章信息
type SignedInfo struct {
	StampAnnot   []StampAnnot `xml:"StampAnnot"`
	Seal         SealRef      `xml:"Seal"`
	References   References   `xml:"References"`
	Provider     string       `xml:"Provider"`
	SignatureMethod string    `xml:"SignatureMethod"`
	// 带命名空间的版本
	StampAnnotNS []StampAnnot `xml:"http://www.ofdspec.org/2016 StampAnnot"`
	SealNS       SealRef      `xml:"http://www.ofdspec.org/2016 Seal"`
	// ofd: 前缀版本
	StampAnnotOFD []StampAnnot `xml:"ofd StampAnnot"`
}

// SealRef 印章引用
type SealRef struct {
	BaseLoc string `xml:"BaseLoc,attr"`
}

// References 引用
type References struct {
	Reference []Reference `xml:"Reference"`
}

// Reference 引用
type Reference struct {
	FileRef string `xml:"FileRef,attr"`
}

// StampAnnot 印章注释
type StampAnnot struct {
	ID       string `xml:"ID,attr"`
	PageRef  string `xml:"PageRef,attr"`
	Boundary string `xml:"Boundary,attr"`
}

// SealXML 印章数据
type SealXML struct {
	XMLName     xml.Name `xml:"Seal"`
	SealID      string   `xml:"SealID"`
	Type        string   `xml:"Type"`
	Picture     Picture  `xml:"Picture"`
	SealData    string   `xml:"SealData"`
	PictureSES  PictureSES `xml:"ofd:Picture"`
}

// PictureSES SES格式图片
type PictureSES struct {
	Type   string `xml:"Type,attr"`
	Width  float64 `xml:"Width,attr"`
	Height float64 `xml:"Height,attr"`
	Data   string `xml:",chardata"`
}

// Picture 图片
type Picture struct {
	Type   string `xml:"Type,attr"`
	Width  string `xml:"Width,attr"`
	Height string `xml:"Height,attr"`
	Data   string `xml:",chardata"`
}


// Annots 注释结构
type Annots struct {
	XMLName xml.Name `xml:"PageAnnot"`
	Annot   []Annot  `xml:"Annot"`
}

// Annot 注释
type Annot struct {
	ID         string      `xml:"ID,attr"`
	Type       string      `xml:"Type,attr"`
	Creator    string      `xml:"Creator,attr"`
	LastModify string      `xml:"LastModify,attr"`
	Subtype    string      `xml:"Subtype,attr"`
	Visible    bool        `xml:"Visible,attr"`
	Print      bool        `xml:"Print,attr"`
	NoZoom     bool        `xml:"NoZoom,attr"`
	NoRotate   bool        `xml:"NoRotate,attr"`
	Appearance Appearance  `xml:"Appearance"`
}

// Appearance 外观
type Appearance struct {
	Boundary string `xml:"Boundary,attr"`
}

package ofd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// PageRenderResult 页面渲染结果
type PageRenderResult struct {
	PageIndex   int               `json:"pageIndex"`
	Width       float64           `json:"width"`
	Height      float64           `json:"height"`
	CanvasData  *CanvasRenderData `json:"canvasData"`
	TextLayer   []TextItem        `json:"textLayer"`
	Error       string            `json:"error,omitempty"`
	Debug       *DebugInfo        `json:"debug,omitempty"`
}

// DebugInfo 调试信息
type DebugInfo struct {
	ImageIDs     []string   `json:"imageIDs"`
	RequestedIDs []string   `json:"requestedIDs"`
	MissingIDs   []string   `json:"missingIDs"`
	Stamps       []string   `json:"stamps"`
	Files        []string   `json:"files"`
	TextDebug    []string   `json:"textDebug,omitempty"`
}

// CanvasRenderData Canvas渲染数据
type CanvasRenderData struct {
	Paths  []PathData  `json:"paths"`
	Images []ImageData `json:"images"`
}

// PathData 路径数据
type PathData struct {
	Commands    string           `json:"commands"`
	FillColor   string           `json:"fillColor,omitempty"`
	StrokeColor string           `json:"strokeColor,omitempty"`
	LineWidth   float64          `json:"lineWidth"`
	LineJoin    string           `json:"lineJoin,omitempty"`    // miter, round, bevel
	LineCap     string           `json:"lineCap,omitempty"`     // butt, round, square
	X           float64          `json:"x"`
	Y           float64          `json:"y"`
	Gradient    *GradientData    `json:"gradient,omitempty"`
	Pattern     *PatternData     `json:"pattern,omitempty"`
}

// PatternData Pattern 填充数据
type PatternData struct {
	Width      float64     `json:"width"`      // 单元格内容宽度 (mm)
	Height     float64     `json:"height"`     // 单元格内容高度 (mm)
	XStep      float64     `json:"xStep"`      // X 方向步长 (mm)
	YStep      float64     `json:"yStep"`      // Y 方向步长 (mm)
	RelativeTo string      `json:"relativeTo"` // "Page" 或 "Object"
	CTM        []float64   `json:"ctm,omitempty"` // Pattern 的 CTM 变换 [a, b, c, d, e, f]
	CellImages []ImageData `json:"cellImages"` // 单元格中的图片
	CellPaths  []PathData  `json:"cellPaths"`  // 单元格中的路径
}

// GradientData 渐变数据
type GradientData struct {
	Type       string          `json:"type"` // "linear" 或 "radial"
	X0         float64         `json:"x0"`
	Y0         float64         `json:"y0"`
	X1         float64         `json:"x1"`
	Y1         float64         `json:"y1"`
	R0         float64         `json:"r0,omitempty"` // 径向渐变起始半径
	R1         float64         `json:"r1,omitempty"` // 径向渐变结束半径
	Stops      []GradientStop  `json:"stops"`
}

// GradientStop 渐变色标
type GradientStop struct {
	Position float64 `json:"position"`
	Color    string  `json:"color"`
}

// ImageData 图片数据
type ImageData struct {
	DataURL string    `json:"dataURL"`
	X       float64   `json:"x"`
	Y       float64   `json:"y"`
	Width   float64   `json:"width"`
	Height  float64   `json:"height"`
	CTM     []float64 `json:"ctm,omitempty"`    // CTM 变换矩阵 [a, b, c, d, e, f]
	Alpha   float64   `json:"alpha,omitempty"`   // 透明度 0~1，0 表示不设置
	IsSeal  bool      `json:"isSeal,omitempty"` // 是否是印章图片
}

// TextItem 文本项
type TextItem struct {
	Text         string    `json:"text"`
	X            float64   `json:"x"`
	Y            float64   `json:"y"`
	Width        float64   `json:"width,omitempty"`        // 宽度（用于占位框）
	Height       float64   `json:"height,omitempty"`       // 高度（用于占位框）
	BoundaryY    float64   `json:"boundaryY,omitempty"`    // Boundary 的 Y 坐标
	TextCodeY    float64   `json:"textCodeY,omitempty"`    // TextCode 的 Y 坐标（相对于 Boundary）
	FontSize     float64   `json:"fontSize"`
	FontFamily   string    `json:"fontFamily"`
	FontID       string    `json:"fontID"`
	Color        string    `json:"color"`
	CTM          []float64 `json:"ctm,omitempty"`          // 变换矩阵 [a, b, c, d, e, f]
	Stroke       bool      `json:"stroke"`                 // 是否描边
	StrokeColor  string    `json:"strokeColor,omitempty"`  // 描边颜色
	LineWidth    float64   `json:"lineWidth,omitempty"`    // 描边线宽
	Fill         bool      `json:"fill"`                   // 是否填充
}

// FontInfo 字体信息
type FontInfo struct {
	ID         string `json:"id"`
	FontName   string `json:"fontName"`
	FamilyName string `json:"familyName"`
	DataURL    string `json:"dataURL,omitempty"`
	HasFile    bool   `json:"hasFile"`
}

// GlyphMapping 记录一个字体中 Unicode 码点到 Glyph ID 的映射
// 从所有页面的 CGTransform 中收集
type GlyphMapping struct {
	Unicode rune
	GlyphID uint16
}

// RenderPage 渲染指定页面
func (p *Parser) RenderPage(pageIndex int) *PageRenderResult {
	result := &PageRenderResult{
		PageIndex:  pageIndex,
		CanvasData: &CanvasRenderData{},
		TextLayer:  make([]TextItem, 0),
	}

	if p.document == nil || pageIndex >= len(p.document.Pages.Page) {
		result.Error = "页面不存在"
		return result
	}

	// 获取页面路径
	pagePath := p.GetPagePath(pageIndex)
	if pagePath == "" {
		result.Error = "无法获取页面路径"
		return result
	}

	// 读取页面XML
	pageData, err := p.readFile(pagePath)
	if err != nil {
		result.Error = "读取页面失败: " + err.Error()
		return result
	}

	// 移除命名空间前缀，使 XML 解析更简单
	pageXML := removeNamespacePrefix(string(pageData))

	// 解析页面
	var page Page
	if err := xml.Unmarshal([]byte(pageXML), &page); err != nil {
		result.Error = "解析页面失败: " + err.Error()
		return result
	}

	// 获取页面尺寸
	width, height := p.getPageSize(&page)
	result.Width = width
	result.Height = height

	// 加载资源
	p.loadResources()

	// 提取渲染数据
	scale := 3.78 // mm to px
	p.extractRenderData(result, &page, scale, pageIndex)

	return result
}

// removeNamespacePrefix 移除 XML 中的命名空间前缀
func removeNamespacePrefix(xmlStr string) string {
	// 移除 ofd: 前缀
	re := regexp.MustCompile(`<(/?)ofd:`)
	xmlStr = re.ReplaceAllString(xmlStr, "<$1")
	
	// 移除 xmlns 声明
	re2 := regexp.MustCompile(`\s+xmlns:[^=]+="[^"]*"`)
	xmlStr = re2.ReplaceAllString(xmlStr, "")
	
	return xmlStr
}

// getPageSize 获取页面尺寸
func (p *Parser) getPageSize(page *Page) (float64, float64) { 
	if page.Area.PhysicalBox != "" {
		return parseBox(page.Area.PhysicalBox)
	}
	if p.document != nil && p.document.CommonData.PageArea.PhysicalBox != "" {
		return parseBox(p.document.CommonData.PageArea.PhysicalBox)
	}
	return 210, 297
}

func parseBox(box string) (float64, float64) {
	scanner := newNumberScanner(box)
	scanner.nextFloat() // skip x
	scanner.nextFloat() // skip y
	w, _ := scanner.nextFloat()
	h, _ := scanner.nextFloat()
	if w == 0 && h == 0 {
		return 210, 297
	}
	return w, h
}

// loadResources 加载资源（优化版：只加载字体声明，不加载字体文件） 
func (p *Parser) loadResources() {
	if p.fonts != nil {
		return
	}

	p.fonts = make(map[string]Font)
	p.images = make(map[string][]byte)
	p.fontFiles = make(map[string][]byte)

	// 获取文档根目录
	docBase := ""
	if p.ofd != nil && len(p.ofd.DocBody) > 0 {
		docRoot := p.ofd.DocBody[0].DocRoot
		docRoot = strings.TrimPrefix(docRoot, "/")
		docBase = path.Dir(docRoot)
	}

	// 只扫描资源声明文件，不加载实际资源
	for _, file := range p.files {
		lower := strings.ToLower(file)
		// 只处理资源声明文件
		if !strings.HasSuffix(lower, "publicres.xml") && 
		   !strings.HasSuffix(lower, "documentres.xml") &&
		   !(strings.Contains(lower, "res") && strings.HasSuffix(lower, ".xml")) {
			continue
		}
		
		data, err := p.readFile(file)
		if err != nil {
			continue
		}

		xmlStr := removeNamespacePrefix(string(data))
		var res Res
		if err := xml.Unmarshal([]byte(xmlStr), &res); err != nil {
			continue
		}

		basePath := path.Dir(file)

		// 只记录字体声明，不加载字体文件
		for _, font := range res.Fonts {
			p.fonts[font.ID] = font
			// 记录字体文件路径，但不立即加载
			if font.FontFile != "" {
				// 存储可能的路径供后续懒加载使用
				font.FontFile = path.Join(basePath, res.BaseLoc, font.FontFile)
				p.fonts[font.ID] = font
			}
		}

		// 只记录图片声明，不加载图片数据
		for _, media := range res.MultiMedias {
			if media.Type == "Image" {
				// 记录图片路径
				imgPath := path.Join(basePath, res.BaseLoc, media.MediaFile)
				// 存储路径而不是数据
				p.images[media.ID] = []byte(imgPath)
			}
		}
	}

	_ = docBase // 保留变量避免编译警告
}

// loadResourcesLazy 懒加载资源（按需加载图片）
func (p *Parser) loadImageLazy(resourceID string) []byte {
	// 如果已经是实际数据（长度大于路径长度），直接返回
	if data, ok := p.images[resourceID]; ok {
		if len(data) > 500 { // 实际图片数据肯定大于500字节
			return data
		}
		// 否则是路径，尝试加载
		imgPath := string(data)
		if imgData, err := p.readFile(imgPath); err == nil {
			p.images[resourceID] = imgData
			return imgData
		}
	}
	
	// 尝试直接用 ID 作为文件名查找
	for _, file := range p.files {
		lower := strings.ToLower(file)
		if strings.Contains(lower, strings.ToLower(resourceID)) {
			if strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || 
			   strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".gif") {
				if imgData, err := p.readFile(file); err == nil {
					p.images[resourceID] = imgData
					return imgData
				}
			}
		}
	}
	
	return nil
}

// GetFonts 获取所有字体信息
// 先扫描所有页面收集 CGTransform 中的 Unicode→GlyphID 映射，
// 然后将映射注入字体的 cmap 表，确保浏览器能正确渲染。
func (p *Parser) GetFonts() []FontInfo {
	p.loadResources()

	// 收集所有页面中每个字体的 Unicode→GlyphID 映射
	glyphMappings := p.collectGlyphMappings()

	fonts := make([]FontInfo, 0, len(p.fonts))
	for id, font := range p.fonts {
		info := FontInfo{
			ID:         id,
			FontName:   font.FontName,
			FamilyName: font.FamilyName,
			HasFile:    false,
		}
		
		// 检查是否有嵌入的字体文件（在 OFD 包内）
		if font.FontFile != "" {
			// 尝试加载字体文件
			if fontData, err := p.readFile(font.FontFile); err == nil && len(fontData) > 0 {
				// 修复字体结构问题，并注入 CGTransform 的 GlyphID 映射到 cmap
				mappings := glyphMappings[id]
				fontData = SanitizeFontWithMappings(fontData, mappings)

				info.HasFile = true
				// 检测字体类型
				mimeType := "font/ttf"
				if len(fontData) > 4 {
					switch {
					case fontData[0] == 0x00 && fontData[1] == 0x01:
						mimeType = "font/ttf"
					case fontData[0] == 0x4F && fontData[1] == 0x54:
						mimeType = "font/otf"
					case fontData[0] == 0x77 && fontData[1] == 0x4F && fontData[2] == 0x46 && fontData[3] == 0x46:
						mimeType = "font/woff"
					case fontData[0] == 0x77 && fontData[1] == 0x4F && fontData[2] == 0x46 && fontData[3] == 0x32:
						mimeType = "font/woff2"
					}
				}
				info.DataURL = fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(fontData))
				p.fontFiles[id] = fontData
			}
		}

		fonts = append(fonts, info)
	}

	return fonts
}

// collectGlyphMappings 扫描所有页面，收集每个字体的 Unicode→GlyphID 映射
// 返回 map[fontID][]GlyphMapping
func (p *Parser) collectGlyphMappings() map[string][]GlyphMapping {
	result := make(map[string][]GlyphMapping)

	if p.document == nil {
		return result
	}

	// 用于去重
	seen := make(map[string]map[uint32]bool) // fontID → set of (unicode<<16 | glyphID)

	for pageIdx := 0; pageIdx < len(p.document.Pages.Page); pageIdx++ {
		pagePath := p.GetPagePath(pageIdx)
		if pagePath == "" {
			continue
		}

		pageData, err := p.readFile(pagePath)
		if err != nil {
			continue
		}

		pageXML := removeNamespacePrefix(string(pageData))
		var page Page
		if err := xml.Unmarshal([]byte(pageXML), &page); err != nil {
			continue
		}

		// 合并所有 Layer 来源
		layers := page.Content.Layer
		if len(layers) == 0 {
			layers = page.ContentNS.Layer
		}
		if len(layers) == 0 {
			layers = page.Layer
		}

		for _, layer := range layers {
			for _, text := range layer.TextObjects {
				if len(text.CGTransform) == 0 {
					continue
				}

				fontID := text.Font

				if seen[fontID] == nil {
					seen[fontID] = make(map[uint32]bool)
				}

				for _, tc := range text.TextCode {
					chars := []rune(tc.Content)

					// 解析所有 CGTransform，建立 position→glyphIDs 映射
					for _, cgt := range text.CGTransform {
						if cgt.GetGlyphs() == "" {
							continue
						}

						glyphIDStrs := strings.Fields(cgt.GetGlyphs())
						codePos := cgt.CodePosition
						codeCount := cgt.CodeCount
						if codeCount == 0 {
							codeCount = 1
						}

						for gi, gidStr := range glyphIDStrs {
							gid, err := strconv.Atoi(gidStr)
							if err != nil || gid <= 0 {
								continue
							}

							// 对应的字符索引
							charIdx := codePos + gi
							if gi >= codeCount {
								// GlyphCount > CodeCount 时，多余的 glyph 对应同一组字符
								charIdx = codePos + codeCount - 1
							}

							if charIdx < len(chars) {
								ch := chars[charIdx]
								key := uint32(ch)<<16 | uint32(gid)
								if !seen[fontID][key] {
									seen[fontID][key] = true
									result[fontID] = append(result[fontID], GlyphMapping{
										Unicode: ch,
										GlyphID: uint16(gid),
									})
								}
							}
						}
					}
				}
			}
		}
	}

	return result
}

// GetDebugInfo 获取调试信息
func (p *Parser) GetDebugInfo() map[string]interface{} {
	p.loadResources()
	
	imageIDs := make([]string, 0, len(p.images))
	for id := range p.images {
		imageIDs = append(imageIDs, id)
	}
	
	fontIDs := make([]string, 0, len(p.fonts))
	for id := range p.fonts {
		fontIDs = append(fontIDs, id)
	}
	
	return map[string]interface{}{
		"files":    p.files,
		"imageIDs": imageIDs,
		"fontIDs":  fontIDs,
	}
}

// extractRenderData 提取渲染数据
func (p *Parser) extractRenderData(result *PageRenderResult, page *Page, scale float64, pageIndex int) {
	// 合并多种可能的 Content 来源
	layers := page.Content.Layer
	if len(layers) == 0 {
		layers = page.ContentNS.Layer
	}
	if len(layers) == 0 {
		layers = page.Layer
	}

	// 调试信息
	debug := &DebugInfo{
		ImageIDs:     make([]string, 0),
		RequestedIDs: make([]string, 0),
		MissingIDs:   make([]string, 0),
		Stamps:       make([]string, 0),
		Files:        p.files,
		TextDebug:    make([]string, 0),
	}
	for id := range p.images {
		debug.ImageIDs = append(debug.ImageIDs, id)
	}

	debug.TextDebug = append(debug.TextDebug, fmt.Sprintf("layers count: %d", len(layers)))

	for layerIdx, layer := range layers {
		debug.TextDebug = append(debug.TextDebug, fmt.Sprintf("layer[%d]: texts=%d, paths=%d, images=%d", 
			layerIdx, len(layer.TextObjects), len(layer.PathObjects), len(layer.ImageObjects)))

		// 提取图片
		for _, img := range layer.ImageObjects {
			debug.RequestedIDs = append(debug.RequestedIDs, img.ResourceID)
			if _, ok := p.images[img.ResourceID]; !ok {
				debug.MissingIDs = append(debug.MissingIDs, img.ResourceID)
			}
			p.extractImage(result, &img, scale)
		}

		// 提取路径
		for _, pathObj := range layer.PathObjects {
			p.extractPath(result, &pathObj, scale, result.Width, result.Height)
		}

		// 提取文本
		for _, text := range layer.TextObjects {
			p.extractText(result, &text, scale, debug)
		}
	}

	debug.TextDebug = append(debug.TextDebug, fmt.Sprintf("total texts: %d, paths: %d", 
		len(result.TextLayer), len(result.CanvasData.Paths)))

	// 加载页面注释中的图片
	p.loadPageAnnotImages(result, scale, pageIndex, debug)

	// 加载签章/印章
	p.loadStamps(result, scale, pageIndex, debug)

	result.Debug = debug
}

// loadPageAnnotImages 加载页面注释中的图片
// 通过 Document.xml 中的 Annotations 引用找到 Annotations.xml 索引文件，
// 再根据 PageID 找到对应页面的注释文件进行解析。
func (p *Parser) loadPageAnnotImages(result *PageRenderResult, scale float64, pageIndex int, debug *DebugInfo) {
	if p.document == nil || pageIndex >= len(p.document.Pages.Page) {
		return
	}

	pageID := p.document.Pages.Page[pageIndex].ID

	// 获取文档根目录
	docBase := ""
	if p.ofd != nil && len(p.ofd.DocBody) > 0 {
		docRoot := strings.TrimPrefix(p.ofd.DocBody[0].DocRoot, "/")
		docBase = path.Dir(docRoot)
	}

	// 1. 找到 Annotations.xml 索引文件
	annotIndexPath := ""
	if p.document.Annotations != "" {
		annotIndexPath = path.Join(docBase, strings.TrimPrefix(p.document.Annotations, "/"))
	}
	// 如果 Document.xml 中没有声明，尝试常见路径
	if annotIndexPath == "" {
		for _, file := range p.files {
			lower := strings.ToLower(file)
			if strings.HasSuffix(lower, "annotations.xml") && !strings.Contains(lower, "page_") {
				annotIndexPath = file
				break
			}
		}
	}
	if annotIndexPath == "" {
		return
	}

	indexData, err := p.readFile(annotIndexPath)
	if err != nil {
		debug.Stamps = append(debug.Stamps, "read annotations index error: "+err.Error())
		return
	}

	xmlStr := removeNamespacePrefix(string(indexData))
	var annotIndex AnnotationsFile
	if err := xml.Unmarshal([]byte(xmlStr), &annotIndex); err != nil {
		debug.Stamps = append(debug.Stamps, "parse annotations index error: "+err.Error())
		return
	}

	annotIndexDir := path.Dir(annotIndexPath)

	// 2. 找到当前页面对应的注释文件
	for _, ap := range annotIndex.Pages {
		if ap.PageID != pageID {
			continue
		}

		annotFilePath := path.Join(annotIndexDir, strings.TrimPrefix(ap.FileLoc, "/"))
		annotData, err := p.readFile(annotFilePath)
		if err != nil {
			debug.Stamps = append(debug.Stamps, "read annot file error: "+annotFilePath+" "+err.Error())
			continue
		}

		annotXML := removeNamespacePrefix(string(annotData))
		var pageAnnot PageAnnot
		if err := xml.Unmarshal([]byte(annotXML), &pageAnnot); err != nil {
			debug.Stamps = append(debug.Stamps, "parse annot error: "+err.Error())
			continue
		}

		// 3. 处理每个注释
		for _, annot := range pageAnnot.Annots {
			ax, ay, _, _ := parseBoundary(annot.Appearance.Boundary)

			// 收集所有 ImageObject：来自 PageBlock 和直接子元素
			var allImages []ImageObject
			for _, block := range annot.Appearance.PageBlocks {
				allImages = append(allImages, block.ImageObjects...)
			}
			allImages = append(allImages, annot.Appearance.ImageObjects...)

			for _, img := range allImages {
				p.renderAnnotImage(result, scale, &img, ax, ay, debug)
			}
		}
	}
}

// renderAnnotImage 渲染注释中的单个图片对象
func (p *Parser) renderAnnotImage(result *PageRenderResult, scale float64, img *ImageObject, ax, ay float64, debug *DebugInfo) {
	// 查找图片资源（先从文档资源中找，再懒加载）
	imgData := p.loadImageLazy(img.ResourceID)
	if imgData == nil || len(imgData) == 0 {
		debug.MissingIDs = append(debug.MissingIDs, img.ResourceID)
		return
	}

	ix, iy, iw, ih := parseBoundary(img.Boundary)

	mimeType := "image/png"
	if len(imgData) > 2 && imgData[0] == 0xFF && imgData[1] == 0xD8 {
		mimeType = "image/jpeg"
	}

	imgDataOut := ImageData{
		DataURL: fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(imgData)),
		X:       (ax + ix) * scale,
		Y:       (ay + iy) * scale,
		Width:   iw * scale,
		Height:  ih * scale,
	}

	// 处理 CTM 变换
	if img.CTM != "" {
		ctm := parseCTM(img.CTM)
		if len(ctm) >= 4 {
			e, f := 0.0, 0.0
			if len(ctm) > 4 {
				e = ctm[4] * scale
			}
			if len(ctm) > 5 {
				f = ctm[5] * scale
			}
			imgDataOut.CTM = []float64{
				ctm[0] * scale, ctm[1] * scale,
				ctm[2] * scale, ctm[3] * scale,
				e, f,
			}
		}
	}

	// 处理 Alpha 透明度（OFD 中 Alpha 范围 0~255）
	if img.Alpha > 0 && img.Alpha < 255 {
		imgDataOut.Alpha = float64(img.Alpha) / 255.0
	}

	result.CanvasData.Images = append(result.CanvasData.Images, imgDataOut)
	debug.Stamps = append(debug.Stamps, fmt.Sprintf("annot image: id=%s, pos=(%.1f,%.1f), alpha=%d",
		img.ResourceID, imgDataOut.X, imgDataOut.Y, img.Alpha))
}

// loadStamps 加载签章
func (p *Parser) loadStamps(result *PageRenderResult, scale float64, pageIndex int, debug *DebugInfo) {
	// 收集所有印章注释
	var allAnnots []struct {
		annot         StampAnnot
		sigDir        string
		sigID         string
		isPrivateAlgo bool
	}

	// 1. 查找 Signatures.xml
	for _, file := range p.files {
		lower := strings.ToLower(file)
		if strings.HasSuffix(lower, "signatures.xml") {
			debug.Stamps = append(debug.Stamps, "found signatures: "+file)

			data, err := p.readFile(file)
			if err != nil {
				debug.Stamps = append(debug.Stamps, "read error: "+err.Error())
				continue
			}

			// 移除命名空间前缀，使 XML 解析更简单
			xmlStr := removeNamespacePrefix(string(data))

			var sigs Signatures
			if err := xml.Unmarshal([]byte(xmlStr), &sigs); err != nil {
				debug.Stamps = append(debug.Stamps, "parse error: "+err.Error())
				continue
			}

			basePath := path.Dir(file)
			for _, sig := range sigs.Signature {
				result := fmt.Sprintf("sig: %+v", sigs)
				debug.Stamps = append(debug.Stamps, "sig: "+ result +" loc: "+sig.BaseLoc)

				// 尝试多种路径组合读取签章 XML
				sigLoc := strings.TrimPrefix(sig.BaseLoc, "/")
				candidates := []string{
					path.Join(basePath, sigLoc),
					sigLoc,
					path.Join(basePath, sig.ID, "Signature.xml"),
				}

				var sigData []byte
				var sigPath string
				for _, candidate := range candidates {
					if data, err := p.readFile(candidate); err == nil {
						sigData = data
						sigPath = candidate
						debug.Stamps = append(debug.Stamps, "sig loaded: "+candidate)
						break
					}
				}

				if sigData == nil {
					debug.Stamps = append(debug.Stamps, "sig read error: all paths failed")
					continue
				}

				// 尝试解析签章 XML（支持多种格式）并获取算法类型
				annots, isPrivateAlgo := p.parseSignatureXMLWithAlgo(sigData, debug)
				sigDir := path.Dir(sigPath)

				for _, annot := range annots {
					debug.Stamps = append(debug.Stamps, fmt.Sprintf("annot: pageRef=%s, boundary=%s, privateAlgo=%v", annot.PageRef, annot.Boundary, isPrivateAlgo))
					allAnnots = append(allAnnots, struct {
						annot         StampAnnot
						sigDir        string
						sigID         string
						isPrivateAlgo bool
					}{annot, sigDir, sig.ID, isPrivateAlgo})
				}
			}
		}
	}

	 
	pageAnnotPath := fmt.Sprintf("page_%d", pageIndex)
	for _, file := range p.files {
		lower := strings.ToLower(file)
		if (strings.Contains(lower, "annot") || strings.Contains(lower, "annotation")) && 
		   strings.HasSuffix(lower, ".xml") &&
		   (strings.Contains(lower, pageAnnotPath) || strings.Contains(lower, fmt.Sprintf("page_%d", pageIndex+1))) {
			debug.Stamps = append(debug.Stamps, "found page annots: "+file)
			
			data, err := p.readFile(file)
			if err != nil {
				continue
			}
			
			// 尝试解析注释文件
			var annots Annots
			if err := xml.Unmarshal(data, &annots); err == nil {
				for _, annot := range annots.Annot {
					if annot.Type == "Stamp" || strings.Contains(strings.ToLower(annot.Subtype), "stamp") ||
					   strings.Contains(strings.ToLower(annot.Type), "stamp") {
						debug.Stamps = append(debug.Stamps, fmt.Sprintf("found stamp annot: %s, boundary=%s", annot.ID, annot.Appearance.Boundary))
						allAnnots = append(allAnnots, struct {
							annot         StampAnnot
							sigDir        string
							sigID         string
							isPrivateAlgo bool
						}{
							StampAnnot{
								ID:       annot.ID,
								PageRef:  fmt.Sprintf("%d", pageIndex),
								Boundary: annot.Appearance.Boundary,
							},
							path.Dir(file),
							annot.ID,
							false, // 页面注释默认不是私有算法
						})
					}
				}
			}
			
			// 使用正则表达式提取印章注释
			content := string(data)
			re := regexp.MustCompile(`(?i)<(?:\w+:)?Annot[^>]*>`)
			tags := re.FindAllString(content, -1)
			for _, tag := range tags {
				if strings.Contains(strings.ToLower(tag), "stamp") {
					boundaryRe := regexp.MustCompile(`Boundary\s*=\s*"([^"]*)"`)
					boundaryMatch := boundaryRe.FindStringSubmatch(tag)
					if len(boundaryMatch) >= 2 {
						debug.Stamps = append(debug.Stamps, fmt.Sprintf("regex found annot: boundary=%s", boundaryMatch[1]))
						allAnnots = append(allAnnots, struct {
							annot         StampAnnot
							sigDir        string
							sigID         string
							isPrivateAlgo bool
						}{
							StampAnnot{
								PageRef:  fmt.Sprintf("%d", pageIndex),
								Boundary: boundaryMatch[1],
							},
							path.Dir(file),
							"",
							false, // 正则提取的默认不是私有算法
						})
					}
				}
			}
		}
	}

	// 3. 直接扫描所有印章相关文件
	for _, file := range p.files {
		lower := strings.ToLower(file)
		if (strings.Contains(lower, "seal") || strings.Contains(lower, "stamp")) && 
		   (strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || 
		    strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".esl")) {
			debug.Stamps = append(debug.Stamps, "found seal file: "+file)
		}
	}

	// 获取当前页面ID（从 Document.xml 中的 Pages.Page[pageIndex].ID）
	pageID := ""
	if p.document != nil && pageIndex < len(p.document.Pages.Page) {
		pageID = p.document.Pages.Page[pageIndex].ID
	}
	debug.Stamps = append(debug.Stamps, fmt.Sprintf("current pageID=%s, pageIndex=%d, totalAnnots=%d", pageID, pageIndex, len(allAnnots)))

	// 查找当前页面的印章
	// 只通过 PageRef 与 pageID 匹配，不使用页面索引
	for _, item := range allAnnots {
		// StampAnnot 的 PageRef 应该等于 Document.xml 中定义的页面 ID
		matched := item.annot.PageRef == pageID
		
		debug.Stamps = append(debug.Stamps, fmt.Sprintf("checking annot: pageRef=%s, currentPageID=%s, matched=%v, privateAlgo=%v", 
			item.annot.PageRef, pageID, matched, item.isPrivateAlgo))
		
		if matched {
			if item.isPrivateAlgo {
				// 私有算法：只显示占位框
				p.addPlaceholderStamp(result, scale, &item.annot, debug)
			} else {
				// 标准算法：加载真实印章图片
				p.loadSealImage(result, scale, item.sigDir, item.sigID, &item.annot, debug)
			}
		}
	}

	// 如果没有找到任何印章注释，说明文档没有印章
	if len(allAnnots) == 0 {
		debug.Stamps = append(debug.Stamps, "no stamp annotations found in document")
	} else if len(result.CanvasData.Images) == 0 {
		debug.Stamps = append(debug.Stamps, fmt.Sprintf("no stamps matched for page %s (index %d)", pageID, pageIndex))
	}
}

// loadSealImage 加载印章图片
func (p *Parser) loadSealImage(result *PageRenderResult, scale float64, sigDir string, sigID string, annot *StampAnnot, debug *DebugInfo) {
	debug.Stamps = append(debug.Stamps, "looking for seal in: "+sigDir)

	// 解析边界
	x, y, w, h := parseBoundary(annot.Boundary)
	if w == 0 || h == 0 {
		debug.Stamps = append(debug.Stamps, "invalid boundary, using default size")
		// 使用默认尺寸
		w = 40
		h = 40
	}
	debug.Stamps = append(debug.Stamps, fmt.Sprintf("boundary: x=%f, y=%f, w=%f, h=%f", x, y, w, h))

	// 收集所有可能的印章文件
	var candidates []string
	
	// 1. 签章目录下的文件
	candidates = append(candidates,
		path.Join(sigDir, "Seal.esl"),
		path.Join(sigDir, "seal.esl"),
		path.Join(sigDir, "Seal.png"),
		path.Join(sigDir, "seal.png"),
		path.Join(sigDir, "Stamp.png"),
		path.Join(sigDir, "stamp.png"),
		path.Join(sigDir, "SignedValue.dat"),
		path.Join(sigDir, "Seal.xml"),
		path.Join(sigDir, "seal.xml"),
	)

	// 2. 扫描签章目录下的所有文件
	sigDirLower := strings.ToLower(sigDir)
	for _, file := range p.files {
		fileLower := strings.ToLower(file)
		if strings.HasPrefix(fileLower, sigDirLower) || strings.Contains(file, sigDir) {
			if strings.HasSuffix(fileLower, ".png") || strings.HasSuffix(fileLower, ".jpg") ||
				strings.HasSuffix(fileLower, ".jpeg") || strings.HasSuffix(fileLower, ".esl") || 
				strings.HasSuffix(fileLower, ".dat") || strings.HasSuffix(fileLower, ".xml") {
				candidates = append(candidates, file)
			}
		}
	}

	// 3. 扫描所有包含 seal/sign/stamp 的文件
	for _, file := range p.files {
		fileLower := strings.ToLower(file)
		if strings.Contains(fileLower, "seal") || strings.Contains(fileLower, "sign") || strings.Contains(fileLower, "stamp") {
			if strings.HasSuffix(fileLower, ".png") || strings.HasSuffix(fileLower, ".jpg") ||
				strings.HasSuffix(fileLower, ".jpeg") || strings.HasSuffix(fileLower, ".esl") ||
				strings.HasSuffix(fileLower, ".dat") {
				candidates = append(candidates, file)
			}
		}
	}

	debug.Stamps = append(debug.Stamps, fmt.Sprintf("found %d candidates", len(candidates)))

	for _, candidate := range candidates {
		imgData, err := p.readFile(candidate)
		if err != nil {
			continue
		}

		debug.Stamps = append(debug.Stamps, "loaded: "+candidate+" size: "+fmt.Sprintf("%d", len(imgData)))

		lower := strings.ToLower(candidate)
		
		// 如果是 XML 文件，尝试解析 Seal.xml 格式
		if strings.HasSuffix(lower, ".xml") {
			extractedImg := p.extractSealFromXML(imgData, debug)
			if extractedImg != nil {
				imgData = extractedImg
				debug.Stamps = append(debug.Stamps, "extracted from XML, size: "+fmt.Sprintf("%d", len(imgData)))
			} else {
				continue
			}
		} else if strings.HasSuffix(lower, ".esl") || strings.HasSuffix(lower, ".dat") {
			// 如果是 ESL 或 DAT 文件，尝试提取图片
			extractedImg := p.extractESLImage(imgData)
			if extractedImg != nil {
				debug.Stamps = append(debug.Stamps, "extracted image, size: "+fmt.Sprintf("%d", len(extractedImg)))
				imgData = extractedImg
			} else {
				debug.Stamps = append(debug.Stamps, "extraction failed, trying next")
				continue
			}
		}

		// 检测图片类型
		mimeType := "image/png"
		if len(imgData) > 2 {
			if imgData[0] == 0xFF && imgData[1] == 0xD8 {
				mimeType = "image/jpeg"
			} else if imgData[0] == 0x89 && imgData[1] == 0x50 {
				mimeType = "image/png"
			} else if imgData[0] == 0x47 && imgData[1] == 0x49 {
				mimeType = "image/gif"
			}
		}

		result.CanvasData.Images = append(result.CanvasData.Images, ImageData{
			DataURL: fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(imgData)),
			X:       x * scale,
			Y:       y * scale,
			Width:   w * scale,
			Height:  h * scale,
			IsSeal:  true,
		})
		debug.Stamps = append(debug.Stamps, fmt.Sprintf("seal added at (%f,%f) size (%f,%f)", x*scale, y*scale, w*scale, h*scale))
		return
	}

	debug.Stamps = append(debug.Stamps, "no seal image found")
}

// addPlaceholderStamp 添加占位框（用于私有算法的印章）
func (p *Parser) addPlaceholderStamp(result *PageRenderResult, scale float64, annot *StampAnnot, debug *DebugInfo) {
	// 解析边界
	x, y, w, h := parseBoundary(annot.Boundary)
	if w == 0 || h == 0 {
		debug.Stamps = append(debug.Stamps, "invalid boundary for placeholder, using default size")
		w = 40
		h = 40
	}
	
	debug.Stamps = append(debug.Stamps, fmt.Sprintf("adding placeholder at (%f,%f) size (%f,%f)", x*scale, y*scale, w*scale, h*scale))
	
	// 添加一个特殊的 TextItem 作为占位标记
	// 使用特殊的 Text 值 "__PLACEHOLDER__" 来标识这是一个占位框
	result.TextLayer = append(result.TextLayer, TextItem{
		Text:   "__PLACEHOLDER__",
		X:      x * scale,
		Y:      y * scale,
		Width:  w * scale,
		Height: h * scale,
		Color:  "transparent",
	})
}

// extractSealFromXML 从 Seal.xml 文件中提取图片
func (p *Parser) extractSealFromXML(data []byte, debug *DebugInfo) []byte {
	// 尝试解析 Seal.xml
	var seal SealXML
	if err := xml.Unmarshal(data, &seal); err == nil {
		// 尝试从 Picture 元素获取图片数据
		if seal.Picture.Data != "" {
			imgData, err := base64.StdEncoding.DecodeString(strings.TrimSpace(seal.Picture.Data))
			if err == nil && len(imgData) > 0 {
				debug.Stamps = append(debug.Stamps, "extracted from Seal.Picture")
				return imgData
			}
		}
		// 尝试从 PictureSES 元素获取图片数据
		if seal.PictureSES.Data != "" {
			imgData, err := base64.StdEncoding.DecodeString(strings.TrimSpace(seal.PictureSES.Data))
			if err == nil && len(imgData) > 0 {
				debug.Stamps = append(debug.Stamps, "extracted from Seal.PictureSES")
				return imgData
			}
		}
		// 尝试从 SealData 元素获取图片数据
		if seal.SealData != "" {
			imgData, err := base64.StdEncoding.DecodeString(strings.TrimSpace(seal.SealData))
			if err == nil && len(imgData) > 0 {
				// 可能是嵌套的数据，尝试提取图片
				extracted := p.extractESLImage(imgData)
				if extracted != nil {
					debug.Stamps = append(debug.Stamps, "extracted from Seal.SealData")
					return extracted
				}
			}
		}
	}
	
	// 尝试使用正则表达式提取 base64 图片数据
	content := string(data)
	
	// 查找 Picture 元素中的 base64 数据
	re := regexp.MustCompile(`<(?:ofd:)?Picture[^>]*>([^<]+)</(?:ofd:)?Picture>`)
	matches := re.FindStringSubmatch(content)
	if len(matches) >= 2 {
		imgData, err := base64.StdEncoding.DecodeString(strings.TrimSpace(matches[1]))
		if err == nil && len(imgData) > 0 {
			debug.Stamps = append(debug.Stamps, "extracted from regex Picture")
			return imgData
		}
	}
	
	return nil
}

// extractESLImage 从 ESL/DAT 文件中提取图片
func (p *Parser) extractESLImage(data []byte) []byte {
	// ESL 是一种 OFD 签章格式，可能包含嵌入的图片
	// 尝试查找 PNG、JPEG 或 GIF 签名
	pngSig := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	jpgSig := []byte{0xFF, 0xD8, 0xFF}
	gifSig := []byte{0x47, 0x49, 0x46, 0x38}

	// 查找 PNG
	for i := 0; i < len(data)-8; i++ {
		if bytes.Equal(data[i:i+8], pngSig) {
			// 找到 PNG 结束标记 IEND
			for j := i + 8; j < len(data)-8; j++ {
				if data[j] == 0x49 && data[j+1] == 0x45 && data[j+2] == 0x4E && data[j+3] == 0x44 {
					// IEND chunk + CRC
					return data[i : j+8]
				}
			}
			// 如果没找到结束标记，返回剩余数据
			return data[i:]
		}
	}

	// 查找 JPEG
	for i := 0; i < len(data)-3; i++ {
		if bytes.Equal(data[i:i+3], jpgSig) {
			// 找到 JPEG 结束标记 FFD9
			for j := i + 3; j < len(data)-1; j++ {
				if data[j] == 0xFF && data[j+1] == 0xD9 {
					return data[i : j+2]
				}
			}
			return data[i:]
		}
	}

	// 查找 GIF
	for i := 0; i < len(data)-4; i++ {
		if bytes.Equal(data[i:i+4], gifSig) {
			return data[i:]
		}
	}

	return nil
}

// extractImage 提取图片数据
func (p *Parser) extractImage(result *PageRenderResult, img *ImageObject, scale float64) {
	// 使用懒加载获取图片数据
	imgData := p.loadImageLazy(img.ResourceID)
	if imgData == nil || len(imgData) == 0 {
		return
	}

	x, y, w, h := parseBoundary(img.Boundary)

	mimeType := "image/png"
	if len(imgData) > 2 && imgData[0] == 0xFF && imgData[1] == 0xD8 {
		mimeType = "image/jpeg"
	}

	imgDataOut := ImageData{
		DataURL: fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(imgData)),
		X:       x * scale,
		Y:       y * scale,
		Width:   w * scale,
		Height:  h * scale,
	}

	// 解析 CTM 变换矩阵
	if img.CTM != "" {
		ctm := parseCTM(img.CTM)
		if len(ctm) >= 4 {
			// CTM 格式: [a, b, c, d, e, f]
			// 检查是否有旋转（b 或 c 不为 0）
			if ctm[1] != 0 || ctm[2] != 0 {
				// 传递 CTM 用于前端变换
				// CTM 的所有值都是 mm 单位，需要转换为像素
				e := float64(0)
				f := float64(0)
				if len(ctm) > 4 {
					e = ctm[4] * scale
				}
				if len(ctm) > 5 {
					f = ctm[5] * scale
				}
				// a, b, c, d 也需要乘以 scale 转换为像素
				imgDataOut.CTM = []float64{
					ctm[0] * scale, ctm[1] * scale,
					ctm[2] * scale, ctm[3] * scale,
					e, f,
				}
			}
		}
	}

	result.CanvasData.Images = append(result.CanvasData.Images, imgDataOut)
}

// extractPath 提取路径数据
// pageWidth, pageHeight: 页面尺寸（mm），用于判断闭合线段是否在边界上
func (p *Parser) extractPath(result *PageRenderResult, pathObj *PathObject, scale float64, pageWidth, pageHeight float64) {
	if pathObj.AbbreviatedData == "" {
		return
	}

	strokeColor := ""
	fillColor := ""
	var gradient *GradientData
	var pattern *PatternData
	
	// 处理描边颜色
	// OFD 规范：只要定义了 StrokeColor，就应该进行描边渲染
	// WPS 行为：当定义了颜色但未定义线宽时，渲染引擎会将其视为 1 像素线
	if pathObj.StrokeColor != nil && pathObj.StrokeColor.Value != "" {
		strokeColor = parseColor(pathObj.StrokeColor.Value)
	} else if pathObj.Stroke || pathObj.LineWidth > 0 {
		// 如果有描边属性但没有颜色，使用默认黑色
		strokeColor = "#000"
	}
	
	// 解析 CTM 变换矩阵
	var ctm []float64
	if pathObj.CTM != "" {
		ctm = parseCTM(pathObj.CTM)
	}
	
	// 处理填充颜色、渐变或图案
	if pathObj.FillColor != nil {
		if pathObj.FillColor.Value != "" {
			// 纯色填充
			fillColor = parseColor(pathObj.FillColor.Value)
		} else if pathObj.FillColor.AxialShd != nil {
			// 轴向渐变（线性渐变）
			gradient = p.parseAxialShd(pathObj.FillColor.AxialShd, ctm, scale)
		} else if pathObj.FillColor.RadialShd != nil {
			// 径向渐变
			gradient = p.parseRadialShd(pathObj.FillColor.RadialShd, ctm, scale)
		} else if pathObj.FillColor.Pattern != nil {
			// Pattern 图案填充
			pattern = p.parsePattern(pathObj.FillColor.Pattern, scale)
		}
	}

	// 解析边界框（Boundary 定义了对象在页面上的位置）
	bx, by, _, _ := parseBoundary(pathObj.Boundary)
	
	// 默认线宽
	lineWidth := pathObj.LineWidth
	if lineWidth == 0 {
		lineWidth = 1/scale
	}else if len(ctm) >= 1 && ctm[0] > 0 {
		// 如果有 CTM 变换矩阵，lineWidth 需要乘以 CTM 的缩放因子
	// CTM 格式: [a, b, c, d, e, f]，其中 a 是 x 方向缩放因子
	// OFD 中 LineWidth 是在对象坐标系中定义的，需要乘以 CTM 缩放来得到页面坐标系中的线宽

		lineWidth = lineWidth * ctm[0]
	}
	
	// 正确的坐标变换流程：
	// 1. PathObject 的 AbbreviatedData 中的坐标是相对于对象自身坐标系的（通常从 0,0 开始）
	// 2. CTM 定义了对象坐标系到页面坐标系的变换（缩放 + 旋转）
	// 3. Boundary 定义了对象在页面上的位置（平移）
	// 
	// 变换顺序：对象坐标 -> CTM 变换（缩放） -> Boundary 平移 -> 像素缩放
	
	// 将渐变坐标转换为绝对坐标
	if gradient != nil {
		// 渐变坐标也需要应用 CTM 和 Boundary
		if len(ctm) >= 6 {
			// 渐变坐标已经在 parseAxialShd/parseRadialShd 中应用了 CTM
			// 这里只需要加上 Boundary 偏移
			gradient.X0 = (gradient.X0 + bx) * scale
			gradient.Y0 = (gradient.Y0 + by) * scale
			gradient.X1 = (gradient.X1 + bx) * scale
			gradient.Y1 = (gradient.Y1 + by) * scale
		} else {
			// 没有 CTM，直接加上 Boundary 偏移
			gradient.X0 = (bx + gradient.X0) * scale
			gradient.Y0 = (by + gradient.Y0) * scale
			gradient.X1 = (bx + gradient.X1) * scale
			gradient.Y1 = (by + gradient.Y1) * scale
		}
		if gradient.R0 > 0 {
			gradient.R0 = gradient.R0 * scale
		}
		if gradient.R1 > 0 {
			gradient.R1 = gradient.R1 * scale
		}
	}

	// 转换 OFD 的 Join 和 Cap 值到 Canvas 格式
	lineJoin := "miter" // 默认值
	if pathObj.Join != "" {
		switch strings.ToLower(pathObj.Join) {
		case "round":
			lineJoin = "round"
		case "bevel":
			lineJoin = "bevel"
		case "miter":
			lineJoin = "miter"
		}
	}
	
	lineCap := "butt" // 默认值
	if pathObj.Cap != "" {
		switch strings.ToLower(pathObj.Cap) {
		case "round":
			lineCap = "round"
		case "square":
			lineCap = "square"
		case "butt":
			lineCap = "butt"
		}
	}

	// 转换路径命令：应用 CTM 缩放和 Boundary 平移
	// 传递页面尺寸（像素），用于判断闭合线段是否在边界上
	result.CanvasData.Paths = append(result.CanvasData.Paths, PathData{
		Commands:    convertOFDPathToCanvasWithCTM(pathObj.AbbreviatedData, scale, bx, by, ctm, pageWidth*scale, pageHeight*scale),
		FillColor:   fillColor,
		StrokeColor: strokeColor,
		LineWidth:   lineWidth * scale,
		LineJoin:    lineJoin,
		LineCap:     lineCap,
		Gradient:    gradient,
		Pattern:     pattern,
		X:           0,
		Y:           0,
	})
}

// parseAxialShd 解析轴向渐变
func (p *Parser) parseAxialShd(shd *AxialShd, ctm []float64, scale float64) *GradientData {
	if shd == nil {
		return nil
	}
	
	// 解析起点和终点
	startParts := strings.Fields(shd.StartPoint)
	endParts := strings.Fields(shd.EndPoint)
	
	if len(startParts) < 2 || len(endParts) < 2 {
		return nil
	}
	
	x0, _ := strconv.ParseFloat(startParts[0], 64)
	y0, _ := strconv.ParseFloat(startParts[1], 64)
	x1, _ := strconv.ParseFloat(endParts[0], 64)
	y1, _ := strconv.ParseFloat(endParts[1], 64)
	
	// 如果有 CTM，应用变换到渐变坐标
	// CTM 格式: [a, b, c, d, e, f]
	// 变换公式: x' = a*x + c*y + e, y' = b*x + d*y + f
	if len(ctm) >= 6 {
		origX0 := x0
		origY0 := y0
		origX1 := x1
		origY1 := y1
		
		x0 = origX0*ctm[0] + origY0*ctm[2] + ctm[4]
		y0 = origX0*ctm[1] + origY0*ctm[3] + ctm[5]
		x1 = origX1*ctm[0] + origY1*ctm[2] + ctm[4]
		y1 = origX1*ctm[1] + origY1*ctm[3] + ctm[5]
	}
	
	// 解析渐变色标
	stops := make([]GradientStop, 0, len(shd.Segment))
	for _, seg := range shd.Segment {
		stops = append(stops, GradientStop{
			Position: seg.Position,
			Color:    parseColor(seg.Color.Value),
		})
	}
	
	return &GradientData{
		Type:  "linear",
		X0:    x0,
		Y0:    y0,
		X1:    x1,
		Y1:    y1,
		Stops: stops,
	}
}

// parseRadialShd 解析径向渐变
func (p *Parser) parseRadialShd(shd *RadialShd, ctm []float64, scale float64) *GradientData {
	if shd == nil {
		return nil
	}
	
	// 解析起点和终点
	startParts := strings.Fields(shd.StartPoint)
	endParts := strings.Fields(shd.EndPoint)
	
	if len(startParts) < 2 || len(endParts) < 2 {
		return nil
	}
	
	x0, _ := strconv.ParseFloat(startParts[0], 64)
	y0, _ := strconv.ParseFloat(startParts[1], 64)
	x1, _ := strconv.ParseFloat(endParts[0], 64)
	y1, _ := strconv.ParseFloat(endParts[1], 64)
	
	// 如果有 CTM，应用变换到渐变坐标
	if len(ctm) >= 6 {
		origX0 := x0
		origY0 := y0
		origX1 := x1
		origY1 := y1
		
		x0 = origX0*ctm[0] + origY0*ctm[2] + ctm[4]
		y0 = origX0*ctm[1] + origY0*ctm[3] + ctm[5]
		x1 = origX1*ctm[0] + origY1*ctm[2] + ctm[4]
		y1 = origX1*ctm[1] + origY1*ctm[3] + ctm[5]
	}
	
	// 解析渐变色标
	stops := make([]GradientStop, 0, len(shd.Segment))
	for _, seg := range shd.Segment {
		stops = append(stops, GradientStop{
			Position: seg.Position,
			Color:    parseColor(seg.Color.Value),
		})
	}
	
	return &GradientData{
		Type:  "radial",
		X0:    x0,
		Y0:    y0,
		X1:    x1,
		Y1:    y1,
		R0:    shd.StartRadius,
		R1:    shd.EndRadius,
		Stops: stops,
	}
}

// parsePattern 解析 Pattern 图案填充
// 
// Pattern 元素示例：
// <ofd:Pattern Width="467" Height="155" XStep="1920" YStep="1080" RelativeTo="Page" 
//              CTM="0.2393 0 0 0.2393 165.7258 -152.0952">
//   <ofd:CellContent>
//     <ofd:ImageObject ID="5" CTM="467 0 0 155 0 0" Boundary="0 0 467 155" ResourceID="4"/>
//   </ofd:CellContent>
// </ofd:Pattern>
//
// 关键参数：
// - Width, Height: 单元格内容尺寸 (mm)
// - XStep, YStep: 平铺步长 (mm)，应用 CTM 缩放后得到实际步长
// - CTM: [a, b, c, d, e, f]
//   - a, d: 缩放因子（如 0.2393）
//   - e, f: 起始偏移（mm），f 可能为负数表示第一个 tile 在页面外
//
// 平铺计算示例（页面高 190.5mm）：
// - 起始 Y = f = -152.1mm（页面外）
// - 步长 = YStep * d = 1080 * 0.2393 = 258.44mm
// - Tile 0: Y = -152.1mm（不可见）
// - Tile 1: Y = -152.1 + 258.44 = 106.34mm（可见，在页面中下部）
func (p *Parser) parsePattern(pattern *Pattern, scale float64) *PatternData {
	if pattern == nil {
		return nil
	}

	patternData := &PatternData{
		Width:      pattern.Width,
		Height:     pattern.Height,
		XStep:      pattern.XStep,
		YStep:      pattern.YStep,
		RelativeTo: pattern.RelativeTo,
		CellImages: make([]ImageData, 0),
		CellPaths:  make([]PathData, 0),
	}

	// 如果没有指定 RelativeTo，默认为 Page
	if patternData.RelativeTo == "" {
		patternData.RelativeTo = "Page"
	}

	// 解析 Pattern 的 CTM
	// CTM 格式: "a b c d e f"
	// - a, d: 缩放因子
	// - e, f: 起始偏移（mm）
	if pattern.CTM != "" {
		patternData.CTM = parseCTM(pattern.CTM)
	}

	// 处理 CellContent 中的图片
	for _, img := range pattern.CellContent.ImageObjects {
		// 使用懒加载获取图片数据
		imgBytes := p.loadImageLazy(img.ResourceID)
		if imgBytes == nil || len(imgBytes) == 0 {
			continue
		}

		// 解析图片的 Boundary
		imgX, imgY, imgW, imgH := parseBoundary(img.Boundary)

		// 如果有 CTM，使用 CTM 中的尺寸
		// ImageObject 的 CTM 格式: "w 0 0 h 0 0" 表示宽高
		if img.CTM != "" {
			imgCTM := parseCTM(img.CTM)
			if len(imgCTM) >= 4 {
				imgW = imgCTM[0]
				imgH = imgCTM[3]
			}
		}

		// 检测图片类型
		mimeType := "image/png"
		if len(imgBytes) > 2 && imgBytes[0] == 0xFF && imgBytes[1] == 0xD8 {
			mimeType = "image/jpeg"
		}

		// 添加图片到 Pattern（坐标保持 mm 单位，前端会处理缩放）
		patternData.CellImages = append(patternData.CellImages, ImageData{
			DataURL: fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(imgBytes)),
			X:       imgX,
			Y:       imgY,
			Width:   imgW,
			Height:  imgH,
		})
	}

	// 处理 CellContent 中的路径
	for _, pathObj := range pattern.CellContent.PathObjects {
		if pathObj.AbbreviatedData == "" {
			continue
		}

		// 解析路径的填充和描边
		fillColor := ""
		strokeColor := ""

		if pathObj.FillColor != nil && pathObj.FillColor.Value != "" {
			fillColor = parseColor(pathObj.FillColor.Value)
		}

		if pathObj.StrokeColor != nil && pathObj.StrokeColor.Value != "" {
			strokeColor = parseColor(pathObj.StrokeColor.Value)
		}

		// 解析边界
		bx, by, _, _ := parseBoundary(pathObj.Boundary)

		// 解析 CTM
		var ctm []float64
		if pathObj.CTM != "" {
			ctm = parseCTM(pathObj.CTM)
		}

		lineWidth := pathObj.LineWidth
		if lineWidth == 0 {
			lineWidth = 0.353
		}

		// 添加路径到 Pattern（坐标保持 mm 单位，不乘以 scale）
		// Pattern 内部路径不需要检查页面边界，传递 0, 0
		patternData.CellPaths = append(patternData.CellPaths, PathData{
			Commands:    convertOFDPathToCanvasWithCTM(pathObj.AbbreviatedData, 1.0, bx, by, ctm, 0, 0),
			FillColor:   fillColor,
			StrokeColor: strokeColor,
			LineWidth:   lineWidth,
		})
	}

	return patternData
}


// extractText 提取文本数据
func (p *Parser) extractText(result *PageRenderResult, text *TextObject, scale float64, debug *DebugInfo) {
	bx, by, _, _ := parseBoundary(text.Boundary)
	
	fontID := text.Font
	fontFamily := "SimSun, serif"
	if font, ok := p.fonts[text.Font]; ok {
		if _, hasFile := p.fontFiles[text.Font]; hasFile {
			fontFamily = fmt.Sprintf("'OFD_Font_%s', '%s', '%s', SimSun, serif", text.Font, font.FontName, font.FamilyName)
		} else {
			fontFamily = fmt.Sprintf("'%s', '%s', SimSun, serif", font.FontName, font.FamilyName)
		}
	}

	// OFD 中 Size 是字体大小（在对象坐标空间中，单位 mm）
	// 前端会通过 ctx.transform 应用 CTM 缩放，所以这里保持原始值
	fontSize := text.Size * scale

	// 处理填充颜色
	fillColor := "#000"
	if text.FillColor != nil && text.FillColor.Value != "" {
		fillColor = parseColor(text.FillColor.Value)
	}

	// 处理描边颜色
	strokeColor := ""
	if text.StrokeColor != nil && text.StrokeColor.Value != "" {
		strokeColor = parseColor(text.StrokeColor.Value)
	}

	// 确定主颜色（用于显示）
	color := fillColor
	if text.Stroke && !text.Fill && strokeColor != "" {
		color = strokeColor
	}

	// 确定是否填充：默认填充，除非只有描边且没有显式设置 Fill
	// OFD 规范中，文字默认是填充的
	shouldFill := true
	if text.Stroke && !text.Fill {
		// 只有描边，没有填充 - 但为了视觉效果，仍然填充
		// 因为纯描边文字看起来是空心的，不符合预期
		shouldFill = true
	}

	// 处理描边线宽
	lineWidth := text.LineWidth * scale
	if lineWidth == 0 && text.Stroke {
		lineWidth = 1 // 默认描边宽度
	}
	
	// 如果有 CTM 变换矩阵，lineWidth 需要乘以 CTM 的缩放因子
	// CTM 格式: [a, b, c, d, e, f]，其中 a 是 x 方向缩放因子
	// OFD 中 LineWidth 是在对象坐标系中定义的，需要乘以 CTM 缩放来得到页面坐标系中的线宽
	if text.CTM != "" && lineWidth > 0 {
		ctmScale := parseCTM(text.CTM)
		if len(ctmScale) >= 1 && ctmScale[0] > 0 {
			lineWidth = lineWidth * ctmScale[0]
		}
	}

	// 解析 CTM 变换矩阵，并转换 e, f 为像素
	var ctm []float64
	if text.CTM != "" {
		ctm = parseCTM(text.CTM)
		// e, f 是平移量（单位 mm），需要转换为像素
		if len(ctm) > 4 {
			ctm[4] = ctm[4] * scale
		}
		if len(ctm) > 5 {
			ctm[5] = ctm[5] * scale
		}
	}

	for _, tc := range text.TextCode {
		// 不要 TrimSpace，保留原始内容（包括空格）
		content := tc.Content
		
		// 获取 GlyphCount 用于确定实际字符数
		glyphCount := 0
		var glyphIDs []int
		
		// 如果有 CGTransform，解析 Glyphs
		if len(text.CGTransform) > 0 {
			for _, cgt := range text.CGTransform {
				glyphCount = cgt.GlyphCount
				if cgt.GetGlyphs() != "" {
					glyphIDStrs := strings.Fields(cgt.GetGlyphs())
					for _, gidStr := range glyphIDStrs {
						gid, err := strconv.Atoi(gidStr)
						if err == nil {
							glyphIDs = append(glyphIDs, gid)
						}
					}
				}
			}
		}
		
		// 解析 DeltaX - 字符间距数组（单位 mm）
		var deltaX []float64
		if tc.DeltaX != "" {
			deltaX = parseDeltas(tc.DeltaX)
		}

		// TextCode 的 X, Y 是在文本对象坐标空间中的坐标
		// 如果有 CTM，需要用 CTM 的缩放/旋转部分 (a,b,c,d) 变换到页面空间
		// CTM 的平移部分 (e,f) 由前端 ctx.transform 处理，这里不包含
		tcXmm := tc.X
		tcYmm := tc.Y
		if len(ctm) >= 4 {
			// CTM 变换（仅缩放/旋转）: newX = a*x + c*y, newY = b*x + d*y
			a, b, c, d := ctm[0], ctm[1], ctm[2], ctm[3]
			tcXmm = a*tc.X + c*tc.Y
			tcYmm = b*tc.X + d*tc.Y
		}
		tcX := (bx + tcXmm) * scale
		tcY := (by + tcYmm) * scale

		// 如果有 GlyphCount，使用它来确定字符数
		// Glyphs 中的 3 通常代表空格
		chars := []rune(content)
		
		// 如果 GlyphCount 与字符数不匹配，可能需要调整
		// 但我们仍然按 DeltaX 的数量来定位
		
		// 调试输出
		if debug != nil {
			debug.TextDebug = append(debug.TextDebug,
				fmt.Sprintf("text='%s' glyphCount=%d glyphIDs=%d deltaX=%d chars=%d",
					content, glyphCount, len(glyphIDs), len(deltaX), len(chars)))
		}

		// 逐字符输出，使用 DeltaX 计算位置
		currentX := tcX

		for i, char := range chars {
			// 检查是否是空格（Glyph ID 3 通常是空格）
			isSpace := false
			if i < len(glyphIDs) && glyphIDs[i] == 3 {
				isSpace = true
			} else if char == ' ' || char == '\u3000' {
				isSpace = true
			}
			
			// 只输出非空格字符，但空格的 DeltaX 仍然要计算
			if !isSpace {
				result.TextLayer = append(result.TextLayer, TextItem{
					Text:        string(char),
					X:           currentX,
					Y:           tcY,
					BoundaryY:   by * scale,    // Boundary 的 Y 坐标（像素）
					TextCodeY:   tc.Y * scale,  // TextCode 的 Y 坐标（像素）
					FontSize:    fontSize,
					FontFamily:  fontFamily,
					FontID:      fontID,
					Color:       color,
					CTM:         ctm,
					Stroke:      text.Stroke,
					StrokeColor: strokeColor,
					LineWidth:   lineWidth,
					Fill:        shouldFill,
				})
			}

			// 计算下一个字符的位置（包括空格的间距）
			if i < len(deltaX) {
				delta := deltaX[i] * scale
				if len(ctm) >= 1 && ctm[0] != 0 {
					delta = delta * ctm[0]
				}
				currentX += delta
			} else if i < len(chars)-1 {
				// 没有 DeltaX，使用字号作为默认间距
				defaultWidth := text.Size * scale
				if len(ctm) >= 1 && ctm[0] != 0 {
					defaultWidth = defaultWidth * ctm[0]
				}
				currentX += defaultWidth
			}
		}
	}
}

// parseCTM 解析 CTM 变换矩阵（Scanner 方式）
// 格式: "a b c d e f" 或 "a b c d"
func parseCTM(ctmStr string) []float64 {
	result := make([]float64, 0, 6)
	scanner := newNumberScanner(ctmStr)
	for {
		val, ok := scanner.nextFloat()
		if !ok {
			break
		}
		result = append(result, val)
		if len(result) >= 6 {
			break
		}
	}
	if len(result) < 4 {
		return nil
	}
	return result
}

// parseDeltas 解析 DeltaX/DeltaY 字符串（Scanner 方式）
// 支持格式: "1 2 3" 或 "g 5 3" (g表示重复 - g count value)
func parseDeltas(deltaStr string) []float64 {
	result := make([]float64, 0, 32)
	scanner := newTokenScanner(deltaStr)
	
	for {
		token, ok := scanner.nextToken()
		if !ok {
			break
		}
		
		if token == "g" {
			// "g count value" 格式
			countToken, ok1 := scanner.nextToken()
			valToken, ok2 := scanner.nextToken()
			if ok1 && ok2 {
				count, err1 := strconv.Atoi(countToken)
				val, err2 := strconv.ParseFloat(valToken, 64)
				if err1 == nil && err2 == nil {
					for j := 0; j < count; j++ {
						result = append(result, val)
					}
				}
			}
		} else {
			val, err := strconv.ParseFloat(token, 64)
			if err == nil {
				result = append(result, val)
			}
		}
	}
	return result
}

// numberScanner 数字扫描器
type numberScanner struct {
	data []byte
	pos  int
}

func newNumberScanner(s string) *numberScanner {
	return &numberScanner{data: []byte(s), pos: 0}
}

func (s *numberScanner) skipWhitespace() {
	for s.pos < len(s.data) {
		c := s.data[s.pos]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',' {
			s.pos++
		} else {
			break
		}
	}
}

func (s *numberScanner) nextFloat() (float64, bool) {
	s.skipWhitespace()
	if s.pos >= len(s.data) {
		return 0, false
	}
	
	start := s.pos
	// 处理符号
	if s.pos < len(s.data) && (s.data[s.pos] == '-' || s.data[s.pos] == '+') {
		s.pos++
	}
	// 整数部分
	for s.pos < len(s.data) && s.data[s.pos] >= '0' && s.data[s.pos] <= '9' {
		s.pos++
	}
	// 小数部分
	if s.pos < len(s.data) && s.data[s.pos] == '.' {
		s.pos++
		for s.pos < len(s.data) && s.data[s.pos] >= '0' && s.data[s.pos] <= '9' {
			s.pos++
		}
	}
	// 科学计数法
	if s.pos < len(s.data) && (s.data[s.pos] == 'e' || s.data[s.pos] == 'E') {
		s.pos++
		if s.pos < len(s.data) && (s.data[s.pos] == '-' || s.data[s.pos] == '+') {
			s.pos++
		}
		for s.pos < len(s.data) && s.data[s.pos] >= '0' && s.data[s.pos] <= '9' {
			s.pos++
		}
	}
	
	if s.pos == start {
		return 0, false
	}
	
	val, err := strconv.ParseFloat(string(s.data[start:s.pos]), 64)
	if err != nil {
		return 0, false
	}
	return val, true
}

// tokenScanner 通用 token 扫描器
type tokenScanner struct {
	data []byte
	pos  int
}

func newTokenScanner(s string) *tokenScanner {
	return &tokenScanner{data: []byte(s), pos: 0}
}

func (s *tokenScanner) skipWhitespace() {
	for s.pos < len(s.data) {
		c := s.data[s.pos]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',' {
			s.pos++
		} else {
			break
		}
	}
}

func (s *tokenScanner) nextToken() (string, bool) {
	s.skipWhitespace()
	if s.pos >= len(s.data) {
		return "", false
	}
	
	start := s.pos
	for s.pos < len(s.data) {
		c := s.data[s.pos]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',' {
			break
		}
		s.pos++
	}
	
	if s.pos == start {
		return "", false
	}
	return string(s.data[start:s.pos]), true
}

// pathScanner 路径命令扫描器
type pathScanner struct {
	data []byte
	pos  int
}

func newPathScanner(s string) *pathScanner {
	return &pathScanner{data: []byte(s), pos: 0}
}

func (s *pathScanner) skipWhitespace() {
	for s.pos < len(s.data) {
		c := s.data[s.pos]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',' {
			s.pos++
		} else {
			break
		}
	}
}

// nextCommand 获取下一个命令字母，如果当前位置不是字母则返回空
func (s *pathScanner) nextCommand() (byte, bool) {
	s.skipWhitespace()
	if s.pos >= len(s.data) {
		return 0, false
	}
	c := s.data[s.pos]
	if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
		s.pos++
		return c, true
	}
	return 0, false
}

// peekCommand 查看当前位置是否是命令字母（不移动位置）
func (s *pathScanner) peekCommand() (byte, bool) {
	s.skipWhitespace()
	if s.pos >= len(s.data) {
		return 0, false
	}
	c := s.data[s.pos]
	if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
		return c, true
	}
	return 0, false
}

// nextFloat 获取下一个浮点数
func (s *pathScanner) nextFloat() (float64, bool) {
	s.skipWhitespace()
	if s.pos >= len(s.data) {
		return 0, false
	}
	
	// 检查是否是命令字母
	c := s.data[s.pos]
	if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
		return 0, false
	}
	
	start := s.pos
	// 处理符号
	if s.pos < len(s.data) && (s.data[s.pos] == '-' || s.data[s.pos] == '+') {
		s.pos++
	}
	// 整数部分
	for s.pos < len(s.data) && s.data[s.pos] >= '0' && s.data[s.pos] <= '9' {
		s.pos++
	}
	// 小数部分
	if s.pos < len(s.data) && s.data[s.pos] == '.' {
		s.pos++
		for s.pos < len(s.data) && s.data[s.pos] >= '0' && s.data[s.pos] <= '9' {
			s.pos++
		}
	}
	// 科学计数法
	if s.pos < len(s.data) && (s.data[s.pos] == 'e' || s.data[s.pos] == 'E') {
		s.pos++
		if s.pos < len(s.data) && (s.data[s.pos] == '-' || s.data[s.pos] == '+') {
			s.pos++
		}
		for s.pos < len(s.data) && s.data[s.pos] >= '0' && s.data[s.pos] <= '9' {
			s.pos++
		}
	}
	
	if s.pos == start {
		return 0, false
	}
	
	val, err := strconv.ParseFloat(string(s.data[start:s.pos]), 64)
	if err != nil {
		return 0, false
	}
	return val, true
}

func (s *pathScanner) hasMore() bool {
	s.skipWhitespace()
	return s.pos < len(s.data)
}

// convertOFDPathToCanvasWithCTM 转换OFD路径命令为Canvas命令JSON（Scanner 方式）
// pageWidth, pageHeight: 页面尺寸（像素），用于判断闭合线段是否在边界上
func convertOFDPathToCanvasWithCTM(data string, scale float64, offsetX, offsetY float64, ctm []float64, pageWidth, pageHeight float64) string {
	var commands []map[string]interface{}

	// CTM 变换函数
	transformPoint := func(x, y float64) (float64, float64) {
		if len(ctm) >= 6 {
			tx := ctm[0]*x + ctm[2]*y + ctm[4]
			ty := ctm[1]*x + ctm[3]*y + ctm[5]
			tx += offsetX
			ty += offsetY
			return tx * scale, ty * scale
		}
		return (offsetX + x) * scale, (offsetY + y) * scale
	}

	// 判断线段是否在页面边界上
	// 边界容差（像素）
	const tolerance = 1.0
	isOnBoundary := func(x1, y1, x2, y2 float64) bool {
		// 检查是否在 x=0 边界上（左边界）
		if x1 <= tolerance && x2 <= tolerance {
			return true
		}
		// 检查是否在 y=0 边界上（上边界）
		if y1 <= tolerance && y2 <= tolerance {
			return true
		}
		// 检查是否在 x=pageWidth 边界上（右边界）
		if pageWidth > 0 && x1 >= pageWidth-tolerance && x2 >= pageWidth-tolerance {
			return true
		}
		// 检查是否在 y=pageHeight 边界上（下边界）
		if pageHeight > 0 && y1 >= pageHeight-tolerance && y2 >= pageHeight-tolerance {
			return true
		}
		return false
	}

	scanner := newPathScanner(data)
	var currentCmd byte = 0
	
	// 记录路径起点和当前点，用于判断闭合线段
	var startX, startY float64 // 当前子路径的起点（像素坐标）
	var currentX, currentY float64 // 当前点（像素坐标）
	hasStart := false

	for scanner.hasMore() {
		// 尝试读取命令字母
		if cmd, ok := scanner.peekCommand(); ok {
			scanner.nextCommand()
			currentCmd = cmd
		}

		switch currentCmd {
		case 'S', 'M', 's', 'm': // MoveTo
			x, ok1 := scanner.nextFloat()
			y, ok2 := scanner.nextFloat()
			if ok1 && ok2 {
				tx, ty := transformPoint(x, y)
				commands = append(commands, map[string]interface{}{
					"cmd": "M", "x": tx, "y": ty,
				})
				// 记录起点
				startX, startY = tx, ty
				currentX, currentY = tx, ty
				hasStart = true
				// M 后面的数字视为 L
				if currentCmd == 'M' || currentCmd == 'm' {
					currentCmd = 'L'
				}
			}
		case 'L', 'l': // LineTo
			x, ok1 := scanner.nextFloat()
			y, ok2 := scanner.nextFloat()
			if ok1 && ok2 {
				tx, ty := transformPoint(x, y)
				commands = append(commands, map[string]interface{}{
					"cmd": "L", "x": tx, "y": ty,
				})
				currentX, currentY = tx, ty
			}
		case 'B', 'b': // Bezier (OFD)
			x1, ok1 := scanner.nextFloat()
			y1, ok2 := scanner.nextFloat()
			x2, ok3 := scanner.nextFloat()
			y2, ok4 := scanner.nextFloat()
			x3, ok5 := scanner.nextFloat()
			y3, ok6 := scanner.nextFloat()
			if ok1 && ok2 && ok3 && ok4 && ok5 && ok6 {
				tx1, ty1 := transformPoint(x1, y1)
				tx2, ty2 := transformPoint(x2, y2)
				tx3, ty3 := transformPoint(x3, y3)
				commands = append(commands, map[string]interface{}{
					"cmd": "C",
					"x1": tx1, "y1": ty1,
					"x2": tx2, "y2": ty2,
					"x":  tx3, "y":  ty3,
				})
				currentX, currentY = tx3, ty3
			}
		case 'Q', 'q': // Quadratic
			x1, ok1 := scanner.nextFloat()
			y1, ok2 := scanner.nextFloat()
			x2, ok3 := scanner.nextFloat()
			y2, ok4 := scanner.nextFloat()
			if ok1 && ok2 && ok3 && ok4 {
				tx1, ty1 := transformPoint(x1, y1)
				tx2, ty2 := transformPoint(x2, y2)
				commands = append(commands, map[string]interface{}{
					"cmd": "Q",
					"x1": tx1, "y1": ty1,
					"x":  tx2, "y":  ty2,
				})
				currentX, currentY = tx2, ty2
			}
		case 'C', 'c': // Close (OFD)
			// 检查闭合线段是否在页面边界上
			// 如果闭合线段（从当前点到起点）落在边界上，则忽略闭合指令
			if hasStart && isOnBoundary(currentX, currentY, startX, startY) {
				// 闭合线段在边界上，忽略闭合，作为开放路径处理
				// 不添加 Z 命令
			} else {
				commands = append(commands, map[string]interface{}{"cmd": "Z"})
			}
			currentCmd = 0
			hasStart = false
		case 'A', 'a': // Arc - 跳过7个参数
			for i := 0; i < 7; i++ {
				scanner.nextFloat()
			}
		case 'Z', 'z': // Close (SVG style)
			// 同样检查闭合线段是否在页面边界上
			if hasStart && isOnBoundary(currentX, currentY, startX, startY) {
				// 闭合线段在边界上，忽略闭合
			} else {
				commands = append(commands, map[string]interface{}{"cmd": "Z"})
			}
			currentCmd = 0
			hasStart = false
		default:
			// 未知命令，尝试跳过一个数字
			scanner.nextFloat()
		}
	}

	jsonData, _ := json.Marshal(commands)
	return string(jsonData)
}
// convertOFDPathToCanvas 转换OFD路径命令为Canvas命令JSON（向后兼容）
func convertOFDPathToCanvas(data string, scale float64, offsetX, offsetY float64) string {
	return convertOFDPathToCanvasWithCTM(data, scale, offsetX, offsetY, nil, 0, 0)
}

// parseBoundary 解析边界框（Scanner 方式）
func parseBoundary(boundary string) (x, y, w, h float64) {
	scanner := newNumberScanner(boundary)
	x, _ = scanner.nextFloat()
	y, _ = scanner.nextFloat()
	w, _ = scanner.nextFloat()
	h, _ = scanner.nextFloat()
	return
}

// parseColor 解析颜色（Scanner 方式）
func parseColor(value string) string {
	scanner := newNumberScanner(value)
	r, ok1 := scanner.nextFloat()
	g, ok2 := scanner.nextFloat()
	b, ok3 := scanner.nextFloat()
	if ok1 && ok2 && ok3 {
		return fmt.Sprintf("rgb(%d,%d,%d)", int(r), int(g), int(b))
	}
	return "#000"
}

// parseSignatureXML 解析签章XML，支持多种格式
func (p *Parser) parseSignatureXML(data []byte, debug *DebugInfo) []StampAnnot {
	annots, _ := p.parseSignatureXMLWithAlgo(data, debug)
	return annots
}

// parseSignatureXMLWithAlgo 解析签章XML并返回算法类型
func (p *Parser) parseSignatureXMLWithAlgo(data []byte, debug *DebugInfo) ([]StampAnnot, bool) {
	var annots []StampAnnot
	content := string(data)
	isPrivateAlgo := false

	// 识别签名算法
	methodRe := regexp.MustCompile(`<(?:\w+:)?SignatureMethod[^>]*>([^<]+)</(?:\w+:)?SignatureMethod>`)
	if methodMatch := methodRe.FindStringSubmatch(content); len(methodMatch) >= 2 {
		signatureMethod := strings.TrimSpace(methodMatch[1])
		
		// 判断是否为标准算法
		isStandard := strings.HasPrefix(signatureMethod, "1.2.156") || // 中国 OID
			strings.HasPrefix(signatureMethod, "1.2.840") || // 美国 OID
			strings.HasPrefix(signatureMethod, "1.3.") // 其他标准 OID
		
		if isStandard {
			debug.Stamps = append(debug.Stamps, fmt.Sprintf("标准算法: %s", signatureMethod))
			isPrivateAlgo = false
		} else {
			debug.Stamps = append(debug.Stamps, fmt.Sprintf("私有算法: %s (将使用占位框)", signatureMethod))
			isPrivateAlgo = true
		}
	}

	// 打印 XML 内容的前1000字符用于调试
	preview := content
	if len(preview) > 1000 {
		preview = preview[:1000]
	}
	debug.Stamps = append(debug.Stamps, "sig xml preview: "+preview)

	// 尝试标准格式
	var sigXML SignatureXML
	if err := xml.Unmarshal(data, &sigXML); err == nil {
		annots = append(annots, sigXML.SignedInfo.StampAnnot...)
		annots = append(annots, sigXML.SignedInfo.StampAnnotNS...)
		annots = append(annots, sigXML.SignedInfo.StampAnnotOFD...)
		if len(annots) > 0 {
			debug.Stamps = append(debug.Stamps, fmt.Sprintf("parsed with SignatureXML: %d annots", len(annots)))
			return annots, isPrivateAlgo
		}
	}

	// 尝试带命名空间的格式
	var sigXMLNS SignatureXMLNS
	if err := xml.Unmarshal(data, &sigXMLNS); err == nil {
		annots = append(annots, sigXMLNS.SignedInfo.StampAnnot...)
		annots = append(annots, sigXMLNS.SignedInfo.StampAnnotNS...)
		annots = append(annots, sigXMLNS.SignedInfo.StampAnnotOFD...)
		if len(annots) > 0 {
			debug.Stamps = append(debug.Stamps, fmt.Sprintf("parsed with SignatureXMLNS: %d annots", len(annots)))
			return annots, isPrivateAlgo
		}
	}

	// 使用正则表达式直接提取 StampAnnot
	debug.Stamps = append(debug.Stamps, "trying regex extraction")

	// 匹配 StampAnnot 标签（支持命名空间前缀和自闭合标签）
	re := regexp.MustCompile(`(?i)<(?:\w+:)?StampAnnot[^/>]*(?:/>|>[^<]*</(?:\w+:)?StampAnnot>)`)
	tags := re.FindAllString(content, -1)
	debug.Stamps = append(debug.Stamps, fmt.Sprintf("found %d StampAnnot tags", len(tags)))

	for _, tag := range tags {
		debug.Stamps = append(debug.Stamps, "tag: "+tag)

		// 提取 PageRef
		pageRefRe := regexp.MustCompile(`PageRef\s*=\s*"([^"]*)"`)
		pageRefMatch := pageRefRe.FindStringSubmatch(tag)

		// 提取 Boundary
		boundaryRe := regexp.MustCompile(`Boundary\s*=\s*"([^"]*)"`)
		boundaryMatch := boundaryRe.FindStringSubmatch(tag)

		if len(boundaryMatch) >= 2 {
			pageRef := "0"
			if len(pageRefMatch) >= 2 {
				pageRef = pageRefMatch[1]
			}
			annots = append(annots, StampAnnot{
				PageRef:  pageRef,
				Boundary: boundaryMatch[1],
			})
			debug.Stamps = append(debug.Stamps, fmt.Sprintf("regex found: pageRef=%s, boundary=%s", pageRef, boundaryMatch[1]))
		}
	}

	// 如果还是没找到，尝试查找 ofd:StampAnnot 格式
	if len(annots) == 0 {
		debug.Stamps = append(debug.Stamps, "trying ofd:StampAnnot extraction")
		re2 := regexp.MustCompile(`<ofd:StampAnnot[^>]*>`)
		tags2 := re2.FindAllString(content, -1)
		debug.Stamps = append(debug.Stamps, fmt.Sprintf("found %d ofd:StampAnnot tags", len(tags2)))
		
		for _, tag := range tags2 {
			pageRefRe := regexp.MustCompile(`PageRef\s*=\s*"([^"]*)"`)
			pageRefMatch := pageRefRe.FindStringSubmatch(tag)
			
			boundaryRe := regexp.MustCompile(`Boundary\s*=\s*"([^"]*)"`)
			boundaryMatch := boundaryRe.FindStringSubmatch(tag)
			
			if len(boundaryMatch) >= 2 {
				pageRef := "0"
				if len(pageRefMatch) >= 2 {
					pageRef = pageRefMatch[1]
				}
				annots = append(annots, StampAnnot{
					PageRef:  pageRef,
					Boundary: boundaryMatch[1],
				})
			}
		}
	}

	// 尝试从 SignedInfo 中提取位置信息
	if len(annots) == 0 {
		debug.Stamps = append(debug.Stamps, "trying SignedInfo/Provider extraction")
		
		// 查找所有 Boundary 属性
		providerRe := regexp.MustCompile(`Boundary\s*=\s*"([^"]*)"`)
		providerMatches := providerRe.FindAllStringSubmatch(content, -1)
		
		for _, match := range providerMatches {
			if len(match) >= 2 {
				debug.Stamps = append(debug.Stamps, "found boundary: "+match[1])
				// 检查是否是有效的边界框格式
				parts := strings.Fields(match[1])
				if len(parts) >= 4 {
					annots = append(annots, StampAnnot{
						PageRef:  "0",
						Boundary: match[1],
					})
					debug.Stamps = append(debug.Stamps, "added annot from boundary")
				}
			}
		}
	}

	// 如果还是没找到，尝试查找 PageRef 和 Boundary 的组合
	if len(annots) == 0 {
		debug.Stamps = append(debug.Stamps, "trying PageRef+Boundary extraction")
		
		// 查找包含 PageRef 的元素
		pageRefRe := regexp.MustCompile(`<[^>]*PageRef\s*=\s*"(\d+)"[^>]*Boundary\s*=\s*"([^"]*)"[^>]*>`)
		pageRefMatches := pageRefRe.FindAllStringSubmatch(content, -1)
		
		for _, match := range pageRefMatches {
			if len(match) >= 3 {
				debug.Stamps = append(debug.Stamps, fmt.Sprintf("found PageRef=%s, Boundary=%s", match[1], match[2]))
				annots = append(annots, StampAnnot{
					PageRef:  match[1],
					Boundary: match[2],
				})
			}
		}
		
		// 也尝试反向顺序
		pageRefRe2 := regexp.MustCompile(`<[^>]*Boundary\s*=\s*"([^"]*)"[^>]*PageRef\s*=\s*"(\d+)"[^>]*>`)
		pageRefMatches2 := pageRefRe2.FindAllStringSubmatch(content, -1)
		
		for _, match := range pageRefMatches2 {
			if len(match) >= 3 {
				debug.Stamps = append(debug.Stamps, fmt.Sprintf("found Boundary=%s, PageRef=%s", match[1], match[2]))
				annots = append(annots, StampAnnot{
					PageRef:  match[2],
					Boundary: match[1],
				})
			}
		}
	}

	return annots, isPrivateAlgo
}

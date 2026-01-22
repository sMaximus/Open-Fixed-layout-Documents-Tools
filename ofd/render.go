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
	parts := strings.Fields(box)
	if len(parts) >= 4 {
		w, _ := strconv.ParseFloat(parts[2], 64)
		h, _ := strconv.ParseFloat(parts[3], 64)
		return w, h
	}
	return 210, 297
}


// loadResources 加载资源
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

	for _, file := range p.files {
		lower := strings.ToLower(file)
		if strings.HasSuffix(lower, "publicres.xml") || strings.HasSuffix(lower, "documentres.xml") || (strings.Contains(lower, "res") && strings.HasSuffix(lower, ".xml")) {
			data, err := p.readFile(file)
			if err != nil {
				continue
			}

			// 移除命名空间前缀以便解析
			xmlStr := removeNamespacePrefix(string(data))

			var res Res
			if err := xml.Unmarshal([]byte(xmlStr), &res); err != nil {
				continue
			}

			basePath := path.Dir(file)

			// 加载字体
			for _, font := range res.Fonts {
				p.fonts[font.ID] = font

				// 尝试加载字体文件
				if font.FontFile != "" {
					fontCandidates := []string{
						path.Join(basePath, res.BaseLoc, font.FontFile),
						path.Join(basePath, font.FontFile),
						path.Join(docBase, res.BaseLoc, font.FontFile),
						path.Join(docBase, font.FontFile),
						font.FontFile,
					}

					for _, fontPath := range fontCandidates {
						if fontData, err := p.readFile(fontPath); err == nil {
							p.fontFiles[font.ID] = fontData
							break
						}
					}
				}
			}

			// 加载图片
			for _, media := range res.MultiMedias {
				if media.Type == "Image" {
					candidates := []string{
						path.Join(basePath, res.BaseLoc, media.MediaFile),
						path.Join(basePath, media.MediaFile),
						path.Join(docBase, res.BaseLoc, media.MediaFile),
						path.Join(docBase, media.MediaFile),
						media.MediaFile,
					}

					for _, imgPath := range candidates {
						if imgData, err := p.readFile(imgPath); err == nil {
							p.images[media.ID] = imgData
							break
						}
					}
				}
			}
		}
	}

	// 直接扫描所有字体文件
	for _, file := range p.files {
		lower := strings.ToLower(file)
		if strings.HasSuffix(lower, ".ttf") || strings.HasSuffix(lower, ".otf") || strings.HasSuffix(lower, ".ttc") || strings.HasSuffix(lower, ".woff") || strings.HasSuffix(lower, ".woff2") {
			baseName := path.Base(file)
			ext := path.Ext(baseName)
			id := strings.TrimSuffix(baseName, ext)

			if _, exists := p.fontFiles[id]; !exists {
				if fontData, err := p.readFile(file); err == nil {
					p.fontFiles[id] = fontData
				}
			}
		}
	}

	// 直接扫描所有图片文件
	for _, file := range p.files {
		lower := strings.ToLower(file)
		if strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".gif") || strings.HasSuffix(lower, ".jb2") || strings.HasSuffix(lower, ".ofd") {
			baseName := path.Base(file)
			ext := path.Ext(baseName)
			id := strings.TrimSuffix(baseName, ext)

			if _, exists := p.images[id]; !exists {
				if imgData, err := p.readFile(file); err == nil {
					p.images[id] = imgData
				}
			}
		}
	}

	// 扫描注释目录下的资源文件
	for _, file := range p.files {
		lower := strings.ToLower(file)
		// 查找 Annots 目录下的资源文件
		if strings.Contains(lower, "annot") && strings.HasSuffix(lower, "res.xml") {
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

			// 加载注释中的图片资源
			for _, media := range res.MultiMedias {
				if media.Type == "Image" {
					candidates := []string{
						path.Join(basePath, res.BaseLoc, media.MediaFile),
						path.Join(basePath, media.MediaFile),
						media.MediaFile,
					}

					for _, imgPath := range candidates {
						if imgData, err := p.readFile(imgPath); err == nil {
							p.images[media.ID] = imgData
							break
						}
					}
				}
			}
		}
	}
}

// GetFonts 获取所有字体信息
func (p *Parser) GetFonts() []FontInfo {
	p.loadResources()

	fonts := make([]FontInfo, 0)
	for id, font := range p.fonts {
		info := FontInfo{
			ID:         id,
			FontName:   font.FontName,
			FamilyName: font.FamilyName,
			HasFile:    false,
		}

		// 检查是否有字体文件 - 尝试多种 ID 匹配
		var fontData []byte
		var found bool

		// 1. 直接用 ID 查找
		if fontData, found = p.fontFiles[id]; !found {
			// 2. 尝试用字体名查找
			if fontData, found = p.fontFiles[font.FontName]; !found {
				// 3. 遍历所有字体文件，查找包含字体名的
				for fileID, data := range p.fontFiles {
					if strings.Contains(strings.ToLower(fileID), strings.ToLower(font.FontName)) ||
						strings.Contains(strings.ToLower(font.FontName), strings.ToLower(fileID)) {
						fontData = data
						found = true
						break
					}
				}
			}
		}

		if found && len(fontData) > 0 {
			info.HasFile = true
			// 检测字体类型
			mimeType := "font/ttf"
			if len(fontData) > 4 {
				if fontData[0] == 0x00 && fontData[1] == 0x01 && fontData[2] == 0x00 && fontData[3] == 0x00 {
					mimeType = "font/ttf"
				} else if fontData[0] == 0x4F && fontData[1] == 0x54 && fontData[2] == 0x54 && fontData[3] == 0x4F {
					mimeType = "font/otf"
				} else if fontData[0] == 0x77 && fontData[1] == 0x4F && fontData[2] == 0x46 && fontData[3] == 0x46 {
					mimeType = "font/woff"
				} else if fontData[0] == 0x77 && fontData[1] == 0x4F && fontData[2] == 0x46 && fontData[3] == 0x32 {
					mimeType = "font/woff2"
				}
			}
			info.DataURL = fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(fontData))
		}

		fonts = append(fonts, info)
	}

	return fonts
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
			p.extractPath(result, &pathObj, scale)
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
func (p *Parser) loadPageAnnotImages(result *PageRenderResult, scale float64, pageIndex int, debug *DebugInfo) {
	// 查找当前页面的注释文件
	for _, file := range p.files {
		lower := strings.ToLower(file)
		// 匹配 Annots/Page_X/Annotation.xml 格式
		if (strings.Contains(lower, "annot") || strings.Contains(lower, "annotation")) && 
		   strings.HasSuffix(lower, ".xml") {
			
			// 检查是否是当前页面的注释
			pageMatch := false
			if strings.Contains(lower, fmt.Sprintf("page_%d", pageIndex)) ||
			   strings.Contains(lower, fmt.Sprintf("page_%d", pageIndex+1)) ||
			   strings.Contains(lower, fmt.Sprintf("/page_%d/", pageIndex)) ||
			   strings.Contains(lower, fmt.Sprintf("\\page_%d\\", pageIndex)) {
				pageMatch = true
			}
			
			if !pageMatch {
				continue
			}
			
			data, err := p.readFile(file)
			if err != nil {
				continue
			}
			
			// 移除命名空间前缀
			xmlStr := removeNamespacePrefix(string(data))
			
			// 解析注释文件
			var pageAnnot PageAnnot
			if err := xml.Unmarshal([]byte(xmlStr), &pageAnnot); err != nil {
				debug.Stamps = append(debug.Stamps, fmt.Sprintf("parse annot error: %v", err))
				continue
			}
			
			annotDir := path.Dir(file)
			
			// 处理每个注释
			for _, annot := range pageAnnot.Annots {
				// 解析 Appearance 中的 Boundary
				ax, ay, _, _ := parseBoundary(annot.Appearance.Boundary)
				
				// 处理 PageBlock 中的图片
				for _, block := range annot.Appearance.PageBlocks {
					for _, img := range block.ImageObjects {
						// 查找图片资源
						imgData, ok := p.images[img.ResourceID]
						if !ok {
							// 尝试从注释目录加载
							imgData = p.loadAnnotImage(annotDir, img.ResourceID, debug)
							if imgData == nil {
								debug.MissingIDs = append(debug.MissingIDs, img.ResourceID)
								continue
							}
						}
						
						// 解析图片边界
						ix, iy, iw, ih := parseBoundary(img.Boundary)
						
						// 计算最终位置（Appearance 位置 + 图片相对位置）
						finalX := (ax + ix) * scale
						finalY := (ay + iy) * scale
						finalW := iw * scale
						finalH := ih * scale
						
						// 检测图片类型
						mimeType := "image/png"
						if len(imgData) > 2 && imgData[0] == 0xFF && imgData[1] == 0xD8 {
							mimeType = "image/jpeg"
						}
						
						result.CanvasData.Images = append(result.CanvasData.Images, ImageData{
							DataURL: fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(imgData)),
							X:       finalX,
							Y:       finalY,
							Width:   finalW,
							Height:  finalH,
						})
						
						debug.Stamps = append(debug.Stamps, fmt.Sprintf("annot image added: id=%s, pos=(%.2f,%.2f), size=(%.2f,%.2f)", 
							img.ResourceID, finalX, finalY, finalW, finalH))
					}
				}
			}
		}
	}
}

// loadAnnotImage 从注释目录加载图片
func (p *Parser) loadAnnotImage(annotDir string, resourceID string, debug *DebugInfo) []byte {
	// 尝试多种路径
	candidates := []string{
		path.Join(annotDir, "Res", resourceID+".png"),
		path.Join(annotDir, "Res", resourceID+".jpg"),
		path.Join(annotDir, "Res", resourceID+".jpeg"),
		path.Join(annotDir, resourceID+".png"),
		path.Join(annotDir, resourceID+".jpg"),
	}
	
	for _, candidate := range candidates {
		if imgData, err := p.readFile(candidate); err == nil {
			debug.Stamps = append(debug.Stamps, "loaded annot image: "+candidate)
			p.images[resourceID] = imgData
			return imgData
		}
	}
	
	// 扫描注释目录下的所有图片文件
	for _, file := range p.files {
		if strings.HasPrefix(file, annotDir) {
			lower := strings.ToLower(file)
			if strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") {
				baseName := path.Base(file)
				ext := path.Ext(baseName)
				id := strings.TrimSuffix(baseName, ext)
				
				if id == resourceID {
					if imgData, err := p.readFile(file); err == nil {
						debug.Stamps = append(debug.Stamps, "loaded annot image by scan: "+file)
						p.images[resourceID] = imgData
						return imgData
					}
				}
			}
		}
	}
	
	return nil
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
	imgData, ok := p.images[img.ResourceID]
	if !ok {
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
func (p *Parser) extractPath(result *PageRenderResult, pathObj *PathObject, scale float64) {
	if pathObj.AbbreviatedData == "" {
		return
	}

	strokeColor := ""
	fillColor := ""
	var gradient *GradientData
	
	// 处理描边颜色
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
	
	// 处理填充颜色或渐变
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
		}
	}

	// 解析边界框（Boundary 定义了对象在页面上的位置）
	bx, by, _, _ := parseBoundary(pathObj.Boundary)
	
	// 默认线宽
	lineWidth := pathObj.LineWidth
	if lineWidth == 0 {
		lineWidth = 0.353 // 默认 1pt = 0.353mm
	}
	
	// 如果有 CTM 变换矩阵，lineWidth 需要乘以 CTM 的缩放因子
	// CTM 格式: [a, b, c, d, e, f]，其中 a 是 x 方向缩放因子
	// OFD 中 LineWidth 是在对象坐标系中定义的，需要乘以 CTM 缩放来得到页面坐标系中的线宽
	if len(ctm) >= 1 && ctm[0] > 0 {
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
	result.CanvasData.Paths = append(result.CanvasData.Paths, PathData{
		Commands:    convertOFDPathToCanvasWithCTM(pathObj.AbbreviatedData, scale, bx, by, ctm),
		FillColor:   fillColor,
		StrokeColor: strokeColor,
		LineWidth:   lineWidth * scale,
		LineJoin:    lineJoin,
		LineCap:     lineCap,
		Gradient:    gradient,
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

	// OFD 中 Size 是字体大小（单位 mm），需要转换为像素
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
				if cgt.Glyphs != "" {
					glyphIDStrs := strings.Fields(cgt.Glyphs)
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

		// TextCode 的 X, Y 是相对于 Boundary 左上角的偏移（单位 mm）
		tcX := (bx + tc.X) * scale
		tcY := (by + tc.Y) * scale

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

// parseCTM 解析 CTM 变换矩阵
// 格式: "a b c d e f" 或 "a b c d"
func parseCTM(ctmStr string) []float64 {
	parts := strings.Fields(ctmStr)
	result := make([]float64, 0, 6)
	for _, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err == nil {
			result = append(result, v)
		}
	}
	// 确保至少有 4 个值 (a, b, c, d)
	if len(result) < 4 {
		return nil
	}
	return result
}

// parseDeltas 解析 DeltaX/DeltaY 字符串
// 支持格式: "1 2 3" 或 "g 5 3" (g表示重复 - g count value)
func parseDeltas(deltaStr string) []float64 {
	parts := strings.Fields(deltaStr)
	result := make([]float64, 0, len(parts))

	i := 0
	for i < len(parts) {
		if parts[i] == "g" && i+2 < len(parts) {
			// "g count value" 格式：重复 count 次 value
			count, err1 := strconv.Atoi(parts[i+1])
			val, err2 := strconv.ParseFloat(parts[i+2], 64)
			if err1 == nil && err2 == nil {
				for j := 0; j < count; j++ {
					result = append(result, val)
				}
			}
			i += 3
		} else {
			val, err := strconv.ParseFloat(parts[i], 64)
			if err == nil {
				result = append(result, val)
			}
			i++
		}
	}
	return result
}

// convertOFDPathToCanvasWithCTM 转换OFD路径命令为Canvas命令JSON，应用CTM变换
func convertOFDPathToCanvasWithCTM(data string, scale float64, offsetX, offsetY float64, ctm []float64) string {
	var commands []map[string]interface{}

	// CTM 变换逻辑 (保持不变)
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

	// 1. 使用正则提取所有 Token (指令字母 或 浮点数)
	// 这个正则解决了 "粘连" 问题 (如 100-20 或 L100)
	// 匹配：单个字母 [a-zA-Z]  OR  数字 (支持负号、小数) [-+]?[0-9]*\.?[0-9]+
	re := regexp.MustCompile(`([a-zA-Z])|([-+]?[0-9]*\.?[0-9]+)`)
	matches := re.FindAllString(data, -1)

	i := 0
	length := len(matches)
	var currentCmd string // 记录当前命令，处理隐含重复

	for i < length {
		token := matches[i]

		// 判断是否是命令字母
		firstChar := token[0]
		if (firstChar >= 'A' && firstChar <= 'Z') || (firstChar >= 'a' && firstChar <= 'z') {
			currentCmd = token
			i++
		}
		// 如果不是字母，则沿用上一个 currentCmd (隐含命令逻辑)

		switch currentCmd {
		case "S", "M": // MoveTo
			if i+1 < length {
				x, _ := strconv.ParseFloat(matches[i], 64)
				y, _ := strconv.ParseFloat(matches[i+1], 64)
				tx, ty := transformPoint(x, y)
				commands = append(commands, map[string]interface{}{
					"cmd": "M", "x": tx, "y": ty,
				})
				i += 2
				// M 后面的数字如果还有，通常视为 L (LineTo)
				currentCmd = "L" 
			} else {
				i++
			}
		case "L": // LineTo
			if i+1 < length {
				x, _ := strconv.ParseFloat(matches[i], 64)
				y, _ := strconv.ParseFloat(matches[i+1], 64)
				tx, ty := transformPoint(x, y)
				commands = append(commands, map[string]interface{}{
					"cmd": "L", "x": tx, "y": ty,
				})
				i += 2
			} else {
				i++
			}
		case "B": // Bezier (OFD 的 B 是贝塞尔)
			if i+5 < length {
				x1, _ := strconv.ParseFloat(matches[i], 64)
				y1, _ := strconv.ParseFloat(matches[i+1], 64)
				x2, _ := strconv.ParseFloat(matches[i+2], 64)
				y2, _ := strconv.ParseFloat(matches[i+3], 64)
				x3, _ := strconv.ParseFloat(matches[i+4], 64)
				y3, _ := strconv.ParseFloat(matches[i+5], 64)
				
				tx1, ty1 := transformPoint(x1, y1)
				tx2, ty2 := transformPoint(x2, y2)
				tx3, ty3 := transformPoint(x3, y3)
				
				// 对应 Canvas 的 bezierCurveTo (cmd: C)
				commands = append(commands, map[string]interface{}{
					"cmd": "C",
					"x1": tx1, "y1": ty1,
					"x2": tx2, "y2": ty2,
					"x":  tx3, "y":  ty3,
				})
				i += 6
			} else {
				i++
			}
		case "Q": // Quadratic
			if i+3 < length {
				x1, _ := strconv.ParseFloat(matches[i], 64)
				y1, _ := strconv.ParseFloat(matches[i+1], 64)
				x2, _ := strconv.ParseFloat(matches[i+2], 64)
				y2, _ := strconv.ParseFloat(matches[i+3], 64)
				
				tx1, ty1 := transformPoint(x1, y1)
				tx2, ty2 := transformPoint(x2, y2)
				
				commands = append(commands, map[string]interface{}{
					"cmd": "Q",
					"x1": tx1, "y1": ty1,
					"x":  tx2, "y":  ty2,
				})
				i += 4
			} else {
				i++
			}
		case "C": // Close (OFD 的 C 是闭合)
			// 修正：OFD 的 C 不需要参数，直接映射为 Canvas 的 Z
			commands = append(commands, map[string]interface{}{"cmd": "Z"})
			// Close 指令后面通常不再跟坐标，直到新的 M 出现
			// 即使 i 不增加也没关系，下一次循环会读到新的指令
		case "A": // Arc
		    // Arc 参数较多，暂时跳过参数防止解析错位
		    // 严谨做法需要消耗掉 A 的参数 (rx ry rot large sweep x y) 共7个
			if i+6 < length {
			    i += 7
			} else {
			    i++
			}
		default:
			i++
		}
	}

	jsonData, _ := json.Marshal(commands)
	return string(jsonData)
}
// convertOFDPathToCanvas 转换OFD路径命令为Canvas命令JSON（向后兼容）
func convertOFDPathToCanvas(data string, scale float64, offsetX, offsetY float64) string {
	return convertOFDPathToCanvasWithCTM(data, scale, offsetX, offsetY, nil)
}

func parseBoundary(boundary string) (x, y, w, h float64) {
	parts := strings.Fields(boundary)
	if len(parts) >= 4 {
		x, _ = strconv.ParseFloat(parts[0], 64)
		y, _ = strconv.ParseFloat(parts[1], 64)
		w, _ = strconv.ParseFloat(parts[2], 64)
		h, _ = strconv.ParseFloat(parts[3], 64)
	}
	return
}

func parseColor(value string) string {
	parts := strings.Fields(value)
	if len(parts) >= 3 {
		r, _ := strconv.Atoi(parts[0])
		g, _ := strconv.Atoi(parts[1])
		b, _ := strconv.Atoi(parts[2])
		return fmt.Sprintf("rgb(%d,%d,%d)", r, g, b)
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

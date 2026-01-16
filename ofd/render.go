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
	Commands    string  `json:"commands"`
	FillColor   string  `json:"fillColor,omitempty"`
	StrokeColor string  `json:"strokeColor,omitempty"`
	LineWidth   float64 `json:"lineWidth"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
}

// ImageData 图片数据
type ImageData struct {
	DataURL string  `json:"dataURL"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Width   float64 `json:"width"`
	Height  float64 `json:"height"`
}

// TextItem 文本项
type TextItem struct {
	Text       string    `json:"text"`
	X          float64   `json:"x"`
	Y          float64   `json:"y"`
	FontSize   float64   `json:"fontSize"`
	FontFamily string    `json:"fontFamily"`
	FontID     string    `json:"fontID"`
	Color      string    `json:"color"`
	CTM        []float64 `json:"ctm,omitempty"` // 变换矩阵 [a, b, c, d, e, f]
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

	// 加载签章/印章
	p.loadStamps(result, scale, pageIndex, debug)

	result.Debug = debug
}

// loadStamps 加载签章
func (p *Parser) loadStamps(result *PageRenderResult, scale float64, pageIndex int, debug *DebugInfo) {
	// 收集所有印章注释
	var allAnnots []struct {
		annot  StampAnnot
		sigDir string
		sigID  string
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

			var sigs Signatures
			if err := xml.Unmarshal(data, &sigs); err != nil {
				debug.Stamps = append(debug.Stamps, "parse error: "+err.Error())
				continue
			}

			basePath := path.Dir(file)
			for _, sig := range sigs.Signature {
				debug.Stamps = append(debug.Stamps, "sig: "+sig.ID+" loc: "+sig.BaseLoc)

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

				// 尝试解析签章 XML（支持多种格式）
				annots := p.parseSignatureXML(sigData, debug)
				sigDir := path.Dir(sigPath)

				for _, annot := range annots {
					debug.Stamps = append(debug.Stamps, fmt.Sprintf("annot: pageRef=%s, boundary=%s", annot.PageRef, annot.Boundary))
					allAnnots = append(allAnnots, struct {
						annot  StampAnnot
						sigDir string
						sigID  string
					}{annot, sigDir, sig.ID})
				}
			}
		}
	}

	// 2. 查找页面注释文件 (Annots/Page_X/Annotation.xml)
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
							annot  StampAnnot
							sigDir string
							sigID  string
						}{
							StampAnnot{
								ID:       annot.ID,
								PageRef:  fmt.Sprintf("%d", pageIndex),
								Boundary: annot.Appearance.Boundary,
							},
							path.Dir(file),
							annot.ID,
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
							annot  StampAnnot
							sigDir string
							sigID  string
						}{
							StampAnnot{
								PageRef:  fmt.Sprintf("%d", pageIndex),
								Boundary: boundaryMatch[1],
							},
							path.Dir(file),
							"",
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

	// 获取当前页面ID
	pageID := ""
	if p.document != nil && pageIndex < len(p.document.Pages.Page) {
		pageID = p.document.Pages.Page[pageIndex].ID
	}
	debug.Stamps = append(debug.Stamps, fmt.Sprintf("pageID=%s, pageIndex=%d, totalAnnots=%d", pageID, pageIndex, len(allAnnots)))

	// 查找当前页面的印章
	for _, item := range allAnnots {
		// 检查是否属于当前页面
		matched := item.annot.PageRef == pageID ||
			item.annot.PageRef == fmt.Sprintf("%d", pageIndex) ||
			item.annot.PageRef == fmt.Sprintf("%d", pageIndex+1)
		
		debug.Stamps = append(debug.Stamps, fmt.Sprintf("checking annot: pageRef=%s, matched=%v", item.annot.PageRef, matched))
		
		if matched {
			p.loadSealImage(result, scale, item.sigDir, item.sigID, &item.annot, debug)
		}
	}

	// 4. 如果没有找到印章，尝试直接加载所有印章文件
	if len(result.CanvasData.Images) == 0 {
		debug.Stamps = append(debug.Stamps, "no stamps found, trying direct load from sig dirs")
		
		// 从签章目录中加载印章
		for _, file := range p.files {
			lower := strings.ToLower(file)
			if strings.Contains(lower, "sign") && 
			   (strings.HasSuffix(lower, ".esl") || strings.HasSuffix(lower, ".png") || 
			    strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg")) {
				
				imgData, err := p.readFile(file)
				if err != nil {
					continue
				}
				
				debug.Stamps = append(debug.Stamps, "loading from sig dir: "+file)
				
				// 如果是 ESL 文件，提取图片
				if strings.HasSuffix(lower, ".esl") {
					extracted := p.extractESLImage(imgData)
					if extracted != nil {
						imgData = extracted
					} else {
						continue
					}
				}
				
				// 检测图片类型
				mimeType := "image/png"
				if len(imgData) > 2 && imgData[0] == 0xFF && imgData[1] == 0xD8 {
					mimeType = "image/jpeg"
				}
				
				// 使用默认位置
				result.CanvasData.Images = append(result.CanvasData.Images, ImageData{
					DataURL: fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(imgData)),
					X:       20 * scale,
					Y:       20 * scale,
					Width:   40 * scale,
					Height:  40 * scale,
				})
				debug.Stamps = append(debug.Stamps, "seal added from sig dir")
			}
		}
	}
	
	// 5. 最后尝试加载所有印章文件
	if len(result.CanvasData.Images) == 0 {
		p.loadAllSeals(result, scale, pageIndex, debug)
	}
}

// loadAllSeals 直接加载所有印章文件
func (p *Parser) loadAllSeals(result *PageRenderResult, scale float64, pageIndex int, debug *DebugInfo) {
	// 查找所有印章文件并尝试加载
	for _, file := range p.files {
		lower := strings.ToLower(file)
		if (strings.Contains(lower, "seal") || strings.Contains(lower, "stamp")) &&
			(strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") ||
				strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".esl")) {

			imgData, err := p.readFile(file)
			if err != nil {
				continue
			}

			debug.Stamps = append(debug.Stamps, "loading seal: "+file)

			// 如果是 ESL 文件，提取图片
			if strings.HasSuffix(lower, ".esl") {
				extracted := p.extractESLImage(imgData)
				if extracted != nil {
					imgData = extracted
				} else {
					continue
				}
			}

			// 检测图片类型
			mimeType := "image/png"
			if len(imgData) > 2 && imgData[0] == 0xFF && imgData[1] == 0xD8 {
				mimeType = "image/jpeg"
			}

			// 使用默认位置（左上角）
			result.CanvasData.Images = append(result.CanvasData.Images, ImageData{
				DataURL: fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(imgData)),
				X:       20 * scale,
				Y:       20 * scale,
				Width:   40 * scale,
				Height:  40 * scale,
			})
			debug.Stamps = append(debug.Stamps, "seal added (default position)")
		}
	}
}// loadSealImage 加载印章图片
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
		})
		debug.Stamps = append(debug.Stamps, fmt.Sprintf("seal added at (%f,%f) size (%f,%f)", x*scale, y*scale, w*scale, h*scale))
		return
	}

	debug.Stamps = append(debug.Stamps, "no seal image found")
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

	result.CanvasData.Images = append(result.CanvasData.Images, ImageData{
		DataURL: fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(imgData)),
		X:       x * scale,
		Y:       y * scale,
		Width:   w * scale,
		Height:  h * scale,
	})
}

// extractPath 提取路径数据
func (p *Parser) extractPath(result *PageRenderResult, pathObj *PathObject, scale float64) {
	if pathObj.AbbreviatedData == "" {
		return
	}

	strokeColor := ""
	fillColor := ""
	
	// 处理描边颜色
	if pathObj.StrokeColor != nil && pathObj.StrokeColor.Value != "" {
		strokeColor = parseColor(pathObj.StrokeColor.Value)
	} else if pathObj.Stroke || pathObj.LineWidth > 0 {
		// 如果有描边属性但没有颜色，使用默认黑色
		strokeColor = "#000"
	}
	
	// 处理填充颜色
	if pathObj.FillColor != nil && pathObj.FillColor.Value != "" {
		fillColor = parseColor(pathObj.FillColor.Value)
	}

	// 解析边界框
	bx, by, _, _ := parseBoundary(pathObj.Boundary)
	
	// 默认线宽
	lineWidth := pathObj.LineWidth
	if lineWidth == 0 {
		lineWidth = 0.353 // 默认 1pt = 0.353mm
	}

	// OFD 路径坐标是相对于边界框的，需要加上边界框偏移
	result.CanvasData.Paths = append(result.CanvasData.Paths, PathData{
		Commands:    convertOFDPathToCanvas(pathObj.AbbreviatedData, scale, bx, by),
		FillColor:   fillColor,
		StrokeColor: strokeColor,
		LineWidth:   lineWidth * scale,
		X:           0,
		Y:           0,
	})
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

	// 处理颜色 - 优先使用 FillColor，如果是描边文字则使用 StrokeColor
	color := "#000"
	if text.FillColor != nil && text.FillColor.Value != "" {
		color = parseColor(text.FillColor.Value)
	} else if text.StrokeColor != nil && text.StrokeColor.Value != "" {
		color = parseColor(text.StrokeColor.Value)
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
		content := strings.TrimSpace(tc.Content)
		if content == "" {
			continue
		}

		// TextCode 的 X, Y 是相对于 Boundary 左上角的偏移（单位 mm）
		// 最终位置 = Boundary位置 + TextCode偏移
		tcX := (bx + tc.X) * scale
		tcY := (by + tc.Y) * scale

		// 调试输出
		if debug != nil && len(content) <= 6 {
			debug.TextDebug = append(debug.TextDebug,
				fmt.Sprintf("text='%s' boundary=(%.2f,%.2f) tc=(%.2f,%.2f,%.2f,%.2f) final=(%.2f,%.2f)",
					content, bx, by, tc.X, tc.Y,by,bx, tcX, tcY))
		}

		chars := []rune(content)

		// 解析 DeltaX - 字符间距数组（单位 mm）
		var deltaX []float64
		if tc.DeltaX != "" {
			deltaX = parseDeltas(tc.DeltaX)
		}		// 如果有多个字符且有 DeltaX，需要逐字符定位
		if len(chars) > 1 && len(deltaX) > 0 {
			currentX := tcX

			for i, char := range chars {
				result.TextLayer = append(result.TextLayer, TextItem{
					Text:       string(char),
					X:          currentX,
					Y:          tcY,
					FontSize:   fontSize,
					FontFamily: fontFamily,
					FontID:     fontID,
					Color:      color,
					CTM:        ctm,
				})

				// 计算下一个字符的位置
				// DeltaX[i] 是第 i 个字符到第 i+1 个字符的偏移（单位 mm）
				if i < len(deltaX) {
					// 如果有 CTM，需要考虑 X 轴缩放
					delta := deltaX[i] * scale
					if len(ctm) >= 1 && ctm[0] != 0 {
						delta = delta * ctm[0]
					}
					currentX += delta
				}
			}
		} else {
			// 没有 DeltaX 或只有一个字符，整体输出
			result.TextLayer = append(result.TextLayer, TextItem{
				Text:       content,
				X:          tcX,
				Y:          tcY,
				FontSize:   fontSize,
				FontFamily: fontFamily,
				FontID:     fontID,
				Color:      color,
				CTM:        ctm,
			})
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

// convertOFDPathToCanvas 转换OFD路径命令为Canvas命令JSON
func convertOFDPathToCanvas(data string, scale float64, offsetX, offsetY float64) string {
	var commands []map[string]interface{}
	
	// 简化解析：按空格分割
	parts := strings.Fields(data)
	i := 0
	
	for i < len(parts) {
		cmd := parts[i]
		switch cmd {
		case "S", "M": // Start/Move
			if i+2 < len(parts) {
				x, _ := strconv.ParseFloat(parts[i+1], 64)
				y, _ := strconv.ParseFloat(parts[i+2], 64)
				commands = append(commands, map[string]interface{}{
					"cmd": "M", "x": (offsetX + x) * scale, "y": (offsetY + y) * scale,
				})
				i += 3
			} else {
				i++
			}
		case "L": // Line
			if i+2 < len(parts) {
				x, _ := strconv.ParseFloat(parts[i+1], 64)
				y, _ := strconv.ParseFloat(parts[i+2], 64)
				commands = append(commands, map[string]interface{}{
					"cmd": "L", "x": (offsetX + x) * scale, "y": (offsetY + y) * scale,
				})
				i += 3
			} else {
				i++
			}
		case "B", "C": // Bezier curve
			if i+6 < len(parts) {
				x1, _ := strconv.ParseFloat(parts[i+1], 64)
				y1, _ := strconv.ParseFloat(parts[i+2], 64)
				x2, _ := strconv.ParseFloat(parts[i+3], 64)
				y2, _ := strconv.ParseFloat(parts[i+4], 64)
				x3, _ := strconv.ParseFloat(parts[i+5], 64)
				y3, _ := strconv.ParseFloat(parts[i+6], 64)
				commands = append(commands, map[string]interface{}{
					"cmd": "C",
					"x1": (offsetX + x1) * scale, "y1": (offsetY + y1) * scale,
					"x2": (offsetX + x2) * scale, "y2": (offsetY + y2) * scale,
					"x":  (offsetX + x3) * scale, "y":  (offsetY + y3) * scale,
				})
				i += 7
			} else {
				i++
			}
		case "Q": // Quadratic curve
			if i+4 < len(parts) {
				x1, _ := strconv.ParseFloat(parts[i+1], 64)
				y1, _ := strconv.ParseFloat(parts[i+2], 64)
				x2, _ := strconv.ParseFloat(parts[i+3], 64)
				y2, _ := strconv.ParseFloat(parts[i+4], 64)
				commands = append(commands, map[string]interface{}{
					"cmd": "Q",
					"x1": (offsetX + x1) * scale, "y1": (offsetY + y1) * scale,
					"x":  (offsetX + x2) * scale, "y":  (offsetY + y2) * scale,
				})
				i += 5
			} else {
				i++
			}
		case "A": // Arc - 简化处理
			i += 6
		case "Z": // Close
			commands = append(commands, map[string]interface{}{"cmd": "Z"})
			i++
		default:
			i++
		}
	}
	
	jsonData, _ := json.Marshal(commands)
	return string(jsonData)
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
	var annots []StampAnnot
	content := string(data)

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
			return annots
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
			return annots
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

	return annots
}

package ofd

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"path"
	"strings"
)

// RenderPageSVG 渲染指定页面为 SVG
// 策略：先用已有的 RenderPage 获取路径/图片数据，
// 然后将文字用 freetype 渲染为图片，最终组装为 SVG。
func (p *Parser) RenderPageSVG(pageIndex int) *PageRenderResult {
	result := &PageRenderResult{
		PageIndex: pageIndex,
	}

	if p.document == nil || pageIndex >= len(p.document.Pages.Page) {
		result.Error = "页面不存在"
		return result
	}

	pagePath := p.GetPagePath(pageIndex)
	if pagePath == "" {
		result.Error = "无法获取页面路径"
		return result
	}

	pageData, err := p.readFile(pagePath)
	if err != nil {
		result.Error = "读取页面失败: " + err.Error()
		return result
	}

	pageXML := removeNamespacePrefix(string(pageData))
	var page Page
	if err := xml.Unmarshal([]byte(pageXML), &page); err != nil {
		result.Error = "解析页面失败: " + err.Error()
		return result
	}

	width, height := p.getPageSize(&page)
	result.Width = width
	result.Height = height

	p.loadResources()
	p.preloadFontFiles()

	// 收集 glyph 映射（用于字体修复）
	pageMappings := p.collectPageGlyphMappings(pageIndex)
	if p.glyphMappingsCache == nil {
		p.glyphMappingsCache = make(map[string][]GlyphMapping)
		p.glyphMappingsSeen = make(map[string]map[uint32]bool)
	}
	for fontID, mappings := range pageMappings {
		if p.glyphMappingsSeen[fontID] == nil {
			p.glyphMappingsSeen[fontID] = make(map[uint32]bool)
		}
		for _, m := range mappings {
			key := uint32(m.Unicode)<<16 | uint32(m.GlyphID)
			if !p.glyphMappingsSeen[fontID][key] {
				p.glyphMappingsSeen[fontID][key] = true
				p.glyphMappingsCache[fontID] = append(p.glyphMappingsCache[fontID], m)
			}
		}
	}

	scale := 3.78
	pxW := width * scale
	pxH := height * scale

	var svgParts []string
	var textOverlay []TextOverlayItem

	debug := &DebugInfo{
		ImageIDs:     make([]string, 0),
		RequestedIDs: make([]string, 0),
		MissingIDs:   make([]string, 0),
		Stamps:       make([]string, 0),
		Files:        p.files,
		TextDebug:    make([]string, 0),
	}

	// 白色背景
	svgParts = append(svgParts, fmt.Sprintf(
		`<rect width="%.2f" height="%.2f" fill="white"/>`, pxW, pxH))

	// 渲染模板层
	p.renderTemplateSVG(&svgParts, &textOverlay, &page, scale, width, height, debug)

	// 获取页面层
	layers := page.Content.Layer
	if len(layers) == 0 {
		layers = page.ContentNS.Layer
	}
	if len(layers) == 0 {
		layers = page.Layer
	}

	for _, layer := range layers {
		for _, obj := range orderedLayerObjects(&layer) {
			switch obj.objType {
			case layerObjectPath:
				s := p.pathObjectToSVG(obj.path, scale, width, height)
				if s != "" {
					svgParts = append(svgParts, s)
				}
			case layerObjectImage:
				s := p.imageObjectToSVG(obj.image, scale, debug)
				if s != "" {
					svgParts = append(svgParts, s)
				}
			case layerObjectText:
				textImgs, textOvl := p.renderTextObject(obj.text, scale, debug)
				for _, ti := range textImgs {
					s := textRenderResultToSVG(ti)
					if s != "" {
						svgParts = append(svgParts, s)
					}
				}
				textOverlay = append(textOverlay, textOvl...)
			}
		}
	}

	// 注释图片
	p.loadPageAnnotSVG(&svgParts, scale, pageIndex, debug)

	// 印章
	p.loadStampsSVG(result, &svgParts, scale, pageIndex, debug)

	// 组装 SVG
	debug.TextDebug = append(debug.TextDebug, fmt.Sprintf(
		"SVG summary: layers=%d, svgParts=%d, textOverlay=%d, fontFiles=%d, fonts=%d",
		len(layers), len(svgParts), len(textOverlay), len(p.fontFiles), len(p.fonts)))

	svg := fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" `+
			`width="%.2f" height="%.2f" viewBox="0 0 %.2f %.2f">`+
			"\n%s\n</svg>",
		pxW, pxH, pxW, pxH, strings.Join(svgParts, "\n"))

	result.SVG = svg
	result.TextOverlay = textOverlay
	result.Debug = debug
	return result
}

// textRenderResultToSVG 将 TextRenderResult 转换为 SVG 元素
func textRenderResultToSVG(ti TextRenderResult) string {
	if ti.DataURL != "" {
		// freetype 渲染的图片
		return fmt.Sprintf(
			`<image href="%s" x="%.2f" y="%.2f" width="%.2f" height="%.2f"/>`,
			ti.DataURL, ti.X, ti.Y, ti.Width, ti.Height)
	}
	if ti.Text != "" {
		// SVG text 回退（系统字体）
		weight := ""
		if ti.Weight >= 700 {
			weight = ` font-weight="bold"`
		}
		italic := ""
		if ti.Italic {
			italic = ` font-style="italic"`
		}
		// 描边属性
		strokeAttr := ""
		fillAttr := fmt.Sprintf(`fill="%s"`, ti.Color)
		if ti.Stroke && ti.StrokeColor != "" {
			strokeAttr = fmt.Sprintf(` stroke="%s" stroke-width="%.2f"`, ti.StrokeColor, ti.StrokeWidth)
			if !ti.Fill {
				fillAttr = `fill="none"`
			}
		}
		// 转义 XML 特殊字符
		escapedText := xmlEscape(ti.Text)
		return fmt.Sprintf(
			`<text x="%.2f" y="%.2f" font-size="%.2f" font-family="%s" %s%s%s%s>%s</text>`,
			ti.X, ti.Y, ti.FontSize, ti.FontFamily, fillAttr, weight, italic, strokeAttr, escapedText)
	}
	return ""
}

// xmlEscape 转义 XML 特殊字符
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}


// preloadFontFiles 预加载所有字体文件到 fontFiles 缓存
func (p *Parser) preloadFontFiles() {
	loaded := 0
	skipped := 0
	failed := 0
	for id, f := range p.fonts {
		if f.FontFile == "" {
			skipped++
			continue
		}
		if _, ok := p.fontFiles[id]; ok {
			loaded++ // 已加载
			continue
		}
		if fontData, err := p.readFile(f.FontFile); err == nil && len(fontData) > 0 {
			var mappings []GlyphMapping
			if p.glyphMappingsCache != nil {
				mappings = p.glyphMappingsCache[id]
			}
			fontData = SanitizeFontWithMappings(fontData, mappings)
			p.fontFiles[id] = fontData
			loaded++
		} else {
			failed++
		}
	}
	fmt.Printf("[preloadFontFiles] total=%d, loaded=%d, skipped(no file)=%d, failed=%d\n",
		len(p.fonts), loaded, skipped, failed)
}

// renderTemplateSVG 渲染模板层为 SVG 元素
func (p *Parser) renderTemplateSVG(svgParts *[]string, textOverlay *[]TextOverlayItem, page *Page, scale, pageW, pageH float64, debug *DebugInfo) {
	if len(page.Template) == 0 || p.document == nil {
		return
	}

	tplPaths := make(map[string]string)
	docBase := ""
	if p.ofd != nil && len(p.ofd.DocBody) > 0 {
		docRoot := strings.TrimPrefix(p.ofd.DocBody[0].DocRoot, "/")
		docBase = path.Dir(docRoot)
	}
	for _, tpl := range p.document.CommonData.TemplatePage {
		tplLoc := strings.TrimPrefix(tpl.BaseLoc, "/")
		tplPaths[tpl.ID] = path.Join(docBase, tplLoc)
	}

	for _, tplRef := range page.Template {
		tplPath, ok := tplPaths[tplRef.TemplateID]
		if !ok {
			continue
		}
		tplData, err := p.readFile(tplPath)
		if err != nil {
			continue
		}
		tplXML := removeNamespacePrefix(string(tplData))
		var tplPage Page
		if err := xml.Unmarshal([]byte(tplXML), &tplPage); err != nil {
			continue
		}

		tplLayers := tplPage.Content.Layer
		if len(tplLayers) == 0 {
			tplLayers = tplPage.ContentNS.Layer
		}
		if len(tplLayers) == 0 {
			tplLayers = tplPage.Layer
		}

		for _, layer := range tplLayers {
			for _, obj := range orderedLayerObjects(&layer) {
				switch obj.objType {
				case layerObjectPath:
					s := p.pathObjectToSVG(obj.path, scale, pageW, pageH)
					if s != "" {
						*svgParts = append(*svgParts, s)
					}
				case layerObjectImage:
					s := p.imageObjectToSVG(obj.image, scale, debug)
					if s != "" {
						*svgParts = append(*svgParts, s)
					}
				case layerObjectText:
					textImgs, textOvl := p.renderTextObject(obj.text, scale, debug)
					for _, ti := range textImgs {
						s := textRenderResultToSVG(ti)
						if s != "" {
							*svgParts = append(*svgParts, s)
						}
					}
					*textOverlay = append(*textOverlay, textOvl...)
				}
			}
		}
	}
}

// svgGradientCounter 用于生成唯一的 SVG 渐变 ID
var svgGradientCounter int

// pathObjectToSVG 将 PathObject 转换为 SVG path 元素
// 复用已有的路径解析逻辑
func (p *Parser) pathObjectToSVG(pathObj *PathObject, scale, pageW, pageH float64) string {
	if pathObj.Visible != nil && !*pathObj.Visible {
		return ""
	}
	if pathObj.AbbreviatedData == "" {
		return ""
	}

	bx, by, _, _ := parseBoundary(pathObj.Boundary)

	// 解析 CTM
	var ctm []float64
	if pathObj.CTM != "" {
		ctm = parseCTM(pathObj.CTM)
	}

	// 使用已有的路径转换函数获取 JSON 命令
	cmdJSON := convertOFDPathToCanvasWithCTM(pathObj.AbbreviatedData, scale, bx, by, ctm, pageW*scale, pageH*scale)

	// 解析 JSON 命令数组，转换为 SVG path d 属性
	var cmds []interface{}
	if err := json.Unmarshal([]byte(cmdJSON), &cmds); err != nil {
		return ""
	}

	var d strings.Builder
	for _, c := range cmds {
		cmdMap, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		cmdType, _ := cmdMap["cmd"].(string)
		switch cmdType {
		case "M":
			x, _ := cmdMap["x"].(float64)
			y, _ := cmdMap["y"].(float64)
			d.WriteString(fmt.Sprintf("M%.4f,%.4f ", x, y))
		case "L":
			x, _ := cmdMap["x"].(float64)
			y, _ := cmdMap["y"].(float64)
			d.WriteString(fmt.Sprintf("L%.4f,%.4f ", x, y))
		case "C":
			x1, _ := cmdMap["x1"].(float64)
			y1, _ := cmdMap["y1"].(float64)
			x2, _ := cmdMap["x2"].(float64)
			y2, _ := cmdMap["y2"].(float64)
			x, _ := cmdMap["x"].(float64)
			y, _ := cmdMap["y"].(float64)
			d.WriteString(fmt.Sprintf("C%.4f,%.4f %.4f,%.4f %.4f,%.4f ", x1, y1, x2, y2, x, y))
		case "Q":
			x1, _ := cmdMap["x1"].(float64)
			y1, _ := cmdMap["y1"].(float64)
			x, _ := cmdMap["x"].(float64)
			y, _ := cmdMap["y"].(float64)
			d.WriteString(fmt.Sprintf("Q%.4f,%.4f %.4f,%.4f ", x1, y1, x, y))
		case "Z":
			d.WriteString("Z ")
		}
	}

	pathD := strings.TrimSpace(d.String())
	if pathD == "" {
		return ""
	}

	// 构建 SVG 属性
	var attrs []string
	attrs = append(attrs, fmt.Sprintf(`d="%s"`, pathD))

	// 填充处理：纯色、渐变、图案
	fillColor := "none"
	var defsSVG string
	if pathObj.FillColor != nil {
		if pathObj.FillColor.Value != "" {
			// 纯色填充
			fillColor = parseColor(pathObj.FillColor.Value)
		} else if pathObj.FillColor.AxialShd != nil {
			// 轴向渐变（线性渐变）
			grad := p.parseAxialShd(pathObj.FillColor.AxialShd, ctm, scale)
			if grad != nil {
				svgGradientCounter++
				gradID := fmt.Sprintf("grad_%d", svgGradientCounter)
				// 渐变坐标需要加上 Boundary 偏移并转换为像素
				gx0 := (grad.X0 + bx) * scale
				gy0 := (grad.Y0 + by) * scale
				gx1 := (grad.X1 + bx) * scale
				gy1 := (grad.Y1 + by) * scale

				var stops strings.Builder
				for _, s := range grad.Stops {
					stops.WriteString(fmt.Sprintf(
						`<stop offset="%.2f" stop-color="%s"/>`, s.Position, s.Color))
				}
				defsSVG = fmt.Sprintf(
					`<defs><linearGradient id="%s" x1="%.4f" y1="%.4f" x2="%.4f" y2="%.4f" gradientUnits="userSpaceOnUse">%s</linearGradient></defs>`,
					gradID, gx0, gy0, gx1, gy1, stops.String())
				fillColor = fmt.Sprintf("url(#%s)", gradID)
			}
		} else if pathObj.FillColor.RadialShd != nil {
			// 径向渐变
			grad := p.parseRadialShd(pathObj.FillColor.RadialShd, ctm, scale)
			if grad != nil {
				svgGradientCounter++
				gradID := fmt.Sprintf("grad_%d", svgGradientCounter)
				gx1 := (grad.X1 + bx) * scale
				gy1 := (grad.Y1 + by) * scale
				r1 := grad.R1 * scale

				var stops strings.Builder
				for _, s := range grad.Stops {
					stops.WriteString(fmt.Sprintf(
						`<stop offset="%.2f" stop-color="%s"/>`, s.Position, s.Color))
				}
				defsSVG = fmt.Sprintf(
					`<defs><radialGradient id="%s" cx="%.4f" cy="%.4f" r="%.4f" gradientUnits="userSpaceOnUse">%s</radialGradient></defs>`,
					gradID, gx1, gy1, r1, stops.String())
				fillColor = fmt.Sprintf("url(#%s)", gradID)
			}
		}
	}
	attrs = append(attrs, fmt.Sprintf(`fill="%s"`, fillColor))

	// fill-rule: Even-Odd 对应 SVG 的 evenodd
	if strings.EqualFold(pathObj.Rule, "Even-Odd") {
		attrs = append(attrs, `fill-rule="evenodd"`)
	}

	// 描边：仅当有 StrokeColor 或明确 Stroke/LineWidth 时描边
	hasStroke := false
	if pathObj.StrokeColor != nil && pathObj.StrokeColor.Value != "" {
		attrs = append(attrs, fmt.Sprintf(`stroke="%s"`, parseColor(pathObj.StrokeColor.Value)))
		hasStroke = true
	} else if pathObj.Stroke || pathObj.LineWidth > 0 {
		attrs = append(attrs, `stroke="#000"`)
		hasStroke = true
	}
	if hasStroke {

		lw := pathObj.LineWidth
		if lw == 0 {
			lw = 1 / scale
		} else if len(ctm) >= 1 && ctm[0] > 0 {
			lw = lw * ctm[0]
		}
		attrs = append(attrs, fmt.Sprintf(`stroke-width="%.4f"`, lw*scale))

		if pathObj.Join != "" {
			attrs = append(attrs, fmt.Sprintf(`stroke-linejoin="%s"`, strings.ToLower(pathObj.Join)))
		}
		if pathObj.Cap != "" {
			attrs = append(attrs, fmt.Sprintf(`stroke-linecap="%s"`, strings.ToLower(pathObj.Cap)))
		}
		hasStroke = true
	}
	if !hasStroke {
		attrs = append(attrs, `stroke="none"`)
	}

	pathElem := fmt.Sprintf(`<path %s/>`, strings.Join(attrs, " "))
	if defsSVG != "" {
		return defsSVG + "\n" + pathElem
	}
	return pathElem
}

// imageObjectToSVG 将 ImageObject 转换为 SVG image 元素
func (p *Parser) imageObjectToSVG(img *ImageObject, scale float64, debug *DebugInfo) string {
	if img.Visible != nil && !*img.Visible {
		return ""
	}

	imgData := p.loadImageLazy(img.ResourceID)
	if imgData == nil || len(imgData) == 0 {
		if debug != nil {
			debug.MissingIDs = append(debug.MissingIDs, img.ResourceID)
		}
		return ""
	}

	ix, iy, iw, ih := parseBoundary(img.Boundary)

	mimeType := "image/png"
	if len(imgData) > 2 && imgData[0] == 0xFF && imgData[1] == 0xD8 {
		mimeType = "image/jpeg"
	}
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(imgData))

	// 处理 CTM 变换
	if img.CTM != "" {
		ctm := parseCTM(img.CTM)
		if len(ctm) >= 4 {
			a, b, c, d := ctm[0]*scale, ctm[1]*scale, ctm[2]*scale, ctm[3]*scale
			e, f := 0.0, 0.0
			if len(ctm) > 4 {
				e = ctm[4] * scale
			}
			if len(ctm) > 5 {
				f = ctm[5] * scale
			}
			px := ix * scale
			py := iy * scale
			// SVG transform: matrix(a,b,c,d,e,f) 然后在 (px,py) 处绘制 1x1 图片
			return fmt.Sprintf(
				`<image href="%s" x="%.2f" y="%.2f" width="1" height="1" transform="matrix(%.6f,%.6f,%.6f,%.6f,%.6f,%.6f)" preserveAspectRatio="none"/>`,
				dataURL, 0.0, 0.0, a, b, c, d, px+e, py+f)
		}
	}

	// 普通绘制
	px := ix * scale
	py := iy * scale
	pw := iw * scale
	ph := ih * scale

	// 处理 Alpha
	opacity := ""
	if img.Alpha > 0 && img.Alpha < 255 {
		opacity = fmt.Sprintf(` opacity="%.2f"`, float64(img.Alpha)/255.0)
	}

	return fmt.Sprintf(
		`<image href="%s" x="%.2f" y="%.2f" width="%.2f" height="%.2f"%s preserveAspectRatio="none"/>`,
		dataURL, px, py, pw, ph, opacity)
}

// loadPageAnnotSVG 加载页面注释图片为 SVG 元素
func (p *Parser) loadPageAnnotSVG(svgParts *[]string, scale float64, pageIndex int, debug *DebugInfo) {
	if p.document == nil || pageIndex >= len(p.document.Pages.Page) {
		return
	}

	pageID := p.document.Pages.Page[pageIndex].ID
	docBase := ""
	if p.ofd != nil && len(p.ofd.DocBody) > 0 {
		docRoot := strings.TrimPrefix(p.ofd.DocBody[0].DocRoot, "/")
		docBase = path.Dir(docRoot)
	}

	annotIndexPath := ""
	if p.document.Annotations != "" {
		annotIndexPath = path.Join(docBase, strings.TrimPrefix(p.document.Annotations, "/"))
	}
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
		return
	}

	xmlStr := removeNamespacePrefix(string(indexData))
	var annotIndex AnnotationsFile
	if err := xml.Unmarshal([]byte(xmlStr), &annotIndex); err != nil {
		return
	}

	annotIndexDir := path.Dir(annotIndexPath)

	for _, ap := range annotIndex.Pages {
		if ap.PageID != pageID {
			continue
		}

		annotFilePath := path.Join(annotIndexDir, strings.TrimPrefix(ap.FileLoc, "/"))
		annotData, err := p.readFile(annotFilePath)
		if err != nil {
			continue
		}

		annotXML := removeNamespacePrefix(string(annotData))
		var pageAnnot PageAnnot
		if err := xml.Unmarshal([]byte(annotXML), &pageAnnot); err != nil {
			continue
		}

		for _, annot := range pageAnnot.Annots {
			ax, ay, _, _ := parseBoundary(annot.Appearance.Boundary)

			var allImages []ImageObject
			for _, block := range annot.Appearance.PageBlocks {
				allImages = append(allImages, block.ImageObjects...)
			}
			allImages = append(allImages, annot.Appearance.ImageObjects...)

			for _, aImg := range allImages {
				imgData := p.loadImageLazy(aImg.ResourceID)
				if imgData == nil {
					continue
				}
				ix, iy, iw, ih := parseBoundary(aImg.Boundary)
				mimeType := "image/png"
				if len(imgData) > 2 && imgData[0] == 0xFF && imgData[1] == 0xD8 {
					mimeType = "image/jpeg"
				}
				dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(imgData))
				px := (ax + ix) * scale
				py := (ay + iy) * scale
				pw := iw * scale
				ph := ih * scale
				*svgParts = append(*svgParts, fmt.Sprintf(
					`<image href="%s" x="%.2f" y="%.2f" width="%.2f" height="%.2f" preserveAspectRatio="none"/>`,
					dataURL, px, py, pw, ph))
			}
		}
	}
}

// loadStampsSVG 加载印章为 SVG 元素
// 复用已有的 loadStamps 逻辑，但输出到 SVG
func (p *Parser) loadStampsSVG(result *PageRenderResult, svgParts *[]string, scale float64, pageIndex int, debug *DebugInfo) {
	// 使用已有的 RenderPage 逻辑获取印章数据
	// 创建临时 result 来收集印章图片
	tmpResult := &PageRenderResult{
		CanvasData: &CanvasRenderData{},
		TextLayer:  make([]TextItem, 0),
	}
	tmpResult.Width = result.Width
	tmpResult.Height = result.Height

	p.loadStamps(tmpResult, scale, pageIndex, debug)

	// 将收集到的印章图片转换为 SVG
	for _, img := range tmpResult.CanvasData.Images {
		if img.CTM != nil && len(img.CTM) >= 4 {
			a, b, c, d := img.CTM[0], img.CTM[1], img.CTM[2], img.CTM[3]
			e, f := 0.0, 0.0
			if len(img.CTM) > 4 {
				e = img.CTM[4]
			}
			if len(img.CTM) > 5 {
				f = img.CTM[5]
			}
			*svgParts = append(*svgParts, fmt.Sprintf(
				`<image href="%s" x="0" y="0" width="1" height="1" transform="matrix(%.6f,%.6f,%.6f,%.6f,%.6f,%.6f)" preserveAspectRatio="none"/>`,
				img.DataURL, a, b, c, d, img.X+e, img.Y+f))
		} else {
			opacity := ""
			if img.Alpha > 0 && img.Alpha < 1 {
				opacity = fmt.Sprintf(` opacity="%.2f"`, img.Alpha)
			}
			*svgParts = append(*svgParts, fmt.Sprintf(
				`<image href="%s" x="%.2f" y="%.2f" width="%.2f" height="%.2f"%s preserveAspectRatio="none"/>`,
				img.DataURL, img.X, img.Y, img.Width, img.Height, opacity))
		}
	}

	// 将占位文本也添加到 textOverlay
	for _, ti := range tmpResult.TextLayer {
		if ti.Text == "__PLACEHOLDER__" {
			result.TextOverlay = append(result.TextOverlay, TextOverlayItem{
				Text:   "[电子签章]",
				X:      ti.X,
				Y:      ti.Y,
				Width:  ti.Width,
				Height: ti.Height,
			})
		}
	}
}



package ofd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strconv"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

// TextRenderResult 文字渲染结果
// 当 DataURL 非空时，使用图片渲染
// 当 DataURL 为空时，使用 SVG text 回退渲染（系统字体）
type TextRenderResult struct {
	DataURL     string  `json:"dataURL,omitempty"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	// SVG text 回退字段
	Text        string  `json:"text,omitempty"`
	FontSize    float64 `json:"fontSize,omitempty"`
	FontFamily  string  `json:"fontFamily,omitempty"`
	Color       string  `json:"color,omitempty"`
	Weight      int     `json:"weight,omitempty"`
	Italic      bool    `json:"italic,omitempty"`
	Stroke      bool    `json:"stroke,omitempty"`
	StrokeColor string  `json:"strokeColor,omitempty"`
	StrokeWidth float64 `json:"strokeWidth,omitempty"`
	Fill        bool    `json:"fill,omitempty"`
}

// TextOverlayItem 文本蒙层项（用于浏览器搜索）
type TextOverlayItem struct {
	Text   string  `json:"text"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// parsedFont 缓存已解析的 opentype 字体
type parsedFont struct {
	font *opentype.Font
}

// fontCache 字体缓存
var fontCache = make(map[string]*parsedFont)

// boldFontCache 加粗字体缓存（通过 EmboldenFont 生成）
var boldFontCache = make(map[string]*parsedFont)

// getOrParseOTFont 获取或解析 opentype 字体
func getOrParseOTFont(fontData []byte, fontID string) *opentype.Font {
	if cached, ok := fontCache[fontID]; ok {
		return cached.font
	}
	f, err := opentype.Parse(fontData)
	if err != nil {
		return nil
	}
	fontCache[fontID] = &parsedFont{font: f}
	return f
}

// getOrParseBoldOTFont 获取或生成加粗版 opentype 字体
// 使用 EmboldenFont 在字体轮廓层面进行真正的加粗（类似 FT_Outline_Embolden）
func getOrParseBoldOTFont(fontData []byte, fontID string) *opentype.Font {
	boldID := fontID + "_bold"
	if cached, ok := boldFontCache[boldID]; ok {
		return cached.font
	}
	boldData := EmboldenFont(fontData)
	f, err := opentype.Parse(boldData)
	if err != nil {
		return nil
	}
	boldFontCache[boldID] = &parsedFont{font: f}
	return f
}

// renderTextObject 将一个 TextObject 渲染为图片
// 返回渲染结果和文本蒙层数据
func (p *Parser) renderTextObject(text *TextObject, scale float64, debug *DebugInfo) ([]TextRenderResult, []TextOverlayItem) {
	if text.Visible != nil && !*text.Visible {
		return nil, nil
	}

	bx, by, _, _ := parseBoundary(text.Boundary)
	fontID := text.Font
	fontSize := text.Size // mm

	// 获取字体数据
	var otFont *opentype.Font
	fontSource := "none"
	if fontData, ok := p.fontFiles[fontID]; ok && len(fontData) > 0 {
		otFont = getOrParseOTFont(fontData, fontID)
		if otFont != nil {
			fontSource = "fontFiles_cache"
		} else {
			fontSource = "fontFiles_parse_failed"
		}
	}
	if otFont == nil {
		// 尝试从字体声明中加载
		if f, ok := p.fonts[fontID]; ok && f.FontFile != "" {
			if fontData, err := p.readFile(f.FontFile); err == nil && len(fontData) > 0 {
				// 修复字体
				fontData = SanitizeFontWithMappings(fontData, nil)
				p.fontFiles[fontID] = fontData
				otFont = getOrParseOTFont(fontData, fontID)
				if otFont != nil {
					fontSource = "lazy_load"
				} else {
					fontSource = fmt.Sprintf("lazy_parse_failed(len=%d)", len(fontData))
				}
			} else if err != nil {
				fontSource = fmt.Sprintf("file_read_err(%s: %v)", f.FontFile, err)
			}
		} else if _, ok := p.fonts[fontID]; !ok {
			fontSource = "font_id_not_found"
		} else {
			fontSource = "no_font_file"
		}
	}

	if debug != nil {
		fontName := ""
		if f, ok := p.fonts[fontID]; ok {
			fontName = f.FontName
		}
		debug.TextDebug = append(debug.TextDebug, fmt.Sprintf(
			"renderTextObject: fontID=%s, fontName=%s, fontSize=%.2fmm, source=%s, hasOTFont=%v",
			fontID, fontName, fontSize, fontSource, otFont != nil))
	}

	// 解析颜色
	fillColor := color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	if text.FillColor != nil && text.FillColor.Value != "" {
		fillColor = parseNRGBAColor(text.FillColor.Value)
	}

	// 描边颜色
	strokeColor := color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	if text.StrokeColor != nil && text.StrokeColor.Value != "" {
		strokeColor = parseNRGBAColor(text.StrokeColor.Value)
	}

	// 确定渲染使用的颜色和模式
	// OFD 规范：Stroke=true 且无 FillColor 时为纯描边（空心字）
	hasFill := !text.Stroke || (text.FillColor != nil && text.FillColor.Value != "")

	// 填充色
	renderFillColor := fillColor
	// 描边色
	renderStrokeColor := strokeColor

	// 解析 CTM
	var ctm []float64
	if text.CTM != "" {
		ctm = parseCTM(text.CTM)
	}

	// 判断是否加粗
	isBold := text.Weight >= 700

	// 如果需要加粗且有字体数据，使用 EmboldenFont 生成加粗版字体
	if isBold && otFont != nil {
		if fontData, ok := p.fontFiles[fontID]; ok && len(fontData) > 0 {
			if boldFont := getOrParseBoldOTFont(fontData, fontID); boldFont != nil {
				otFont = boldFont
			}
		}
	}

	// 计算描边线宽（像素）
	lineWidthPx := 0.0
	if text.Stroke && text.LineWidth > 0 {
		lw := text.LineWidth
		if len(ctm) >= 4 {
			det := math.Abs(ctm[0]*ctm[3] - ctm[1]*ctm[2])
			if det > 0 {
				lw *= math.Sqrt(det)
			}
		}
		lineWidthPx = lw * scale
	}

	// HScale：水平缩放因子（默认 1.0）
	hScale := text.HScale
	if hScale == 0 {
		hScale = 1.0
	}

	// CharSpace：字符间距（mm，默认 0）
	charSpace := text.CharSpace

	var results []TextRenderResult
	var overlays []TextOverlayItem

	for _, tc := range text.TextCode {
		content := tc.Content
		if len(content) == 0 {
			continue
		}
		chars := []rune(content)

		// 解析 DeltaX/DeltaY
		var deltaX, deltaY []float64
		if tc.DeltaX != "" {
			deltaX = parseDeltas(tc.DeltaX)
		}
		if tc.DeltaY != "" {
			deltaY = parseDeltas(tc.DeltaY)
		}

		// 计算 TextCode 坐标
		tcXmm := tc.X
		tcYmm := tc.Y
		if len(ctm) >= 4 {
			a, b, c, d := ctm[0], ctm[1], ctm[2], ctm[3]
			tcXmm = a*tc.X + c*tc.Y
			tcYmm = b*tc.X + d*tc.Y
		}

		currentXmm := tcXmm
		currentYmm := tcYmm

		// CTM 缩放
		scaleX := 1.0
		scaleY := 1.0
		if len(ctm) >= 4 {
			scaleX = math.Abs(ctm[0])
			scaleY = math.Abs(ctm[3])
			if scaleX < 0.001 {
				scaleX = 1.0
			}
			if scaleY < 0.001 {
				scaleY = 1.0
			}
		}
		fontSizePx := fontSize * scale
		effectiveFontSizePx := fontSizePx * scaleY

		// 准备回退字体信息
		fontFamily := "SimSun, serif"
		if f, ok := p.fonts[fontID]; ok {
			fontFamily = fmt.Sprintf("'%s', '%s', SimSun, serif", f.FontName, f.FamilyName)
		}
		colorStr := fmt.Sprintf("rgb(%d,%d,%d)", fillColor.R, fillColor.G, fillColor.B)
		strokeColorStr := ""
		if text.Stroke && text.StrokeColor != nil && text.StrokeColor.Value != "" {
			strokeColorStr = parseColor(text.StrokeColor.Value)
			if text.FillColor == nil || text.FillColor.Value == "" {
				colorStr = strokeColorStr
			}
		}

		var segText []rune
		var segPositions []float64
		segStartYmm := currentYmm

		flushSegment := func() {
			if len(segText) == 0 {
				return
			}

			if otFont != nil {
				for ci, ch := range segText {
					charPx := (bx + segPositions[ci]) * scale
					charPy := (by + segStartYmm) * scale
					imgResult := renderTextToImage(otFont, string(ch), effectiveFontSizePx, renderFillColor, renderStrokeColor, isBold, lineWidthPx, hasFill)
					if imgResult != nil {
						results = append(results, TextRenderResult{
							DataURL: imgResult.dataURL,
							X:       charPx,
							Y:       charPy - effectiveFontSizePx*0.85,
							Width:   imgResult.width,
							Height:  imgResult.height,
						})
					}
				}
				if debug != nil && len(results) <= 5 {
					txt := string(segText)
					px := (bx + segPositions[0]) * scale
					py := (by + segStartYmm) * scale
					debug.TextDebug = append(debug.TextDebug, fmt.Sprintf(
						"  rendered: text='%s', pos=(%.1f,%.1f), chars=%d, fontSizePx=%.1f, effectivePx=%.1f, scaleXY=(%.2f,%.2f), hScale=%.2f",
						txt, px, py, len(segText), fontSizePx, effectiveFontSizePx, scaleX, scaleY, hScale))
				}
			}

			// 文本蒙层
			txt := string(segText)
			firstPx := (bx + segPositions[0]) * scale
			lastPx := (bx + segPositions[len(segPositions)-1]) * scale
			py := (by + segStartYmm) * scale
			textWidth := lastPx - firstPx + effectiveFontSizePx*0.9
			overlays = append(overlays, TextOverlayItem{
				Text:   txt,
				X:      firstPx,
				Y:      py - effectiveFontSizePx*0.85,
				Width:  textWidth,
				Height: effectiveFontSizePx * 1.2,
			})

			segText = nil
			segPositions = nil
		}

		for i, char := range chars {
			isSpace := char == ' ' || char == '\u3000'

			if isSpace {
				flushSegment()
			} else {
				if len(segText) == 0 {
					segStartYmm = currentYmm
				}
				segText = append(segText, char)
				segPositions = append(segPositions, currentXmm)

				// 没有嵌入字体时，逐字符生成 SVG text
				if otFont == nil {
					cpx := (bx + currentXmm) * scale
					cpy := (by + currentYmm) * scale
					tr := TextRenderResult{
						Text:       string(char),
						X:          cpx,
						Y:          cpy,
						FontSize:   effectiveFontSizePx * hScale,
						FontFamily: fontFamily,
						Color:      colorStr,
						Weight:     text.Weight,
						Italic:     text.Italic,
					}
					if text.Stroke && strokeColorStr != "" {
						tr.Stroke = true
						tr.StrokeColor = strokeColorStr
						tr.StrokeWidth = lineWidthPx
						tr.Fill = text.FillColor != nil && text.FillColor.Value != ""
					}
					results = append(results, tr)
				}
			}

			// 计算下一个字符位置
			dx := float64(0)
			dy := float64(0)
			if i < len(deltaX) {
				dx = deltaX[i]
			} else if i < len(chars)-1 {
				dx = fontSize * hScale + charSpace
			}
			if i < len(deltaY) {
				dy = deltaY[i]
			}

			if dx != 0 || dy != 0 {
				if len(ctm) >= 4 {
					a, b, c, d := ctm[0], ctm[1], ctm[2], ctm[3]
					tdx := a*dx + c*dy
					tdy := b*dx + d*dy
					currentXmm += tdx
					currentYmm += tdy
				} else {
					currentXmm += dx
					currentYmm += dy
				}
			}
		}
		flushSegment()
	}

	return results, overlays
}

// textImageResult 渲染结果
type textImageResult struct {
	dataURL string
	width   float64
	height  float64
}

// renderTextToImage 使用 opentype 将文字渲染为 PNG 图片
// 描边使用 sfnt 提取字形轮廓 + vector.Rasterizer 精确光栅化
// fillClr: 填充色, strokeClr: 描边色, hasFill: 是否填充（false=纯描边空心字）
func renderTextToImage(otFont *opentype.Font, text string, fontSizePx float64, fillClr, strokeClr color.NRGBA, _ bool, lineWidthPx float64, hasFill bool) *textImageResult {
	if fontSizePx < 1 {
		fontSizePx = 1
	}

	dpi := 72.0
	ptSize := fontSizePx

	// 1. 创建 Face（HintingFull）
	face, err := opentype.NewFace(otFont, &opentype.FaceOptions{
		Size:    ptSize,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil
	}
	defer face.Close()

	// 2. 测量文字宽度
	var totalAdvance fixed.Int26_6
	for _, r := range text {
		adv, ok := face.GlyphAdvance(r)
		if !ok {
			adv = face.Metrics().Height / 2
		}
		totalAdvance += adv
	}

	textWidth := float64(totalAdvance) / 64.0
	textHeight := fontSizePx * 1.4

	// 计算 Padding
	padding := 2.0
	if lineWidthPx > 0 {
		padding += math.Ceil(lineWidthPx) + 1
	}

	imgW := int(math.Ceil(textWidth+padding*2)) + 4
	imgH := int(math.Ceil(textHeight+padding*2)) + 2
	if imgW < 1 {
		imgW = 1
	}
	if imgH < 1 {
		imgH = 1
	}
	if imgW > 4096 {
		imgW = 4096
	}
	if imgH > 1024 {
		imgH = 1024
	}

	img := image.NewRGBA(image.Rect(0, 0, imgW, imgH))

	startX := int(padding + 1)
	baselineY := int(fontSizePx*1.0 + padding)

	// 3. 描边渲染：使用 sfnt 提取字形轮廓 + vector.Rasterizer
	if lineWidthPx > 0 {
		renderGlyphStrokeOT(img, otFont, text, fontSizePx, dpi, float64(startX), float64(baselineY), lineWidthPx, strokeClr)
	}

	// 4. 填充渲染：仅在需要填充时绘制（hasFill=false 时为纯描边空心字，不填充）
	if hasFill {
		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(fillClr),
			Face: face,
			Dot:  fixed.P(startX, baselineY),
		}
		d.DrawString(text)
	}

	// 5. 后处理：轻微黑度增强（仅对填充文字，描边文字不做增强以保持线宽准确）
	if hasFill {
		pixels := img.Pix
		gamma := 0.55
		gain := 1.1
		for i := 0; i < len(pixels); i += 4 {
			a := float64(pixels[i+3]) / 255.0
			if a > 0 {
				newA := math.Pow(a, gamma)
				newA *= gain
				if newA > 1.0 {
					newA = 1.0
				}
				pixels[i+3] = uint8(newA * 255)
				pixels[i] = fillClr.R
				pixels[i+1] = fillClr.G
				pixels[i+2] = fillClr.B
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}

	dataURL := fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(buf.Bytes()))
	return &textImageResult{
		dataURL: dataURL,
		width:   float64(imgW),
		height:  float64(imgH),
	}
}

// renderGlyphStrokeOT 使用 sfnt 提取字形轮廓，沿法线偏移生成描边路径，
// 用 vector.Rasterizer 精确光栅化。支持任意 lineWidth。
func renderGlyphStrokeOT(dst *image.RGBA, otFont *opentype.Font, text string, fontSizePx, dpi, startX, baselineY, lineWidthPx float64, clr color.NRGBA) {
	// sfnt.Font 是 opentype.Font 的底层类型，可以直接转换
	sf := (*sfnt.Font)(otFont)

	ppem := fixed.Int26_6(fontSizePx * 64)
	var buf sfnt.Buffer

	halfLW := float32(lineWidthPx / 2.0)
	curX := startX

	bounds := dst.Bounds()
	rast := vector.NewRasterizer(bounds.Dx(), bounds.Dy())

	for _, r := range text {
		idx, err := sf.GlyphIndex(&buf, r)
		if err != nil || idx == 0 {
			curX += fontSizePx * 0.5
			continue
		}

		// 获取字形前进宽度
		adv, err := sf.GlyphAdvance(&buf, idx, ppem, font.HintingFull)
		if err != nil {
			curX += fontSizePx * 0.5
			continue
		}

		// 加载字形轮廓
		segs, err := sf.LoadGlyph(&buf, idx, ppem, nil)
		if err != nil {
			curX += float64(adv) / 64.0
			continue
		}

		// 将 sfnt segments 转换为像素坐标点列表（按轮廓分组）
		// sfnt 坐标系：Y 轴向下（与屏幕坐标一致）
		ox := float32(curX)
		oy := float32(baselineY)

		var contours [][]struct{ x, y float32 }
		var currentContour []struct{ x, y float32 }

		for _, seg := range segs {
			switch seg.Op {
			case sfnt.SegmentOpMoveTo:
				if len(currentContour) > 0 {
					contours = append(contours, currentContour)
				}
				px := ox + float32(seg.Args[0].X)/64.0
				py := oy + float32(seg.Args[0].Y)/64.0
				currentContour = []struct{ x, y float32 }{{px, py}}

			case sfnt.SegmentOpLineTo:
				px := ox + float32(seg.Args[0].X)/64.0
				py := oy + float32(seg.Args[0].Y)/64.0
				currentContour = append(currentContour, struct{ x, y float32 }{px, py})

			case sfnt.SegmentOpQuadTo:
				// 二次贝塞尔 → 细分为线段
				if len(currentContour) == 0 {
					continue
				}
				prev := currentContour[len(currentContour)-1]
				cx := ox + float32(seg.Args[0].X)/64.0
				cy := oy + float32(seg.Args[0].Y)/64.0
				ex := ox + float32(seg.Args[1].X)/64.0
				ey := oy + float32(seg.Args[1].Y)/64.0
				steps := 8
				for t := 1; t <= steps; t++ {
					tt := float32(t) / float32(steps)
					inv := 1.0 - tt
					bx := inv*inv*prev.x + 2*inv*tt*cx + tt*tt*ex
					by := inv*inv*prev.y + 2*inv*tt*cy + tt*tt*ey
					currentContour = append(currentContour, struct{ x, y float32 }{bx, by})
				}

			case sfnt.SegmentOpCubeTo:
				// 三次贝塞尔 → 细分为线段
				if len(currentContour) == 0 {
					continue
				}
				prev := currentContour[len(currentContour)-1]
				c1x := ox + float32(seg.Args[0].X)/64.0
				c1y := oy + float32(seg.Args[0].Y)/64.0
				c2x := ox + float32(seg.Args[1].X)/64.0
				c2y := oy + float32(seg.Args[1].Y)/64.0
				ex := ox + float32(seg.Args[2].X)/64.0
				ey := oy + float32(seg.Args[2].Y)/64.0
				steps := 12
				for t := 1; t <= steps; t++ {
					tt := float32(t) / float32(steps)
					inv := 1.0 - tt
					bx := inv*inv*inv*prev.x + 3*inv*inv*tt*c1x + 3*inv*tt*tt*c2x + tt*tt*tt*ex
					by := inv*inv*inv*prev.y + 3*inv*inv*tt*c1y + 3*inv*tt*tt*c2y + tt*tt*tt*ey
					currentContour = append(currentContour, struct{ x, y float32 }{bx, by})
				}
			}
		}
		if len(currentContour) > 0 {
			contours = append(contours, currentContour)
		}

		// 对每个轮廓生成描边几何
		for _, pts := range contours {
			if len(pts) < 2 {
				continue
			}
			strokeContour(rast, pts, halfLW)
		}

		curX += float64(adv) / 64.0
	}

	// 光栅化到目标图片
	rast.Draw(dst, dst.Bounds(), image.NewUniform(clr), image.Point{})
}

// strokeContour 对一个闭合轮廓生成描边几何（矩形条带 + 圆形端点）
func strokeContour(rast *vector.Rasterizer, pts []struct{ x, y float32 }, halfLW float32) {
	n := len(pts)

	// 每条线段生成描边矩形
	for i := 0; i < n; i++ {
		p1 := pts[i]
		p2 := pts[(i+1)%n]

		dx := p2.x - p1.x
		dy := p2.y - p1.y
		length := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		if length < 0.001 {
			continue
		}

		// 法线方向
		nx := -dy / length * halfLW
		ny := dx / length * halfLW

		// 描边矩形
		rast.MoveTo(p1.x+nx, p1.y+ny)
		rast.LineTo(p2.x+nx, p2.y+ny)
		rast.LineTo(p2.x-nx, p2.y-ny)
		rast.LineTo(p1.x-nx, p1.y-ny)
		rast.ClosePath()
	}

	// 圆形连接（round join）：在每个顶点画小扇形
	circleSteps := 8
	for _, p := range pts {
		for j := 0; j < circleSteps; j++ {
			a1 := 2.0 * math.Pi * float64(j) / float64(circleSteps)
			a2 := 2.0 * math.Pi * float64(j+1) / float64(circleSteps)
			rast.MoveTo(p.x, p.y)
			rast.LineTo(p.x+float32(math.Cos(a1))*halfLW, p.y+float32(math.Sin(a1))*halfLW)
			rast.LineTo(p.x+float32(math.Cos(a2))*halfLW, p.y+float32(math.Sin(a2))*halfLW)
			rast.ClosePath()
		}
	}
}

// parseNRGBAColor 解析 OFD 颜色值为 NRGBA
func parseNRGBAColor(value string) color.NRGBA {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "#") {
		if len(value) == 7 {
			r, _ := strconv.ParseUint(value[1:3], 16, 8)
			g, _ := strconv.ParseUint(value[3:5], 16, 8)
			b, _ := strconv.ParseUint(value[5:7], 16, 8)
			return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
		}
	}
	parts := strings.Fields(value)
	if len(parts) >= 3 {
		r, _ := strconv.Atoi(parts[0])
		g, _ := strconv.Atoi(parts[1])
		b, _ := strconv.Atoi(parts[2])
		return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
	}
	return color.NRGBA{R: 0, G: 0, B: 0, A: 255}
}

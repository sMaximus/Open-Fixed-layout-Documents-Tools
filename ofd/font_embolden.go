package ofd

import (
	"encoding/binary"
	"math"
	"sort"
)

// EmboldenFont 对 Regular 字体进行真正的加粗处理
// 通过解析并重建 glyf 表中的轮廓点坐标实现加粗，
// 同时更新 hmtx 的 advanceWidth 和 OS/2、head 的 Bold 元数据。
// 加粗强度为 unitsPerEm 的 ~2.5%，与 FreeType 的 FT_GlyphSlot_Embolden 类似。
func EmboldenFont(data []byte) []byte {
	if len(data) < 12 || !IsFontData(data) {
		return data
	}

	sfVersion := binary.BigEndian.Uint32(data[0:4])
	numTables := int(binary.BigEndian.Uint16(data[4:6]))
	if numTables == 0 || 12+numTables*16 > len(data) {
		return data
	}

	// 解析所有表
	ftables := make([]fontTable, 0, numTables)
	for i := 0; i < numTables; i++ {
		off := 12 + i*16
		tag := string(data[off : off+4])
		tOff := int(binary.BigEndian.Uint32(data[off+8 : off+12]))
		tLen := int(binary.BigEndian.Uint32(data[off+12 : off+16]))
		if tOff+tLen > len(data) {
			continue
		}
		td := make([]byte, tLen)
		copy(td, data[tOff:tOff+tLen])
		ftables = append(ftables, fontTable{tag: tag, data: td})
	}

	tableMap := make(map[string]*fontTable)
	for i := range ftables {
		tableMap[ftables[i].tag] = &ftables[i]
	}

	headT := tableMap["head"]
	glyfT := tableMap["glyf"]
	locaT := tableMap["loca"]
	maxpT := tableMap["maxp"]
	hmtxT := tableMap["hmtx"]
	hheaT := tableMap["hhea"]

	if headT == nil || len(headT.data) < 54 {
		return data
	}

	unitsPerEm := int(binary.BigEndian.Uint16(headT.data[18:20]))
	if unitsPerEm == 0 {
		unitsPerEm = 1000
	}
	strength := int16(math.Round(float64(unitsPerEm) * 0.025))
	if strength < 1 {
		strength = 1
	}

	// 如果有 glyf 表，进行真正的轮廓加粗
	if glyfT != nil && locaT != nil && maxpT != nil && len(maxpT.data) >= 6 {
		indexToLocFormat := int16(binary.BigEndian.Uint16(headT.data[50:52]))
		numGlyphs := int(binary.BigEndian.Uint16(maxpT.data[4:6]))
		offsets := parseLocaTable(locaT.data, indexToLocFormat, numGlyphs)

		if len(offsets) >= numGlyphs+1 {
			newGlyfData, newOffsets := emboldenGlyfTable(glyfT.data, offsets, numGlyphs, strength)
			glyfT.data = newGlyfData

			// 重建 loca 表（long 格式）
			newLoca := make([]byte, (numGlyphs+1)*4)
			for i, o := range newOffsets {
				off := i * 4
				if off+4 <= len(newLoca) {
					binary.BigEndian.PutUint32(newLoca[off:off+4], o)
				}
			}
			locaT.data = newLoca
			// 更新 head.indexToLocFormat = 1 (long)
			binary.BigEndian.PutUint16(headT.data[50:52], 1)
		}
	}

	// 更新 hmtx：增加 advanceWidth
	if hmtxT != nil && hheaT != nil && len(hheaT.data) >= 36 {
		numHMetrics := int(binary.BigEndian.Uint16(hheaT.data[34:36]))
		for i := 0; i < numHMetrics && i*4+2 <= len(hmtxT.data); i++ {
			aw := binary.BigEndian.Uint16(hmtxT.data[i*4 : i*4+2])
			newAW := int(aw) + int(strength)
			if newAW > 0xFFFF {
				newAW = 0xFFFF
			}
			binary.BigEndian.PutUint16(hmtxT.data[i*4:i*4+2], uint16(newAW))
		}
	}

	// 设置 Bold 元数据
	if os2T := tableMap["OS/2"]; os2T != nil {
		if len(os2T.data) >= 6 {
			binary.BigEndian.PutUint16(os2T.data[4:6], 700)
		}
		if len(os2T.data) >= 64 {
			fs := binary.BigEndian.Uint16(os2T.data[62:64])
			fs |= 0x0020
			fs &^= 0x0040
			binary.BigEndian.PutUint16(os2T.data[62:64], fs)
		}
	}
	if headT != nil && len(headT.data) >= 46 {
		ms := binary.BigEndian.Uint16(headT.data[44:46])
		ms |= 0x0001
		binary.BigEndian.PutUint16(headT.data[44:46], ms)
	}

	// 排序并重建字体
	sort.Slice(ftables, func(i, j int) bool {
		return ftables[i].tag < ftables[j].tag
	})

	return serializeFont(sfVersion, ftables)
}

// emboldenGlyfTable 对 glyf 表中的所有简单 glyph 进行加粗
// 返回新的 glyf 数据和新的 loca 偏移表
func emboldenGlyfTable(glyfData []byte, offsets []uint32, numGlyphs int, strength int16) ([]byte, []uint32) {
	newGlyf := make([]byte, 0, len(glyfData)*2)
	newOffsets := make([]uint32, numGlyphs+1)

	for gi := 0; gi < numGlyphs; gi++ {
		newOffsets[gi] = uint32(len(newGlyf))

		gOff := offsets[gi]
		gEnd := offsets[gi+1]

		if gOff >= gEnd {
			continue // 空 glyph
		}
		if int(gOff) >= len(glyfData) || int(gEnd) > len(glyfData) {
			newGlyf = append(newGlyf, glyfData[gOff:gEnd]...)
			for len(newGlyf)%4 != 0 {
				newGlyf = append(newGlyf, 0)
			}
			continue
		}

		glyph := glyfData[gOff:gEnd]
		if len(glyph) < 10 {
			newGlyf = append(newGlyf, glyph...)
			for len(newGlyf)%4 != 0 {
				newGlyf = append(newGlyf, 0)
			}
			continue
		}

		numContours := int(int16(binary.BigEndian.Uint16(glyph[0:2])))
		if numContours <= 0 {
			newGlyf = append(newGlyf, glyph...)
			for len(newGlyf)%4 != 0 {
				newGlyf = append(newGlyf, 0)
			}
			continue
		}

		boldGlyph := emboldenSimpleGlyph(glyph, numContours, strength)
		if boldGlyph == nil {
			newGlyf = append(newGlyf, glyph...)
		} else {
			newGlyf = append(newGlyf, boldGlyph...)
		}
		for len(newGlyf)%4 != 0 {
			newGlyf = append(newGlyf, 0)
		}
	}
	newOffsets[numGlyphs] = uint32(len(newGlyf))

	return newGlyf, newOffsets
}

// emboldenSimpleGlyph 解析简单 glyph 并返回加粗后的新 glyph 数据
func emboldenSimpleGlyph(glyph []byte, numContours int, strength int16) []byte {
	if len(glyph) < 10+numContours*2 {
		return nil
	}

	// 读取 header
	// xMin := int16(binary.BigEndian.Uint16(glyph[2:4]))
	yMin := int16(binary.BigEndian.Uint16(glyph[4:6]))
	xMax := int16(binary.BigEndian.Uint16(glyph[6:8]))
	yMax := int16(binary.BigEndian.Uint16(glyph[8:10]))

	// 读取 endPtsOfContours
	endPts := make([]int, numContours)
	for i := 0; i < numContours; i++ {
		off := 10 + i*2
		endPts[i] = int(binary.BigEndian.Uint16(glyph[off : off+2]))
	}
	totalPoints := endPts[numContours-1] + 1
	if totalPoints <= 0 || totalPoints > 16384 {
		return nil
	}

	// 读取 instructions
	instrOff := 10 + numContours*2
	if instrOff+2 > len(glyph) {
		return nil
	}
	instrLen := int(binary.BigEndian.Uint16(glyph[instrOff : instrOff+2]))
	instructions := glyph[instrOff+2 : instrOff+2+instrLen]
	if instrOff+2+instrLen > len(glyph) {
		return nil
	}

	// 解析 flags
	flagsStart := instrOff + 2 + instrLen
	flags := make([]byte, 0, totalPoints)
	pos := flagsStart
	for len(flags) < totalPoints {
		if pos >= len(glyph) {
			return nil
		}
		f := glyph[pos]
		pos++
		flags = append(flags, f)
		if f&0x08 != 0 { // repeat
			if pos >= len(glyph) {
				return nil
			}
			repeat := int(glyph[pos])
			pos++
			for j := 0; j < repeat; j++ {
				flags = append(flags, f)
			}
		}
	}

	// 解析 X 坐标
	xCoords := make([]int16, totalPoints)
	var cx int16
	for i := 0; i < totalPoints; i++ {
		xShort := flags[i]&0x02 != 0
		xSame := flags[i]&0x10 != 0
		if xShort {
			if pos >= len(glyph) {
				return nil
			}
			v := int16(glyph[pos])
			pos++
			if !xSame {
				v = -v
			}
			cx += v
		} else if !xSame {
			if pos+2 > len(glyph) {
				return nil
			}
			cx += int16(binary.BigEndian.Uint16(glyph[pos : pos+2]))
			pos += 2
		}
		xCoords[i] = cx
	}

	// 解析 Y 坐标
	yCoords := make([]int16, totalPoints)
	var cy int16
	for i := 0; i < totalPoints; i++ {
		yShort := flags[i]&0x04 != 0
		ySame := flags[i]&0x20 != 0
		if yShort {
			if pos >= len(glyph) {
				return nil
			}
			v := int16(glyph[pos])
			pos++
			if !ySame {
				v = -v
			}
			cy += v
		} else if !ySame {
			if pos+2 > len(glyph) {
				return nil
			}
			cy += int16(binary.BigEndian.Uint16(glyph[pos : pos+2]))
			pos += 2
		}
		yCoords[i] = cy
	}

	// 加粗：沿轮廓法线方向外扩
	newX := make([]int16, totalPoints)
	newY := make([]int16, totalPoints)
	copy(newX, xCoords)
	copy(newY, yCoords)

	str := float64(strength)
	contourStart := 0
	for c := 0; c < numContours; c++ {
		contourEnd := endPts[c]
		contourLen := contourEnd - contourStart + 1
		if contourLen < 3 {
			contourStart = contourEnd + 1
			continue
		}

		for i := contourStart; i <= contourEnd; i++ {
			prev := i - 1
			if prev < contourStart {
				prev = contourEnd
			}
			next := i + 1
			if next > contourEnd {
				next = contourStart
			}

			dx1 := float64(xCoords[i] - xCoords[prev])
			dy1 := float64(yCoords[i] - yCoords[prev])
			dx2 := float64(xCoords[next] - xCoords[i])
			dy2 := float64(yCoords[next] - yCoords[i])

			len1 := math.Sqrt(dx1*dx1 + dy1*dy1)
			len2 := math.Sqrt(dx2*dx2 + dy2*dy2)

			var nx, ny float64
			if len1 > 0.001 && len2 > 0.001 {
				nx1, ny1 := dy1/len1, -dx1/len1
				nx2, ny2 := dy2/len2, -dx2/len2
				nx = (nx1 + nx2) / 2
				ny = (ny1 + ny2) / 2
				nlen := math.Sqrt(nx*nx + ny*ny)
				if nlen > 0.001 {
					nx /= nlen
					ny /= nlen
				}
			} else if len1 > 0.001 {
				nx, ny = dy1/len1, -dx1/len1
			} else if len2 > 0.001 {
				nx, ny = dy2/len2, -dx2/len2
			}

			newX[i] = xCoords[i] + int16(math.Round(str*nx))
			newY[i] = yCoords[i] + int16(math.Round(str*ny*0.5))
		}
		contourStart = contourEnd + 1
	}

	// 更新 bounding box
	xMax += strength
	_ = yMin
	_ = yMax

	// 重新编码 glyph
	return encodeSimpleGlyph(numContours, int16(binary.BigEndian.Uint16(glyph[2:4])), yMin, xMax, yMax,
		endPts, instructions, newX, newY, flags)
}

// encodeSimpleGlyph 编码简单 glyph 为 TrueType 格式
func encodeSimpleGlyph(numContours int, xMin, yMin, xMax, yMax int16,
	endPts []int, instructions []byte, xCoords, yCoords []int16, origFlags []byte) []byte {

	totalPoints := len(xCoords)
	// 预估大小
	buf := make([]byte, 0, 10+numContours*2+2+len(instructions)+totalPoints*5)

	// Header
	h := make([]byte, 10)
	binary.BigEndian.PutUint16(h[0:2], uint16(numContours))
	binary.BigEndian.PutUint16(h[2:4], uint16(xMin))
	binary.BigEndian.PutUint16(h[4:6], uint16(yMin))
	binary.BigEndian.PutUint16(h[6:8], uint16(xMax))
	binary.BigEndian.PutUint16(h[8:10], uint16(yMax))
	buf = append(buf, h...)

	// endPtsOfContours
	for _, ep := range endPts {
		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, uint16(ep))
		buf = append(buf, b...)
	}

	// instructions
	il := make([]byte, 2)
	binary.BigEndian.PutUint16(il, uint16(len(instructions)))
	buf = append(buf, il...)
	buf = append(buf, instructions...)

	// 编码 flags 和坐标
	// 先计算 X deltas
	xDeltas := make([]int16, totalPoints)
	var prevX int16
	for i := 0; i < totalPoints; i++ {
		xDeltas[i] = xCoords[i] - prevX
		prevX = xCoords[i]
	}

	yDeltas := make([]int16, totalPoints)
	var prevY int16
	for i := 0; i < totalPoints; i++ {
		yDeltas[i] = yCoords[i] - prevY
		prevY = yCoords[i]
	}

	// 编码 flags
	newFlags := make([]byte, totalPoints)
	for i := 0; i < totalPoints; i++ {
		f := origFlags[i] & 0x01 // 保留 on-curve bit
		dx := xDeltas[i]
		dy := yDeltas[i]

		if dx == 0 {
			f |= 0x10 // x same
		} else if dx >= -255 && dx <= 255 {
			f |= 0x02 // x short
			if dx > 0 {
				f |= 0x10 // positive
			}
		}

		if dy == 0 {
			f |= 0x20 // y same
		} else if dy >= -255 && dy <= 255 {
			f |= 0x04 // y short
			if dy > 0 {
				f |= 0x20 // positive
			}
		}

		newFlags[i] = f
	}

	// 写入 flags（不使用 repeat 以简化）
	buf = append(buf, newFlags...)

	// 写入 X 坐标
	for i := 0; i < totalPoints; i++ {
		dx := xDeltas[i]
		if newFlags[i]&0x02 != 0 {
			// short
			if dx < 0 {
				dx = -dx
			}
			buf = append(buf, byte(dx))
		} else if newFlags[i]&0x10 == 0 {
			// word
			b := make([]byte, 2)
			binary.BigEndian.PutUint16(b, uint16(dx))
			buf = append(buf, b...)
		}
		// else: same as previous, no data
	}

	// 写入 Y 坐标
	for i := 0; i < totalPoints; i++ {
		dy := yDeltas[i]
		if newFlags[i]&0x04 != 0 {
			// short
			if dy < 0 {
				dy = -dy
			}
			buf = append(buf, byte(dy))
		} else if newFlags[i]&0x20 == 0 {
			// word
			b := make([]byte, 2)
			binary.BigEndian.PutUint16(b, uint16(dy))
			buf = append(buf, b...)
		}
	}

	return buf
}

// parseLocaTable 解析 loca 表
func parseLocaTable(locaData []byte, format int16, numGlyphs int) []uint32 {
	offsets := make([]uint32, 0, numGlyphs+1)
	if format == 0 {
		for i := 0; i <= numGlyphs; i++ {
			off := i * 2
			if off+2 > len(locaData) {
				break
			}
			offsets = append(offsets, uint32(binary.BigEndian.Uint16(locaData[off:off+2]))*2)
		}
	} else {
		for i := 0; i <= numGlyphs; i++ {
			off := i * 4
			if off+4 > len(locaData) {
				break
			}
			offsets = append(offsets, binary.BigEndian.Uint32(locaData[off:off+4]))
		}
	}
	return offsets
}

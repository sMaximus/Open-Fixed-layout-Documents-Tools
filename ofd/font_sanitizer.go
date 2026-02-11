package ofd

import (
	"encoding/binary"
	"fmt"
	"sort"
)

// SanitizeFont 修复字体文件的结构问题
func SanitizeFont(data []byte) ([]byte, error) {
	return sanitizeFontInternal(data, nil)
}

// sanitizeFontInternal 修复字体文件的结构问题
// 主要解决：
// 1. Table Directory 乱序 → 按 Tag 字母顺序重排
// 2. 缺少 OS/2 表 → 生成最小合规的 OS/2 表
// 3. 缺少 cmap 表 → 生成合规的 cmap 表（含 CGTransform 映射）
// 4. head.macStyle 与 OS/2.fsSelection 不一致 → 同步
// 5. 校验和错误 → 重新计算所有表的校验和
// 6. 如果提供了 glyphMappings，将 Unicode→GlyphID 映射注入 cmap 表
func sanitizeFontInternal(data []byte, glyphMappings []GlyphMapping) ([]byte, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("font data too short: %d bytes", len(data))
	}

	sfVersion := binary.BigEndian.Uint32(data[0:4])
	switch sfVersion {
	case 0x00010000, 0x4F54544F, 0x74727565:
		// TrueType / OpenType CFF / Apple TrueType
	default:
		return nil, fmt.Errorf("unsupported font format: 0x%08X", sfVersion)
	}

	numTables := int(binary.BigEndian.Uint16(data[4:6]))
	if numTables == 0 || numTables > 200 {
		return nil, fmt.Errorf("invalid numTables: %d", numTables)
	}

	headerSize := 12 + numTables*16
	if len(data) < headerSize {
		return nil, fmt.Errorf("font data truncated: need %d, got %d", headerSize, len(data))
	}

	// 解析所有表
	tables := make([]fontTable, 0, numTables)
	for i := 0; i < numTables; i++ {
		off := 12 + i*16
		tag := string(data[off : off+4])
		tableOffset := binary.BigEndian.Uint32(data[off+8 : off+12])
		length := binary.BigEndian.Uint32(data[off+12 : off+16])

		if int(tableOffset)+int(length) > len(data) {
			continue
		}

		tableData := make([]byte, length)
		copy(tableData, data[tableOffset:tableOffset+length])
		tables = append(tables, fontTable{tag: tag, data: tableData})
	}

	if len(tables) == 0 {
		return nil, fmt.Errorf("no valid tables found")
	}

	// 建立表索引
	tableMap := make(map[string]*fontTable)
	for i := range tables {
		tableMap[tables[i].tag] = &tables[i]
	}

	// 1. 同步 head.macStyle 与 OS/2.fsSelection
	if headT, ok := tableMap["head"]; ok {
		fixHeadMacStyle(headT, tableMap["OS/2"])
	}

	// 2. 补全缺失的 OS/2 表
	if _, ok := tableMap["OS/2"]; !ok {
		os2Data := generateOS2Table(tableMap["head"], tableMap["hhea"], tableMap["cmap"])
		t := fontTable{tag: "OS/2", data: os2Data}
		tables = append(tables, t)
		tableMap["OS/2"] = &tables[len(tables)-1]
	} else {
		// OS/2 已存在，确保 fsSelection 与 head.macStyle 一致
		syncOS2WithHead(tableMap["OS/2"], tableMap["head"])
	}

	// 3. 补全或重建 cmap 表
	if _, ok := tableMap["cmap"]; !ok {
		// cmap 缺失，生成新的
		cmapData := generateCmapFromMappings(glyphMappings, tableMap["head"])
		t := fontTable{tag: "cmap", data: cmapData}
		tables = append(tables, t)
		tableMap["cmap"] = &tables[len(tables)-1]
	} else if len(glyphMappings) > 0 {
		// cmap 存在但有 CGTransform 映射，将映射合并到现有 cmap
		tableMap["cmap"].data = mergeCmapMappings(tableMap["cmap"].data, glyphMappings)
	}

	// 4. 补全缺失的 name 表
	if _, ok := tableMap["name"]; !ok {
		nameData := generateMinimalName()
		t := fontTable{tag: "name", data: nameData}
		tables = append(tables, t)
		tableMap["name"] = &tables[len(tables)-1]
	}

	// 5. 补全缺失的 post 表
	if _, ok := tableMap["post"]; !ok {
		postData := generateMinimalPost()
		t := fontTable{tag: "post", data: postData}
		tables = append(tables, t)
		tableMap["post"] = &tables[len(tables)-1]
	}

	// 按 Tag 字母顺序排序
	sort.Slice(tables, func(i, j int) bool {
		return tables[i].tag < tables[j].tag
	})

	return serializeFont(sfVersion, tables), nil
}

type fontTable struct {
	tag       string
	data      []byte
	newOffset uint32
}

// fixHeadMacStyle 确保 head.macStyle 的值合理
// head 表偏移 44 处是 macStyle (uint16)
// bit 0 = Bold, bit 1 = Italic
func fixHeadMacStyle(headT *fontTable, os2T *fontTable) {
	if headT == nil || len(headT.data) < 46 {
		return
	}
	macStyle := binary.BigEndian.Uint16(headT.data[44:46])

	// 如果 OS/2 存在，从 OS/2.fsSelection 推导 macStyle
	if os2T != nil && len(os2T.data) >= 64 {
		fsSelection := binary.BigEndian.Uint16(os2T.data[62:64])
		var newMacStyle uint16
		if fsSelection&0x0020 != 0 { // Bold
			newMacStyle |= 0x0001
		}
		if fsSelection&0x0001 != 0 { // Italic
			newMacStyle |= 0x0002
		}
		if macStyle != newMacStyle {
			binary.BigEndian.PutUint16(headT.data[44:46], newMacStyle)
		}
		return
	}

	// 没有 OS/2，确保 macStyle 至少是合理的（不做修改）
	_ = macStyle
}

// syncOS2WithHead 同步已有的 OS/2.fsSelection 与 head.macStyle
func syncOS2WithHead(os2T, headT *fontTable) {
	if os2T == nil || len(os2T.data) < 64 || headT == nil || len(headT.data) < 46 {
		return
	}

	macStyle := binary.BigEndian.Uint16(headT.data[44:46])
	fsSelection := binary.BigEndian.Uint16(os2T.data[62:64])

	// 根据 macStyle 推导正确的 fsSelection
	var newFS uint16
	isBold := macStyle&0x0001 != 0
	isItalic := macStyle&0x0002 != 0

	if isBold {
		newFS |= 0x0020 // BOLD
	}
	if isItalic {
		newFS |= 0x0001 // ITALIC
	}
	if !isBold && !isItalic {
		newFS |= 0x0040 // REGULAR
	}
	// 保留 USE_TYPO_METRICS 位 (bit 7)
	newFS |= fsSelection & 0x0080

	if fsSelection != newFS {
		binary.BigEndian.PutUint16(os2T.data[62:64], newFS)
	}

	// 同时确保 head.macStyle 与 fsSelection 一致
	var expectedMacStyle uint16
	if newFS&0x0020 != 0 {
		expectedMacStyle |= 0x0001
	}
	if newFS&0x0001 != 0 {
		expectedMacStyle |= 0x0002
	}
	if macStyle != expectedMacStyle {
		binary.BigEndian.PutUint16(headT.data[44:46], expectedMacStyle)
	}
}

// generateCmapFromMappings 根据 GlyphMapping 生成 cmap 表
// 如果 mappings 为空，生成一个最小合规的 cmap
func generateCmapFromMappings(mappings []GlyphMapping, headT *fontTable) []byte {
	// 构建 Unicode→GlyphID 映射表
	charMap := make(map[uint16]uint16)
	for _, m := range mappings {
		if m.Unicode <= 0xFFFF && m.GlyphID > 0 {
			charMap[uint16(m.Unicode)] = m.GlyphID
		}
	}

	// 如果没有映射，至少映射 space
	if len(charMap) == 0 {
		charMap[0x0020] = 1
	}

	return buildCmapFormat4(charMap)
}

// mergeCmapMappings 将 GlyphMapping 合并到现有的 cmap 表
// 解析现有 cmap 的 Format 4 子表，添加新映射，重新生成
func mergeCmapMappings(cmapData []byte, mappings []GlyphMapping) []byte {
	if len(mappings) == 0 {
		return cmapData
	}

	// 从现有 cmap 提取映射
	existing := extractCmapFormat4Mappings(cmapData)

	// 合并新映射（CGTransform 的映射优先）
	for _, m := range mappings {
		if m.Unicode <= 0xFFFF && m.GlyphID > 0 {
			existing[uint16(m.Unicode)] = m.GlyphID
		}
	}

	return buildCmapFormat4(existing)
}

// extractCmapFormat4Mappings 从 cmap 表中提取 Format 4 的映射
func extractCmapFormat4Mappings(cmapData []byte) map[uint16]uint16 {
	result := make(map[uint16]uint16)

	if len(cmapData) < 4 {
		return result
	}

	numSub := int(binary.BigEndian.Uint16(cmapData[2:4]))
	for i := 0; i < numSub && 4+i*8+8 <= len(cmapData); i++ {
		off := 4 + i*8
		subOff := int(binary.BigEndian.Uint32(cmapData[off+4 : off+8]))
		if subOff+14 > len(cmapData) {
			continue
		}
		format := binary.BigEndian.Uint16(cmapData[subOff : subOff+2])
		if format != 4 {
			continue
		}

		segCount := int(binary.BigEndian.Uint16(cmapData[subOff+6:subOff+8])) / 2
		endStart := subOff + 14
		startStart := endStart + segCount*2 + 2
		deltaStart := startStart + segCount*2
		rangeStart := deltaStart + segCount*2

		for s := 0; s < segCount; s++ {
			eOff := endStart + s*2
			sOff := startStart + s*2
			dOff := deltaStart + s*2
			rOff := rangeStart + s*2

			if eOff+2 > len(cmapData) || sOff+2 > len(cmapData) ||
				dOff+2 > len(cmapData) || rOff+2 > len(cmapData) {
				break
			}

			endCode := binary.BigEndian.Uint16(cmapData[eOff : eOff+2])
			startCode := binary.BigEndian.Uint16(cmapData[sOff : sOff+2])
			idDelta := int16(binary.BigEndian.Uint16(cmapData[dOff : dOff+2]))
			idRangeOffset := binary.BigEndian.Uint16(cmapData[rOff : rOff+2])

			if startCode == 0xFFFF {
				continue
			}

			for c := startCode; c <= endCode; c++ {
				var gid uint16
				if idRangeOffset == 0 {
					gid = uint16(int16(c) + idDelta)
				} else {
					// glyphIndex = *(idRangeOffset/2 + (c - startCode) + &idRangeOffset)
					glyphOff := rOff + int(idRangeOffset) + int(c-startCode)*2
					if glyphOff+2 <= len(cmapData) {
						gid = binary.BigEndian.Uint16(cmapData[glyphOff : glyphOff+2])
						if gid != 0 {
							gid = uint16(int16(gid) + idDelta)
						}
					}
				}
				if gid != 0 {
					result[c] = gid
				}
			}
		}
		break // 只处理第一个 Format 4
	}

	return result
}

// buildCmapFormat4 从 Unicode→GlyphID 映射构建完整的 cmap 表
func buildCmapFormat4(charMap map[uint16]uint16) []byte {
	// 收集并排序所有码点
	codes := make([]uint16, 0, len(charMap))
	for c := range charMap {
		codes = append(codes, c)
	}
	sort.Slice(codes, func(i, j int) bool { return codes[i] < codes[j] })

	// 构建连续段
	type segment struct {
		start, end uint16
		delta      int16
	}
	var segments []segment

	if len(codes) > 0 {
		segStart := codes[0]
		segEnd := codes[0]
		// 检查是否可以用 delta 表示（连续的 GlyphID）
		canUseDelta := true
		baseDelta := int16(charMap[codes[0]]) - int16(codes[0])

		for i := 1; i < len(codes); i++ {
			c := codes[i]
			thisDelta := int16(charMap[c]) - int16(c)

			if c == segEnd+1 && thisDelta == baseDelta {
				// 连续且 delta 相同，扩展当前段
				segEnd = c
			} else {
				// 结束当前段
				if canUseDelta {
					segments = append(segments, segment{segStart, segEnd, baseDelta})
				} else {
					// 每个字符单独一段
					for cc := segStart; cc <= segEnd; cc++ {
						d := int16(charMap[cc]) - int16(cc)
						segments = append(segments, segment{cc, cc, d})
					}
				}
				segStart = c
				segEnd = c
				baseDelta = thisDelta
				canUseDelta = true
			}
		}
		// 最后一段
		if canUseDelta {
			segments = append(segments, segment{segStart, segEnd, baseDelta})
		} else {
			for cc := segStart; cc <= segEnd; cc++ {
				d := int16(charMap[cc]) - int16(cc)
				segments = append(segments, segment{cc, cc, d})
			}
		}
	}

	// 添加 sentinel 段 (0xFFFF)
	segments = append(segments, segment{0xFFFF, 0xFFFF, 1})

	segCount := len(segments)
	searchRange, entrySelector, rangeShift := calcCmapSearchParams(segCount)

	// Format 4 子表大小
	// header(14) + endCode(segCount*2) + reservedPad(2) + startCode(segCount*2) + idDelta(segCount*2) + idRangeOffset(segCount*2)
	subtableSize := 14 + 2 + segCount*8 // 16 + segCount*8
	totalSize := 12 + subtableSize       // cmap header(4) + encoding record(8) + subtable

	buf := make([]byte, totalSize)

	// cmap header
	binary.BigEndian.PutUint16(buf[0:2], 0) // version
	binary.BigEndian.PutUint16(buf[2:4], 1) // numTables

	// Encoding record: Platform 3 (Windows), Encoding 1 (Unicode BMP)
	binary.BigEndian.PutUint16(buf[4:6], 3)
	binary.BigEndian.PutUint16(buf[6:8], 1)
	binary.BigEndian.PutUint32(buf[8:12], 12) // offset

	// Format 4 subtable
	st := buf[12:]
	binary.BigEndian.PutUint16(st[0:2], 4)                       // format
	binary.BigEndian.PutUint16(st[2:4], uint16(subtableSize))    // length
	binary.BigEndian.PutUint16(st[4:6], 0)                       // language
	binary.BigEndian.PutUint16(st[6:8], uint16(segCount*2))      // segCountX2
	binary.BigEndian.PutUint16(st[8:10], searchRange)
	binary.BigEndian.PutUint16(st[10:12], entrySelector)
	binary.BigEndian.PutUint16(st[12:14], rangeShift)

	// endCode array
	endOff := 14
	for i, seg := range segments {
		binary.BigEndian.PutUint16(st[endOff+i*2:endOff+i*2+2], seg.end)
	}

	// reservedPad
	padOff := endOff + segCount*2
	binary.BigEndian.PutUint16(st[padOff:padOff+2], 0)

	// startCode array
	startOff := padOff + 2
	for i, seg := range segments {
		binary.BigEndian.PutUint16(st[startOff+i*2:startOff+i*2+2], seg.start)
	}

	// idDelta array
	deltaOff := startOff + segCount*2
	for i, seg := range segments {
		putInt16(st[deltaOff+i*2:deltaOff+i*2+2], seg.delta)
	}

	// idRangeOffset array (all zeros - we use delta for everything)
	// Already zero from make()

	return buf
}

func calcCmapSearchParams(segCount int) (searchRange, entrySelector, rangeShift uint16) {
	power := 1
	sel := 0
	for power*2 <= segCount {
		power *= 2
		sel++
	}
	searchRange = uint16(power * 2)
	entrySelector = uint16(sel)
	rangeShift = uint16(segCount*2) - searchRange
	return
}

// generateMinimalName 生成最小合规的 name 表
func generateMinimalName() []byte {
	// name 表需要至少包含 nameID 1 (Family) 和 nameID 2 (Subfamily)
	familyName := "Unknown"
	subfamilyName := "Regular"

	names := []struct {
		nameID uint16
		value  string
	}{
		{1, familyName},    // Font Family
		{2, subfamilyName}, // Font Subfamily
		{4, familyName},    // Full Name
		{6, familyName},    // PostScript Name
	}

	// 计算大小
	// header: 6 bytes
	// records: len(names) * 12 bytes each
	// string data follows
	headerSize := 6
	recordsSize := len(names) * 12
	storageOffset := headerSize + recordsSize

	var stringData []byte
	type nameRecord struct {
		platformID, encodingID, languageID, nameID, length, offset uint16
	}
	var records []nameRecord

	for _, n := range names {
		// Platform 1 (Mac), Encoding 0 (Roman), Language 0 (English)
		strBytes := []byte(n.value)
		records = append(records, nameRecord{
			platformID: 1, encodingID: 0, languageID: 0,
			nameID: n.nameID,
			length: uint16(len(strBytes)),
			offset: uint16(len(stringData)),
		})
		stringData = append(stringData, strBytes...)
	}

	totalSize := storageOffset + len(stringData)
	buf := make([]byte, totalSize)

	// Header
	binary.BigEndian.PutUint16(buf[0:2], 0)                       // format
	binary.BigEndian.PutUint16(buf[2:4], uint16(len(records)))    // count
	binary.BigEndian.PutUint16(buf[4:6], uint16(storageOffset))   // stringOffset

	// Records
	for i, r := range records {
		off := headerSize + i*12
		binary.BigEndian.PutUint16(buf[off:off+2], r.platformID)
		binary.BigEndian.PutUint16(buf[off+2:off+4], r.encodingID)
		binary.BigEndian.PutUint16(buf[off+4:off+6], r.languageID)
		binary.BigEndian.PutUint16(buf[off+6:off+8], r.nameID)
		binary.BigEndian.PutUint16(buf[off+8:off+10], r.length)
		binary.BigEndian.PutUint16(buf[off+10:off+12], r.offset)
	}

	// String data
	copy(buf[storageOffset:], stringData)

	return buf
}

// generateMinimalPost 生成最小合规的 post 表 (Format 3 = 无字形名称)
func generateMinimalPost() []byte {
	buf := make([]byte, 32)
	// Format 3.0 (0x00030000) - 不包含字形名称
	binary.BigEndian.PutUint32(buf[0:4], 0x00030000)
	// italicAngle = 0 (Fixed 16.16)
	binary.BigEndian.PutUint32(buf[4:8], 0)
	// underlinePosition = -100
	putInt16(buf[8:10], -100)
	// underlineThickness = 50
	putInt16(buf[10:12], 50)
	// isFixedPitch = 0
	binary.BigEndian.PutUint32(buf[12:16], 0)
	// minMemType42, maxMemType42, minMemType1, maxMemType1 = 0
	return buf
}

// serializeFont 将字体表重新序列化为完整的字体文件
func serializeFont(sfVersion uint32, tables []fontTable) []byte {
	numTables := len(tables)
	searchRange, entrySelector, rangeShift := calcSearchParams(numTables)

	headerSize := 12 + numTables*16
	dataOffset := headerSize
	for i := range tables {
		padded := (len(tables[i].data) + 3) & ^3
		tables[i].newOffset = uint32(dataOffset)
		dataOffset += padded
	}

	buf := make([]byte, dataOffset)

	// Offset Table
	binary.BigEndian.PutUint32(buf[0:4], sfVersion)
	binary.BigEndian.PutUint16(buf[4:6], uint16(numTables))
	binary.BigEndian.PutUint16(buf[6:8], searchRange)
	binary.BigEndian.PutUint16(buf[8:10], entrySelector)
	binary.BigEndian.PutUint16(buf[10:12], rangeShift)

	// Table Records + Data
	for i, t := range tables {
		copy(buf[t.newOffset:], t.data)
		checksum := calcTableChecksum(buf[t.newOffset : t.newOffset+uint32(len(t.data))])

		recOff := 12 + i*16
		copy(buf[recOff:recOff+4], []byte(t.tag))
		binary.BigEndian.PutUint32(buf[recOff+4:recOff+8], checksum)
		binary.BigEndian.PutUint32(buf[recOff+8:recOff+12], t.newOffset)
		binary.BigEndian.PutUint32(buf[recOff+12:recOff+16], uint32(len(t.data)))
	}

	fixHeadChecksum(buf, tables)
	return buf
}

func calcTableChecksum(data []byte) uint32 {
	padded := data
	if rem := len(data) % 4; rem != 0 {
		padded = make([]byte, len(data)+4-rem)
		copy(padded, data)
	}
	var sum uint32
	for i := 0; i < len(padded); i += 4 {
		sum += binary.BigEndian.Uint32(padded[i : i+4])
	}
	return sum
}

func fixHeadChecksum(buf []byte, tables []fontTable) {
	for _, t := range tables {
		if t.tag == "head" && len(t.data) >= 12 {
			headOff := t.newOffset
			binary.BigEndian.PutUint32(buf[headOff+8:headOff+12], 0)
			fileChecksum := calcTableChecksum(buf)
			binary.BigEndian.PutUint32(buf[headOff+8:headOff+12], 0xB1B0AFBA-fileChecksum)
			return
		}
	}
}

func calcSearchParams(numTables int) (searchRange, entrySelector, rangeShift uint16) {
	power := 1
	sel := 0
	for power*2 <= numTables {
		power *= 2
		sel++
	}
	searchRange = uint16(power * 16)
	entrySelector = uint16(sel)
	rangeShift = uint16(numTables*16) - searchRange
	return
}

func putInt16(b []byte, v int16) {
	binary.BigEndian.PutUint16(b, uint16(v))
}

// generateOS2Table 生成最小合规的 OS/2 表 (Version 4, 96 bytes)
func generateOS2Table(headT, hheaT, cmapT *fontTable) []byte {
	buf := make([]byte, 96)

	// Version = 4
	binary.BigEndian.PutUint16(buf[0:2], 4)
	// xAvgCharWidth
	binary.BigEndian.PutUint16(buf[2:4], 500)
	// usWeightClass = 400 (Regular)
	binary.BigEndian.PutUint16(buf[4:6], 400)
	// usWidthClass = 5 (Medium)
	binary.BigEndian.PutUint16(buf[6:8], 5)
	// fsType = 0 (Installable)
	binary.BigEndian.PutUint16(buf[8:10], 0)

	unitsPerEm := uint16(1000)
	if headT != nil && len(headT.data) >= 20 {
		unitsPerEm = binary.BigEndian.Uint16(headT.data[18:20])
	}

	sub := unitsPerEm * 65 / 100
	binary.BigEndian.PutUint16(buf[10:12], sub) // ySubscriptXSize
	binary.BigEndian.PutUint16(buf[12:14], sub) // ySubscriptYSize
	binary.BigEndian.PutUint16(buf[14:16], 0)
	putInt16(buf[16:18], int16(unitsPerEm*14/100))
	binary.BigEndian.PutUint16(buf[18:20], sub) // ySuperscriptXSize
	binary.BigEndian.PutUint16(buf[20:22], sub) // ySuperscriptYSize
	binary.BigEndian.PutUint16(buf[22:24], 0)
	putInt16(buf[24:26], int16(unitsPerEm*48/100))
	putInt16(buf[26:28], int16(unitsPerEm*5/100))  // yStrikeoutSize
	putInt16(buf[28:30], int16(unitsPerEm*26/100)) // yStrikeoutPosition
	binary.BigEndian.PutUint16(buf[30:32], 0)      // sFamilyClass

	// panose[10] = all zeros (buf[32:42])

	// ulUnicodeRange1-4
	binary.BigEndian.PutUint32(buf[42:46], 0x00000003) // Basic Latin + Latin-1
	binary.BigEndian.PutUint32(buf[46:50], 0x10000000) // CJK
	binary.BigEndian.PutUint32(buf[50:54], 0)
	binary.BigEndian.PutUint32(buf[54:58], 0)

	if cmapT != nil {
		ur1, ur2, ur3, ur4 := detectUnicodeRanges(cmapT.data)
		if ur1 != 0 || ur2 != 0 || ur3 != 0 || ur4 != 0 {
			binary.BigEndian.PutUint32(buf[42:46], ur1)
			binary.BigEndian.PutUint32(buf[46:50], ur2)
			binary.BigEndian.PutUint32(buf[50:54], ur3)
			binary.BigEndian.PutUint32(buf[54:58], ur4)
		}
	}

	copy(buf[58:62], []byte("    ")) // achVendID

	// fsSelection: 从 head.macStyle 推导，确保一致
	var fsSelection uint16 = 0x0040 // Regular
	if headT != nil && len(headT.data) >= 46 {
		macStyle := binary.BigEndian.Uint16(headT.data[44:46])
		fsSelection = 0
		if macStyle&0x0001 != 0 { // Bold
			fsSelection |= 0x0020
		}
		if macStyle&0x0002 != 0 { // Italic
			fsSelection |= 0x0001
		}
		if fsSelection == 0 {
			fsSelection = 0x0040 // Regular
		}
	}
	binary.BigEndian.PutUint16(buf[62:64], fsSelection)

	// usFirstCharIndex, usLastCharIndex
	firstChar, lastChar := getCharRange(cmapT)
	binary.BigEndian.PutUint16(buf[64:66], firstChar)
	binary.BigEndian.PutUint16(buf[66:68], lastChar)

	// sTypoAscender, sTypoDescender, sTypoLineGap
	var asc, desc, gap int16
	if hheaT != nil && len(hheaT.data) >= 10 {
		asc = int16(binary.BigEndian.Uint16(hheaT.data[4:6]))
		desc = int16(binary.BigEndian.Uint16(hheaT.data[6:8]))
		gap = int16(binary.BigEndian.Uint16(hheaT.data[8:10]))
	} else {
		asc = int16(unitsPerEm * 80 / 100)
		desc = -int16(unitsPerEm * 20 / 100)
	}
	putInt16(buf[68:70], asc)
	putInt16(buf[70:72], desc)
	putInt16(buf[72:74], gap)

	// usWinAscent, usWinDescent
	winAsc := asc
	if winAsc < 0 {
		winAsc = -winAsc
	}
	winDesc := desc
	if winDesc > 0 {
		winDesc = -winDesc
	}
	binary.BigEndian.PutUint16(buf[74:76], uint16(winAsc))
	binary.BigEndian.PutUint16(buf[76:78], uint16(-winDesc))

	// ulCodePageRange1-2
	binary.BigEndian.PutUint32(buf[78:82], 0x00000001)
	binary.BigEndian.PutUint32(buf[82:86], 0)

	// sxHeight, sCapHeight
	putInt16(buf[86:88], int16(unitsPerEm*50/100))
	putInt16(buf[88:90], int16(unitsPerEm*70/100))

	// usDefaultChar = 0
	binary.BigEndian.PutUint16(buf[90:92], 0)
	// usBreakChar = space
	binary.BigEndian.PutUint16(buf[92:94], 0x0020)
	// usMaxContext = 1
	binary.BigEndian.PutUint16(buf[94:96], 1)

	return buf
}

func getCharRange(cmapT *fontTable) (first, last uint16) {
	if cmapT == nil || len(cmapT.data) < 4 {
		return 0x0020, 0xFFFF
	}

	data := cmapT.data
	numSub := int(binary.BigEndian.Uint16(data[2:4]))

	for i := 0; i < numSub && 4+i*8+8 <= len(data); i++ {
		off := 4 + i*8
		subOff := int(binary.BigEndian.Uint32(data[off+4 : off+8]))
		if subOff+14 > len(data) {
			continue
		}
		format := binary.BigEndian.Uint16(data[subOff : subOff+2])
		if format != 4 {
			continue
		}

		segCount := int(binary.BigEndian.Uint16(data[subOff+6:subOff+8])) / 2
		endStart := subOff + 14
		startStart := endStart + segCount*2 + 2

		if startStart+segCount*2 > len(data) || endStart+segCount*2 > len(data) {
			continue
		}

		firstCode := binary.BigEndian.Uint16(data[startStart : startStart+2])
		if firstCode == 0 && segCount > 1 && startStart+4 <= len(data) {
			firstCode = binary.BigEndian.Uint16(data[startStart+2 : startStart+4])
		}

		lastIdx := segCount - 1
		lastCode := binary.BigEndian.Uint16(data[endStart+lastIdx*2 : endStart+lastIdx*2+2])
		if lastCode == 0xFFFF && segCount > 1 {
			lastCode = binary.BigEndian.Uint16(data[endStart+(lastIdx-1)*2 : endStart+(lastIdx-1)*2+2])
		}

		return firstCode, lastCode
	}

	return 0x0020, 0xFFFF
}

func detectUnicodeRanges(cmapData []byte) (ur1, ur2, ur3, ur4 uint32) {
	if len(cmapData) < 4 {
		return
	}

	numSub := int(binary.BigEndian.Uint16(cmapData[2:4]))
	for i := 0; i < numSub && 4+i*8+8 <= len(cmapData); i++ {
		off := 4 + i*8
		subOff := int(binary.BigEndian.Uint32(cmapData[off+4 : off+8]))
		if subOff+14 > len(cmapData) {
			continue
		}
		if binary.BigEndian.Uint16(cmapData[subOff:subOff+2]) != 4 {
			continue
		}

		segCount := int(binary.BigEndian.Uint16(cmapData[subOff+6:subOff+8])) / 2
		endStart := subOff + 14
		startStart := endStart + segCount*2 + 2

		for s := 0; s < segCount; s++ {
			sOff := startStart + s*2
			eOff := endStart + s*2
			if sOff+2 > len(cmapData) || eOff+2 > len(cmapData) {
				break
			}
			sc := binary.BigEndian.Uint16(cmapData[sOff : sOff+2])
			ec := binary.BigEndian.Uint16(cmapData[eOff : eOff+2])
			setUnicodeRangeBits(sc, ec, &ur1, &ur2, &ur3, &ur4)
		}
		break
	}
	return
}

func setUnicodeRangeBits(start, end uint16, ur1, ur2, ur3, ur4 *uint32) {
	type rm struct {
		lo, hi uint16
		reg    int
		bit    uint
	}
	ranges := []rm{
		{0x0000, 0x007F, 0, 0}, {0x0080, 0x00FF, 0, 1},
		{0x0100, 0x017F, 0, 2}, {0x0180, 0x024F, 0, 3},
		{0x2000, 0x206F, 0, 9}, {0x3000, 0x303F, 1, 16},
		{0x3040, 0x309F, 1, 17}, {0x30A0, 0x30FF, 1, 18},
		{0x4E00, 0x9FFF, 1, 27}, {0xF900, 0xFAFF, 1, 29},
		{0xFF00, 0xFFEF, 1, 31},
	}
	for _, r := range ranges {
		if start <= r.hi && end >= r.lo {
			switch r.reg {
			case 0:
				*ur1 |= 1 << r.bit
			case 1:
				*ur2 |= 1 << r.bit
			case 2:
				*ur3 |= 1 << r.bit
			case 3:
				*ur4 |= 1 << r.bit
			}
		}
	}
}

// IsFontData 检测数据是否为字体文件
func IsFontData(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	sig := binary.BigEndian.Uint32(data[0:4])
	return sig == 0x00010000 || sig == 0x4F54544F || sig == 0x74727565
}

// NeedsSanitization 检测字体是否需要修复
func NeedsSanitization(data []byte) bool {
	if len(data) < 12 {
		return false
	}

	numTables := int(binary.BigEndian.Uint16(data[4:6]))
	if numTables == 0 || 12+numTables*16 > len(data) {
		return true
	}

	hasOS2 := false
	hasCmap := false
	hasName := false
	hasPost := false
	prevTag := ""
	ordered := true

	var headMacStyle uint16
	var os2FSSelection uint16
	hasHead := false
	headOS2Mismatch := false

	for i := 0; i < numTables; i++ {
		off := 12 + i*16
		if off+16 > len(data) {
			return true
		}
		tag := string(data[off : off+4])
		tableOffset := binary.BigEndian.Uint32(data[off+8 : off+12])
		tableLen := binary.BigEndian.Uint32(data[off+12 : off+16])

		switch tag {
		case "OS/2":
			hasOS2 = true
			if int(tableOffset)+64 <= len(data) && tableLen >= 64 {
				os2FSSelection = binary.BigEndian.Uint16(data[tableOffset+62 : tableOffset+64])
			}
		case "cmap":
			hasCmap = true
		case "name":
			hasName = true
		case "post":
			hasPost = true
		case "head":
			hasHead = true
			if int(tableOffset)+46 <= len(data) && tableLen >= 46 {
				headMacStyle = binary.BigEndian.Uint16(data[tableOffset+44 : tableOffset+46])
			}
		}

		if tag < prevTag {
			ordered = false
		}
		prevTag = tag
	}

	// 检查 head.macStyle 与 OS/2.fsSelection 一致性
	if hasHead && hasOS2 {
		isBold := headMacStyle&0x0001 != 0
		isItalic := headMacStyle&0x0002 != 0
		isRegular := !isBold && !isItalic

		os2Bold := os2FSSelection&0x0020 != 0
		os2Italic := os2FSSelection&0x0001 != 0
		os2Regular := os2FSSelection&0x0040 != 0

		if isBold != os2Bold || isItalic != os2Italic || isRegular != os2Regular {
			headOS2Mismatch = true
		}
	}

	return !ordered || !hasOS2 || !hasCmap || !hasName || !hasPost || headOS2Mismatch
}

// SanitizeFontIfNeeded 仅在需要时修复字体
func SanitizeFontIfNeeded(data []byte) []byte {
	if !IsFontData(data) {
		return data
	}
	fixed, err := SanitizeFont(data)
	if err != nil {
		return data
	}
	return fixed
}

// SanitizeFontWithMappings 修复字体并注入 CGTransform 的 GlyphID 映射
// 只在字体结构有问题或有映射需要注入时才处理，否则原样返回
func SanitizeFontWithMappings(data []byte, mappings []GlyphMapping) []byte {
	if !IsFontData(data) {
		return data
	}
	// 没有映射且字体结构正常，直接返回原始数据
	if len(mappings) == 0 && !NeedsSanitization(data) {
		return data
	}
	fixed, err := sanitizeFontInternal(data, mappings)
	if err != nil {
		return data
	}
	return fixed
}

// FontIsBold 检测字体文件是否本身就是 Bold 字体
// 通过读取 OS/2 表的 usWeightClass 和 fsSelection，以及 head 表的 macStyle 来判断
func FontIsBold(data []byte) bool {
	if len(data) < 12 {
		return false
	}

	numTables := int(binary.BigEndian.Uint16(data[4:6]))
	if numTables == 0 || 12+numTables*16 > len(data) {
		return false
	}

	for i := 0; i < numTables; i++ {
		off := 12 + i*16
		if off+16 > len(data) {
			break
		}
		tag := string(data[off : off+4])
		tableOffset := int(binary.BigEndian.Uint32(data[off+8 : off+12]))
		tableLen := int(binary.BigEndian.Uint32(data[off+12 : off+16]))

		if tag == "OS/2" && tableOffset+6 <= len(data) && tableLen >= 6 {
			weightClass := binary.BigEndian.Uint16(data[tableOffset+4 : tableOffset+6])
			// usWeightClass >= 700 表示 Bold
			if weightClass >= 700 {
				return true
			}
			// 也检查 fsSelection bit 5 (BOLD)
			if tableLen >= 64 && tableOffset+64 <= len(data) {
				fsSelection := binary.BigEndian.Uint16(data[tableOffset+62 : tableOffset+64])
				if fsSelection&0x0020 != 0 {
					return true
				}
			}
		}

		if tag == "head" && tableOffset+46 <= len(data) && tableLen >= 46 {
			macStyle := binary.BigEndian.Uint16(data[tableOffset+44 : tableOffset+46])
			if macStyle&0x0001 != 0 {
				return true
			}
		}
	}

	return false
}

// EmboldenFont is now in font_embolden.go with real glyph-level emboldening

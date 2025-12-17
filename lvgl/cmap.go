package lvgl

import "encoding/binary"

type CmapTable struct {
	Size   uint32  //4	Record size (for quick skip)
	Label  [4]byte //4	head (table marker)
	Tables uint32  //4	Number of additional tables (2 bytes to simplify align)
	//SubTables     []CmapSubTableHeader
	//SubTablesData []any
}

type CmapSubTableHeader struct {
	DataOffset       uint32 //4	Data offset (or 0 if data segment not exists)
	RangeStart       uint32 //4	Range start (min codePoint)
	RangeLength      uint16 //2	Range length (up to 65535)
	GlyphIdOffset    uint16 //2	Glyph ID offset (for delta-coding)
	DataEntriesCount uint16 //2	Data entries count (for sparse data)
	FormatType       byte   //1	Format type (0 => format 0, 1 => format sparse, 2 => format 0 tiny, 3 => format sparse tiny)
	Blank            byte   //1	- (align to 4)
}

func (c *CmapSubTableHeader) Size() int {
	return binary.Size(c)
}

type CmapFormat0Data []uint8

type CmapSparseData struct {
	CodeDeltas  []uint16 // codePoint - range_start
	GlyphDeltas []uint16 // delta-encoded glyph IDs（下一条我补充解释）
}

type CmapSparseTinyData []uint16 // 只存 codePoint - range_start

func NewCmapTable(runes []rune) (*CmapTable, []CmapSubTableHeader, []uint16) {

	tableRunes := CmapSplitSubTable(runes)
	t := &CmapTable{
		Size:   0,
		Label:  [4]byte{'c', 'm', 'a', 'p'},
		Tables: uint32(len(tableRunes)),
	}
	cmapDataOffset := binary.Size(t)
	subHeaders := make([]CmapSubTableHeader, t.Tables)
	for i, subRunes := range tableRunes {
		subHeaders[i] = CmapSubTableHeader{
			RangeStart:       uint32(subRunes[0]),
			RangeLength:      uint16(subRunes[len(subRunes)-1] - subRunes[0] + 1),
			GlyphIdOffset:    1,
			DataEntriesCount: uint16(len(subRunes)),
			FormatType:       3,
		}
		if i > 0 {
			subHeaders[i].GlyphIdOffset = subHeaders[i-1].GlyphIdOffset + subHeaders[i-1].DataEntriesCount
		}
	}
	cmapDataOffset += binary.Size(subHeaders)
	subDatas := make(CmapSparseTinyData, 0)
	for i, subRunes := range tableRunes {
		subHeaders[i].DataOffset = uint32(cmapDataOffset)
		subData := make([]uint16, 0)
		for i2 := range subRunes {
			subData = append(subData, uint16(subRunes[i2]-subRunes[0]))
		}
		if blank := ((len(subData) * 2) % 4) / 2; blank != 0 {
			subData = append(subData, make(CmapSparseTinyData, blank)...)
		}
		cmapDataOffset += len(subData) * 2
		subDatas = append(subDatas, subData...)
	}
	t.Size = uint32(cmapDataOffset)
	return t, subHeaders, subDatas
}

func CmapSplitSubTable(runes []rune) [][]rune {
	startRune := runes[0]
	item := make([]rune, 0)
	resp := make([][]rune, 0)
	for i := range runes {
		if runes[i]-startRune >= 65535 {
			resp = append(resp, item)
			item = make([]rune, 0)
			startRune = runes[i]
		}
		item = append(item, runes[i])
	}
	if len(item) > 0 {
		resp = append(resp, item)
	}
	return resp
}

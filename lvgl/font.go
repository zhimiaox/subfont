package lvgl

import (
	"bytes"
	"encoding/binary"
	"log/slog"
	"slices"

	"golang.org/x/image/font/sfnt"
)

type Font struct {
	*HeadTable
	*CmapTable
	*LocaTable
	*GlyfTable
}

func NewFont(pf *sfnt.Font, size uint16, runes []rune) ([]byte, error) {
	if len(runes) == 0 {
		return nil, nil
	}
	slices.Sort(runes)
	runes = slices.Compact(runes)
	slog.Info(string(runes))
	f := new(Font)
	f.HeadTable = NewHeadTable(pf, size)
	cmapTable, cmapSubHeaders, cmapSubData := NewCmapTable(runes)
	f.CmapTable = cmapTable
	f.LocaTable = NewLocaTable()
	f.LocaTable.EntryCount = uint32(len(runes) + 1)
	f.GlyfTable = NewGlyfTable()
	sfntBuf := &sfnt.Buffer{}
	bitmap := make([][]byte, len(runes))
	bitmapSize := int(f.GlyfTable.Size)
	locaOffset := []uint32{
		uint32(bitmapSize), uint32(bitmapSize),
	}
	ascent, descent := 0, 0
	for i, r := range runes {
		if glyfData, err := AddGlyfData(sfntBuf, pf, size, r); err == nil {
			bitmap[i] = glyfData.Bytes()
			if i == 0 {
				ascent, descent = int(glyfData.BBoxY)+int(glyfData.BBoxHeight), int(glyfData.BBoxY)
			} else {
				ascent, descent = max(ascent, int(glyfData.BBoxY)+int(glyfData.BBoxHeight)), min(descent, int(glyfData.BBoxY))
			}
		} else {
			slog.Error("字体数据生成失败", "r", string(r), "glyfData", glyfData, "err", err)
		}
		bitmapSize += len(bitmap[i])
		locaOffset = append(locaOffset, uint32(bitmapSize))
	}
	f.HeadTable.Ascent, f.HeadTable.Descent = uint16(ascent), int16(descent)
	f.HeadTable.MaxY, f.HeadTable.MinY = int16(ascent), int16(descent)
	f.LocaTable.Size += uint32(len(locaOffset) * 4)
	f.GlyfTable.Size += uint32(bitmapSize)
	binBuf := &bytes.Buffer{}
	if err := binary.Write(binBuf, binary.LittleEndian, f.HeadTable); err != nil {
		slog.Error("Error encoding HeadTable", "err", err)
	}
	if err := binary.Write(binBuf, binary.LittleEndian, f.CmapTable); err != nil {
		slog.Error("Error encoding CmapTable", "err", err)
	}
	if err := binary.Write(binBuf, binary.LittleEndian, cmapSubHeaders); err != nil {
		slog.Error("Error encoding CmapTable sub header", "err", err)
	}
	if err := binary.Write(binBuf, binary.LittleEndian, cmapSubData); err != nil {
		slog.Error("Error encoding CmapTable sub data", "err", err)
	}
	if err := binary.Write(binBuf, binary.LittleEndian, f.LocaTable); err != nil {
		slog.Error("Error encoding LocaTable", "err", err)
	}
	if err := binary.Write(binBuf, binary.LittleEndian, locaOffset); err != nil {
		slog.Error("Error encoding LocaTable data", "err", err)
	}
	if err := binary.Write(binBuf, binary.LittleEndian, f.GlyfTable); err != nil {
		slog.Error("Error encoding GlyfTable", "err", err)
	}
	for i := range bitmap {
		binBuf.Write(bitmap[i])
	}
	return binBuf.Bytes(), nil
}

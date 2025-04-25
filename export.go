/*
 * This file is subject to the terms and conditions defined in
 * file 'LICENSE.md', which is part of this source code package.
 */

package subfont

import (
	"bytes"
	"io"
	"log/slog"
	"math"
	"os"
	"slices"
)

// Font wraps font for outside access.
type Font struct {
	br *byteReader
	*font
}

// Parse parses the truetype font from `rs` and returns a new Font.
func Parse(rs io.ReadSeeker) (*Font, error) {
	r := newByteReader(rs)

	fnt, err := parseFont(r)
	if err != nil {
		return nil, err
	}

	return &Font{
		br:   r,
		font: fnt,
	}, nil
}

// ParseFile parses the truetype font from file given by path.
func ParseFile(filePath string) (*Font, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	defer f.Close()
	return Parse(f)
}

// ValidateBytes validates the turetype font represented by the byte stream.
func ValidateBytes(b []byte) error {
	r := bytes.NewReader(b)
	br := newByteReader(r)
	fnt, err := parseFont(br)
	if err != nil {
		return err
	}

	return fnt.validate(br)
}

// GetCmap returns the specific cmap specified by `platformID` and platform-specific `encodingID`.
// If not available, nil is returned. Used in PDF for decoding.
func (f *Font) GetCmap(platformID, encodingID int) map[rune]GlyphIndex {
	if f.cmap == nil {
		return nil
	}

	for _, subt := range f.cmap.subtables {
		if subt.platformID == platformID && subt.encodingID == encodingID {
			return subt.cmap
		}
	}

	return nil
}

// LookupRunes looks up each rune in `rune` and returns a matching slice of glyph indices.
// When a rune is not found, a GID of 0 is used (notdef).
func (f *Font) LookupRunes(runes []rune) ([]GlyphIndex, []rune) {
	slices.Sort(runes)
	runes = slices.Compact(runes)
	// Search order (3,1), (1,0), (0,3), (3,10).
	cmaps := []map[rune]GlyphIndex{
		f.GetCmap(3, 1),
		f.GetCmap(1, 0),
		f.GetCmap(0, 3),
		f.GetCmap(3, 10),
	}
	indices := make([]GlyphIndex, 0)
	searchRunes := make([]rune, 0)
	missRunes := make([]rune, 0)
	for _, r := range runes {
		has := false
		for _, cmap := range cmaps {
			if cmap == nil {
				continue
			}
			if ind, ok := cmap[r]; ok {
				indices = append(indices, ind)
				searchRunes = append(searchRunes, r)
				has = true
				break
			}
		}
		if !has {
			missRunes = append(missRunes, r)
		}
	}
	if len(missRunes) > 0 {
		slog.Warn("LookupRunes missing some runes", "runes", string(missRunes), "runes_raw", missRunes)
	}
	return indices, searchRunes
}

// Subset creates a subset of `f` including only glyph indices specified by `indices`.
// Returns the new subsetted font, a map of old to new GlyphIndex to GlyphIndex as the removal
// of glyphs requires reordering.
func (f *Font) Subset(runes []rune) (*Font, error) {
	indices, runes := f.LookupRunes(runes)
	if len(indices) == 0 || indices[1] != 0 {
		indices = slices.Insert(indices, 0, 0)
	}
	newfnt := font{}

	newfnt.ot = new(offsetTable)
	*newfnt.ot = *f.font.ot

	newfnt.trec = new(tableRecords)
	*newfnt.trec = *f.font.trec

	if f.font.cmap != nil {
		newfnt.cmap = &cmapTable{
			version:   f.cmap.version,
			subtables: make(map[string]*cmapSubtable),
		}
		for _, name := range f.cmap.subtableKeys {
			oldSubt := f.cmap.subtables[name]
			newSubt := &cmapSubtable{
				format:        oldSubt.format,
				platformID:    oldSubt.platformID,
				encodingID:    oldSubt.encodingID,
				ctx:           oldSubt.ctx,
				cmap:          make(map[rune]GlyphIndex),
				runes:         runes,
				charcodes:     make([]CharCode, 0),
				charcodeToGID: make(map[CharCode]GlyphIndex),
			}
			for gid, cc := range runes {
				newSubt.cmap[cc] = GlyphIndex(gid + 1)
				newSubt.charcodeToGID[CharCode(cc)] = GlyphIndex(gid + 1)
				newSubt.charcodes = append(newSubt.charcodes, CharCode(cc))
			}
			switch t := oldSubt.ctx.(type) {
			case cmapSubtableFormat4:
				newt := cmapSubtableFormat4{}
				segments := 0
				i := 0
				for i < len(newSubt.charcodes) {
					j := i + 1
					for ; j < len(newSubt.charcodes); j++ {
						if int(newSubt.charcodes[j]-newSubt.charcodes[i]) != j-i ||
							int(newSubt.charcodeToGID[newSubt.charcodes[j]]-newSubt.charcodeToGID[newSubt.charcodes[i]]) != j-i {
							break
						}
					}
					// from i:j-1 maps to subt.charcodes[i]:subt.charcodes[i]+j-i-1
					startCode := uint16(newSubt.charcodes[i])
					endCode := uint16(newSubt.charcodes[i]) + uint16(j-i-1)
					idDelta := uint16(newSubt.charcodeToGID[newSubt.charcodes[i]]) - uint16(newSubt.charcodes[i])

					newt.startCode = append(newt.startCode, startCode)
					newt.endCode = append(newt.endCode, endCode)
					newt.idDelta = append(newt.idDelta, idDelta)
					newt.idRangeOffset = append(newt.idRangeOffset, 0)
					segments++
					i = j
				}

				if segments > 0 && newt.endCode[segments-1] < 0xFFFF {
					newt.endCode = append(newt.endCode, 0xFFFF)
					newt.startCode = append(newt.startCode, 0xFFFF)
					newt.idDelta = append(newt.idDelta, 1)
					newt.idRangeOffset = append(newt.idRangeOffset, 0)
					segments++
				}

				newt.length = uint16(2*8 + 2*4*segments)
				newt.language = t.language
				newt.segCountX2 = uint16(segments * 2)
				newt.searchRange = 2 * uint16(math.Pow(2, math.Floor(math.Log2(float64(segments)))))
				newt.entrySelector = uint16(math.Log2(float64(newt.searchRange) / 2.0))
				newt.rangeShift = uint16(segments*2) - newt.searchRange
				newSubt.ctx = newt
			case cmapSubtableFormat12:
				newt := cmapSubtableFormat12{}
				groups := 0
				i := 0
				for i < len(newSubt.charcodes) {
					j := i + 1
					for ; j < len(newSubt.charcodes); j++ {
						if int(newSubt.charcodes[j]-newSubt.charcodes[i]) != j-i ||
							int(newSubt.charcodeToGID[newSubt.charcodes[j]]-newSubt.charcodeToGID[newSubt.charcodes[i]]) != j-i {
							break
						}
					}
					// from i:j-1 maps to subt.charcodes[i]:subt.charcodes[i]+j-i-1
					startCharCode := uint32(newSubt.charcodes[i])
					endCharCode := uint32(newSubt.charcodes[i]) + uint32(j-i-1)
					startGlyphID := uint32(newSubt.charcodeToGID[newSubt.charcodes[i]])

					group := sequentialMapGroup{
						startCharCode: startCharCode,
						endCharCode:   endCharCode,
						startGlyphID:  startGlyphID,
					}
					newt.groups = append(newt.groups, group)
					groups++
					i = j
				}
				newt.length = uint32(2*2 + 3*4 + groups*3*4)
				newt.language = t.language
				newt.numGroups = uint32(groups)
				newSubt.ctx = newt
			}
			newfnt.cmap.subtableKeys = append(newfnt.cmap.subtableKeys, name)
			newfnt.cmap.subtables[name] = newSubt
		}
		newfnt.cmap.numTables = uint16(len(newfnt.cmap.subtables))
	}

	// if f.font.name != nil {
	// 	newfnt.name = &nameTable{}
	// 	*newfnt.name = *f.font.name
	// 	for i, record := range newfnt.name.nameRecords {
	// 		record.data = []byte{0}
	// 		record.offset = offset16(i)
	// 		record.length = 1
	// 	}
	// }

	// if f.font.os2 != nil {
	// 	newfnt.os2 = &os2Table{}
	// 	*newfnt.os2 = *f.font.os2
	// }

	// if f.font.post != nil {
	// 	newfnt.post = &postTable{}
	// 	*newfnt.post = *f.font.post
	// 	if newfnt.post.numGlyphs > 0 {
	// 		newfnt.post.numGlyphs = uint16(numGlyphs)
	// 	}
	// 	if len(newfnt.post.glyphNameIndex) > numGlyphs {
	// 		glyphNameIndex := make([]uint16, 0)
	// 		for gid := range indices {
	// 			glyphNameIndex = append(glyphNameIndex, uint16(gid))
	// 		}
	// 		newfnt.post.glyphNameIndex = glyphNameIndex
	// 	}
	// 	if len(newfnt.post.offsets) > numGlyphs {
	// 		newfnt.post.offsets = newfnt.post.offsets[0:numGlyphs]
	// 	}
	// 	if len(newfnt.post.glyphNames) > numGlyphs {
	// 		names := make([]GlyphName, 0)
	// 		for _, gid := range indices {
	// 			names = append(names, f.font.post.glyphNames[gid])
	// 		}
	// 		newfnt.post.glyphNames = names
	// 	}
	// }

	if f.font.glyf != nil && f.font.loca != nil {
		newfnt.loca = new(locaTable)
		newfnt.glyf = new(glyfTable)
		for _, gid := range indices {
			newfnt.glyf.descs = append(newfnt.glyf.descs, f.font.glyf.descs[gid])
		}
		isShort := f.font.head.indexToLocFormat == 0
		if isShort {
			newfnt.loca.offsetsShort = make([]offset16, len(newfnt.glyf.descs)+1)
			newfnt.loca.offsetsShort[0] = f.font.loca.offsetsShort[0]
		} else {
			newfnt.loca.offsetsLong = make([]offset32, len(newfnt.glyf.descs)+1)
			newfnt.loca.offsetsLong[0] = f.font.loca.offsetsLong[0]
		}
		for i, desc := range newfnt.glyf.descs {
			if isShort {
				newfnt.loca.offsetsShort[i+1] = newfnt.loca.offsetsShort[i] + offset16(len(desc.raw))/2
			} else {
				newfnt.loca.offsetsLong[i+1] = newfnt.loca.offsetsLong[i] + offset32(len(desc.raw))
			}
		}
	}

	if f.font.hhea != nil {
		newfnt.hhea = &hheaTable{}
		*newfnt.hhea = *f.font.hhea
		newfnt.hhea.numberOfHMetrics = uint16(len(newfnt.glyf.descs))
	}
	if f.font.head != nil {
		newfnt.head = new(headTable)
		*newfnt.head = *f.font.head
	}

	if f.font.hmtx != nil {
		newfnt.hmtx = new(hmtxTable)
		hmLen := len(f.font.hmtx.hMetrics)
		for _, gid := range indices {
			newfnt.hmtx.hMetrics = append(newfnt.hmtx.hMetrics, f.font.hmtx.hMetrics[min(hmLen-1, int(gid))])
		}
		newfnt.optimizeHmtx()
	}

	if f.font.maxp != nil {
		newfnt.maxp = new(maxpTable)
		*newfnt.maxp = *f.font.maxp
		newfnt.maxp.numGlyphs = uint16(len(newfnt.glyf.descs))
	}

	subfnt := &Font{
		br:   nil,
		font: &newfnt,
	}
	return subfnt, nil
}

// Write writes the font to `w`.
func (f *Font) Write(w io.Writer) error {
	bw := newByteWriter(w)
	err := f.font.write(bw)
	if err != nil {
		return err
	}
	return bw.flush()
}

package lvgl

import (
	"encoding/binary"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

type HeadTable struct {
	Size    uint32  //4	Record size (for quick skip)
	Label   [4]byte //4	head (table marker)
	Version uint32  //4	Version (reserved)
	Tables  uint16  //2	Number of additional tables (2 bytes to simplify align)

	//字体排版度量（来自 OpenType）
	FontSize    uint16 //2	Font size (px), as defined in convertor params
	Ascent      uint16 //2	Ascent (uint16), as returned by Font.ascender of opentype.js (usually HHead ascent)
	Descent     int16  //2	Descent (int16, negative), as returned by Font.descender of opentype.js (usually HHead descent)
	TypoAscent  uint16 //2	typoAscent (uint16), typographic ascent
	TypoDescent int16  //2	typoDescent (int16), typographic descent
	TypoLineGap uint16 //2	typoLineGap (uint16), typographic line gap
	MinY        int16  //2	min Y (used to quick check line intersections with other objects)
	MaxY        int16  //2	max Y

	DefAdvanceWidth uint16 //2	default advanceWidth (uint16), if glyph advanceWidth bits length = 0
	KerningScale    uint16 //2	kerningScale, FP12.4 unsigned, scale for kerning data, to fit source in 1 byte

	//glyph ID / loca 格式
	IndexToLocFormat byte //1	indexToLocFormat in loca table (0 - Offset16, 1 - Offset32)
	GlyphIdFormat    byte //1	glyphIdFormat (0 - 1 byte, 1 - 2 bytes)

	AdvanceWidthFormat byte //1	advanceWidthFormat (0 - Uint, 1 - unsigned with 4 bits)
	//位图与 BBox 配置
	BitsPerPixel     byte //1	Bits per pixel (1, 2, 3 or 4)
	XyBits           byte //1	Glyph BBox x/y bits length (unsigned)
	WhBits           byte //1	Glyph BBox w/h bits length (unsigned)
	AdvanceWidthBits byte //1	Glyph advanceWidth bits length (unsigned, may be FP4)
	// 压缩信息
	CompressionId byte //1	Compression alg ID (0 - raw bits, 1 - RLE-like with XOR prefilter, 2 - RLE-like only without prefilter)
	SubpixelsMode byte //1	Subpixel rendering. 0 - none, 1 - horisontal resolution of bitmaps is 3x, 2 - vertical resolution of bitmaps is 3x.
	tmpReserved1  byte //1	Reserved (align to 2x)
	// 下划线喜喜
	UnderlinePosition  int16 //2	Underline position (int16), scaled post.underlinePosition
	UnderlineThickness int16 //2	Underline thickness (uint16), scaled post.underlineThickness
	//尾部对其
	//Blank []uint8 //x	Unused (Align header length to 4x)
}

func NewHeadTable(pf *sfnt.Font, fontSize uint16) *HeadTable {
	metrics, _ := pf.Metrics(nil, fixed.I(int(fontSize)), font.HintingNone)
	t := &HeadTable{
		Size:               48,
		Label:              [4]byte{'h', 'e', 'a', 'd'},
		Version:            1,
		Tables:             3,
		FontSize:           fontSize,
		Ascent:             0, //Math.max(...glyphs.map(g => g.bbox.y + g.bbox.height)),
		Descent:            0, //Math.min(...glyphs.map(g => g.bbox.y)),
		TypoAscent:         uint16(metrics.Ascent.Round()),
		TypoDescent:        int16(metrics.Descent.Round()),
		TypoLineGap:        0,
		MinY:               0, //Math.min(...glyphs.map(g => g.bbox.y)),
		MaxY:               0, //Math.max(...glyphs.map(g => g.bbox.y + g.bbox.height)),
		DefAdvanceWidth:    fontSize,
		KerningScale:       1.0,
		IndexToLocFormat:   1,
		GlyphIdFormat:      1,
		AdvanceWidthFormat: 1,
		BitsPerPixel:       4,
		XyBits:             8,
		WhBits:             8,
		AdvanceWidthBits:   16,
	}
	if postTable := pf.PostTable(); postTable != nil {
		t.UnderlinePosition = postTable.UnderlinePosition
		t.UnderlineThickness = postTable.UnderlineThickness
	}
	t.Size = uint32(binary.Size(t))
	return t
}

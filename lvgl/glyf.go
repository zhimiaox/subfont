package lvgl

import (
	"bytes"
	"encoding/binary"
	"image"
	"os"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/vector"
)

type GlyfTable struct {
	Size  uint32  //4	Record size (for quick skip)
	Label [4]byte //4	glyf (table marker)
	//Data  []byte  // glyph id1 id2 ... data
}

type GlyfData struct {
	GlyfDataInfo
	Bitmap *bytes.Buffer
}

type GlyfDataInfo struct {
	AdvanceWidth int16 //advanceWidth (length/format in font header, may have 4 fractional bits)
	BBoxX        int8  //NN	BBox X (length in font header)
	BBoxY        int8  //NN	BBox Y (length in font header)
	BBoxWidth    uint8 //NN	BBox Width (length in font header)
	BBoxHeight   uint8 //NN	BBox Height (length in font header)
}

func (d *GlyfData) Bytes() []byte {
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.BigEndian, d.GlyfDataInfo)
	_, _ = d.Bitmap.WriteTo(buf)
	return buf.Bytes()
}

func NewGlyfTable() *GlyfTable {
	return &GlyfTable{
		Size:  8,
		Label: [4]byte{'g', 'l', 'y', 'f'},
		//Data:  nil,
	}
}

func AddGlyfData(buf *sfnt.Buffer, pf *sfnt.Font, fontSize uint16, r rune) (*GlyfData, error) {
	glyphIndex, err := pf.GlyphIndex(buf, r)
	if err != nil {
		return nil, err
	}
	fontI := fixed.I(int(fontSize))
	bounds, advance, err := pf.GlyphBounds(buf, glyphIndex, fontI, font.HintingNone)
	segments, err := pf.LoadGlyph(buf, glyphIndex, fontI, nil)
	if err != nil {
		return nil, err
	}
	info := &GlyfData{
		GlyfDataInfo: GlyfDataInfo{
			AdvanceWidth: int16(advance.Round() * 16), // LVGL FP4,
			//BBoxX:        int8(bounds.Min.X.Round()),
			//BBoxY:        int8(bounds.Min.Y.Round()),
			BBoxWidth:  uint8(bounds.Max.X.Round() - bounds.Min.X.Round()),
			BBoxHeight: uint8(bounds.Max.Y.Round() - bounds.Min.Y.Round()),
		},
		Bitmap: new(bytes.Buffer),
	}
	var (
		width   = int(info.BBoxWidth)
		height  = int(info.BBoxHeight)
		originX = float32(-bounds.Min.X.Round())
		originY = float32(-bounds.Min.Y.Round())
	)
	rasterizer := vector.NewRasterizer(width, height)
	rasterizer.DrawOp = draw.Src
	for _, seg := range segments {
		switch seg.Op {
		case sfnt.SegmentOpMoveTo:
			rasterizer.MoveTo(
				originX+float32(seg.Args[0].X)/64,
				originY+float32(seg.Args[0].Y)/64,
			)
		case sfnt.SegmentOpLineTo:
			rasterizer.LineTo(
				originX+float32(seg.Args[0].X)/64,
				originY+float32(seg.Args[0].Y)/64,
			)
		case sfnt.SegmentOpQuadTo:
			rasterizer.QuadTo(
				originX+float32(seg.Args[0].X)/64,
				originY+float32(seg.Args[0].Y)/64,
				originX+float32(seg.Args[1].X)/64,
				originY+float32(seg.Args[1].Y)/64,
			)
		case sfnt.SegmentOpCubeTo:
			rasterizer.CubeTo(
				originX+float32(seg.Args[0].X)/64,
				originY+float32(seg.Args[0].Y)/64,
				originX+float32(seg.Args[1].X)/64,
				originY+float32(seg.Args[1].Y)/64,
				originX+float32(seg.Args[2].X)/64,
				originY+float32(seg.Args[2].Y)/64,
			)
		}
	}
	dst := image.NewAlpha(image.Rect(0, 0, width, height))
	rasterizer.Draw(dst, dst.Bounds(), image.Opaque, image.Point{})
	// 4bit一个像素点
	bSplit, bByte := 0, byte(0)
	for y := range height {
		for x := range width {
			a := dst.AlphaAt(x, y).A >> 4
			if bSplit == 0 {
				bByte = a << 4
				bSplit = 1
			} else {
				bByte |= a
				info.Bitmap.WriteByte(bByte)
				bSplit = 0
			}
		}
	}
	if bSplit != 0 {
		info.Bitmap.WriteByte(bByte)
	}

	// Visualize the pixels.
	const asciiArt = ".++8"
	buf23 := make([]byte, 0, height*(width+1))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			a := dst.AlphaAt(x, y).A
			buf23 = append(buf23, asciiArt[a>>6])
		}
		buf23 = append(buf23, '\n')
	}
	os.Stdout.Write(buf23)

	return info, nil
}

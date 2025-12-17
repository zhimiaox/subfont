// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lvgl

import (
	"os"

	"golang.org/x/image/font/sfnt"
)

func ExampleNewFont() {
	// 1. 读取字体文件
	fontBytes, err := os.ReadFile("../testdata/NotoSansSC-Bold.ttf")
	if err != nil {
		panic(err)
	}
	pf, err := sfnt.Parse(fontBytes)
	if err != nil {
		panic(err)
	}

	bin, _ := NewFont(pf, 32, append([]rune("0123"), 0x71CA, 0x01F16C, 0x2265))
	os.WriteFile("out.bin", bin, 655)
}

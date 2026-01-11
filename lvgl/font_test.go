package lvgl

import (
	"os"
	"testing"

	"golang.org/x/image/font/sfnt"
)

func TestNewFont(t *testing.T) {
	// 1. 读取字体文件
	fontBytes, err := os.ReadFile("../testdata/NotoSansSC-Bold.ttf")
	if err != nil {
		panic(err)
	}
	pf, err := sfnt.Parse(fontBytes)
	if err != nil {
		panic(err)
	}

	bin, _ := NewFont(pf, 32, append([]rune("abgpqttx"), 0x71CA, 0x01F16C, 0x2265))
	_ = os.WriteFile("out.bin", bin, 655)
}

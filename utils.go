package subfont

import (
	"math"
	"unicode/utf16"
)

// NumT is a constraint for all integers and floats
type NumT interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64
}

// ConvNumber 数字类型安全转换
// Examples
//
// converted, ok := ConvertNumber[uint8](int64(1))
//
// output:
//
//	1, true
func ConvNumber[OutT NumT, InT NumT](orig InT) (converted OutT, ok bool) {
	converted = OutT(orig)
	switch any(converted).(type) {
	case float64:
		return converted, true
	case float32:
		f64, isF64 := any(orig).(float64)
		if !isF64 {
			return converted, true
		}
		if math.Abs(f64) < math.MaxFloat32 {
			return converted, true
		}
		return 0, false
	}
	if (orig < 0) != (converted < 0) {
		return 0, false
	}
	cast := InT(converted)
	base := orig
	switch f := any(orig).(type) {
	case float64:
		base = InT(math.Trunc(f))
	case float32:
		base = InT(math.Trunc(float64(f)))
	}
	if cast == base {
		return converted, true
	}
	return 0, false
}

// UTF16ToString decodes the UTF-16BE encoded byte slice `b` to a Unicode go string.
func UTF16ToString(b []byte) string {
	if len(b) == 1 {
		return string(rune(b[0]))
	}
	if len(b)%2 != 0 {
		b = append(b, 0)
	}
	n := len(b) >> 1
	chars := make([]uint16, n)
	for i := 0; i < n; i++ {
		chars[i] = uint16(b[i<<1])<<8 + uint16(b[i<<1+1])
	}
	return string(utf16.Decode(chars))
}

package frame

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"unicode/utf8"
)

const (
	// MaxLine is the protocol's line limit in bytes, including the terminating "\n".
	MaxLine = 2048
	// MaxStr caps every string value (in characters) so odd names can't eat the budget.
	MaxStr = 48
)

// Encode serializes one message as a compact UTF-8 JSON line terminated by "\n".
// Floats are rounded to one decimal, nil and non-finite floats are dropped, strings
// are capped at MaxStr characters.
func Encode(v any) []byte {
	var b bytes.Buffer
	if !writeValue(&b, v) {
		b.WriteString("null")
	}
	b.WriteByte('\n')
	return b.Bytes()
}

// dropped reports whether a value is removed by cleaning (Python: clean(v) is None).
func dropped(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case float64:
		return math.IsNaN(x) || math.IsInf(x, 0)
	case float32:
		return math.IsNaN(float64(x)) || math.IsInf(float64(x), 0)
	case *Obj:
		return x == nil
	}
	return false
}

// writeValue writes v and reports whether anything was written.
func writeValue(b *bytes.Buffer, v any) bool {
	if dropped(v) {
		return false
	}
	switch x := v.(type) {
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case int:
		b.WriteString(strconv.Itoa(x))
	case int64:
		b.WriteString(strconv.FormatInt(x, 10))
	case int32:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case uint64:
		b.WriteString(strconv.FormatUint(x, 10))
	case uint32:
		b.WriteString(strconv.FormatUint(uint64(x), 10))
	case float64:
		b.WriteString(FormatFloat(x))
	case float32:
		b.WriteString(FormatFloat(float64(x)))
	case string:
		writeString(b, x)
	case *Obj:
		b.WriteByte('{')
		first := true
		for _, k := range x.keys {
			val := x.m[k]
			if dropped(val) {
				continue
			}
			if !first {
				b.WriteByte(',')
			}
			first = false
			writeString(b, k)
			b.WriteByte(':')
			writeValue(b, val)
		}
		b.WriteByte('}')
	case []any:
		writeList(b, len(x), func(i int) any { return x[i] })
	case []*Obj:
		writeList(b, len(x), func(i int) any { return x[i] })
	case []float64:
		writeList(b, len(x), func(i int) any { return x[i] })
	case []string:
		writeList(b, len(x), func(i int) any { return x[i] })
	default:
		writeString(b, fmt.Sprint(x))
	}
	return true
}

func writeList(b *bytes.Buffer, n int, at func(int) any) {
	b.WriteByte('[')
	first := true
	for i := 0; i < n; i++ {
		v := at(i)
		if dropped(v) {
			continue
		}
		if !first {
			b.WriteByte(',')
		}
		first = false
		writeValue(b, v)
	}
	b.WriteByte(']')
}

// Round1 rounds to one decimal exactly like Python's round(x, 1) (correctly rounded,
// half-to-even on exact binary ties) and turns -0.0 into 0.0.
func Round1(x float64) float64 {
	r, err := strconv.ParseFloat(strconv.FormatFloat(x, 'f', 1, 64), 64)
	if err != nil || r == 0 {
		return 0
	}
	return r
}

// FormatFloat renders a float the way Python's json module renders round(x, 1):
// always with a decimal point ("55.0"), exponent form only from 1e16 up.
func FormatFloat(x float64) string {
	r := Round1(x)
	if math.Abs(r) >= 1e16 {
		s := strconv.FormatFloat(r, 'g', -1, 64) // e.g. 1e+16
		return s
	}
	s := strconv.FormatFloat(r, 'f', -1, 64)
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return s
		}
	}
	return s + ".0"
}

const hexDigits = "0123456789abcdef"

// writeString writes a JSON string (capped at MaxStr characters), escaping like Python's
// json.dumps(ensure_ascii=False): quotes, backslashes and control characters only.
func writeString(b *bytes.Buffer, s string) {
	b.WriteByte('"')
	n := 0
	for i := 0; i < len(s) && n < MaxStr; n++ {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\f':
			b.WriteString(`\f`)
		case r < 0x20:
			b.WriteString(`\u00`)
			b.WriteByte(hexDigits[r>>4])
			b.WriteByte(hexDigits[r&0xf])
		default:
			b.WriteRune(r) // invalid UTF-8 becomes U+FFFD
		}
	}
	b.WriteByte('"')
}

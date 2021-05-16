package zaptextencoder

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestTextEncodeEntry(t *testing.T) {
	type bar struct {
		Key string  `json:"key"`
		Val float64 `json:"val"`
	}

	type foo struct {
		A string  `json:"aee"`
		B int     `json:"bee"`
		C float64 `json:"cee"`
		D []bar   `json:"dee"`
	}

	tests := []struct {
		desc     string
		expected string
		ent      zapcore.Entry
		fields   []zapcore.Field
	}{
		{
			desc: "info entry with some fields",
			expected: `2018-06-19T16:33:42.000Z` +
				`  info ` +
				`  bob` +
				`  lob law` +
				`  so="passes"` +
				`  answer=42` +
				`  common_pie=3.14` +
				`  null_value=null` +
				`  array_with_null_elements=[{},null,null,2]` +
				`  such={"aee":"lol","bee":123,"cee":0.9999,"dee":[{"key":"pi","val":3.141592653589793},{"key":"tau","val":6.283185307179586}]}` +
				"\n",
			ent: zapcore.Entry{
				Level:      zapcore.InfoLevel,
				Time:       time.Date(2018, 6, 19, 16, 33, 42, 99, time.UTC),
				LoggerName: "bob",
				Message:    "lob law",
			},
			fields: []zapcore.Field{
				zap.String("so", "passes"),
				zap.Int("answer", 42),
				zap.Float64("common_pie", 3.14),
				// Cover special-cased handling of nil in AddReflect() and
				// AppendReflect(). Note that for the latter, we explicitly test
				// correct results for both the nil static interface{} value
				// (`nil`), as well as the non-nil interface value with a
				// dynamic type and nil value (`(*struct{})(nil)`).
				zap.Reflect("null_value", nil),
				zap.Reflect("array_with_null_elements", []interface{}{&struct{}{}, nil, (*struct{})(nil), 2}),
				zap.Reflect("such", foo{
					A: "lol",
					B: 123,
					C: 0.9999,
					D: []bar{
						{"pi", 3.141592653589793},
						{"tau", 6.283185307179586},
					},
				}),
			},
		},
	}

	enc := NewTextEncoder(zapcore.EncoderConfig{
		MessageKey:     "M",
		LevelKey:       "L",
		TimeKey:        "T",
		NameKey:        "N",
		CallerKey:      "C",
		FunctionKey:    "F",
		StacktraceKey:  "S",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	})

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			buf, err := enc.EncodeEntry(tt.ent, tt.fields)
			if assert.NoError(t, err, "Unexpected text encoding error.") {
				assert.Equal(t, tt.expected, buf.String(), "Incorrect encoded text entry.")
			}
			buf.Free()
		})
	}
}

func TestTextEmptyConfig(t *testing.T) {
	tests := []struct {
		name     string
		field    zapcore.Field
		expected string
	}{
		{
			name:     "time",
			field:    zap.Time("foo", time.Unix(1591287718, 0)), // 2020-06-04 09:21:58 -0700 PDT
			expected: "foo=1591287718000000000\n",
		},
		{
			name:     "duration",
			field:    zap.Duration("bar", time.Microsecond),
			expected: "bar=1000\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := NewTextEncoder(zapcore.EncoderConfig{})

			buf, err := enc.EncodeEntry(zapcore.Entry{
				Level:      zapcore.DebugLevel,
				Time:       time.Now(),
				LoggerName: "mylogger",
				Message:    "things happened",
			}, []zapcore.Field{tt.field})
			if assert.NoError(t, err, "Unexpected text encoding error.") {
				assert.Equal(t, tt.expected, buf.String(), "Incorrect encoded text entry.")
			}

			buf.Free()
		})
	}
}

var _defaultEncoderConfig = zapcore.EncoderConfig{
	EncodeTime:     zapcore.EpochTimeEncoder,
	EncodeDuration: zapcore.SecondsDurationEncoder,
}

func TestTextClone(t *testing.T) {
	// The parent encoder is created with plenty of excess capacity.
	parent := &textEncoder{buf: bufferPool.Get()}
	clone := parent.Clone()

	// Adding to the parent shouldn't affect the clone, and vice versa.
	parent.AddString("foo", "bar")
	clone.AddString("baz", "bing")

	assertText(t, `foo="bar"`, parent)
	assertText(t, `baz="bing"`, clone.(*textEncoder))
}

func TestTextEscaping(t *testing.T) {
	enc := &textEncoder{buf: bufferPool.Get()}
	// Test all the edge cases of JSON escaping directly.
	cases := map[string]string{
		// ASCII.
		`foo`: `foo`,
		// Special-cased characters.
		`"`: `\"`,
		`\`: `\\`,
		// Special-cased characters within everyday ASCII.
		`foo"foo`: `foo\"foo`,
		"foo\n":   `foo\n`,
		// Special-cased control characters.
		"\n": `\n`,
		"\r": `\r`,
		"\t": `\t`,
		// \b and \f are sometimes backslash-escaped, but this representation is also
		// conformant.
		"\b": `\u0008`,
		"\f": `\u000c`,
		// The standard lib special-cases angle brackets and ampersands by default,
		// because it wants to protect users from browser exploits. In a logging
		// context, we shouldn't special-case these characters.
		"<": "<",
		">": ">",
		"&": "&",
		// ASCII bell - not special-cased.
		string(byte(0x07)): `\u0007`,
		// Astral-plane unicode.
		`☃`: `☃`,
		// Decodes to (RuneError, 1)
		"\xed\xa0\x80":    `\ufffd\ufffd\ufffd`,
		"foo\xed\xa0\x80": `foo\ufffd\ufffd\ufffd`,
	}

	t.Run("String", func(t *testing.T) {
		for input, output := range cases {
			enc.truncate()
			enc.safeAddString(input)
			assertText(t, output, enc)
		}
	})

	t.Run("ByteString", func(t *testing.T) {
		for input, output := range cases {
			enc.truncate()
			enc.safeAddByteString([]byte(input))
			assertText(t, output, enc)
		}
	})
}

func TestTextEncoderObjectFields(t *testing.T) {
	tests := []struct {
		desc     string
		expected string
		f        func(zapcore.Encoder)
	}{
		{"binary", `k="YWIxMg=="`, func(e zapcore.Encoder) { e.AddBinary("k", []byte("ab12")) }},
		{"bool", `k\\=true`, func(e zapcore.Encoder) { e.AddBool(`k\`, true) }}, // test key escaping once
		{"bool", `k=true`, func(e zapcore.Encoder) { e.AddBool("k", true) }},
		{"bool", `k=false`, func(e zapcore.Encoder) { e.AddBool("k", false) }},
		{"byteString", `k="v\\"`, func(e zapcore.Encoder) { e.AddByteString(`k`, []byte(`v\`)) }},
		{"byteString", `k="v"`, func(e zapcore.Encoder) { e.AddByteString("k", []byte("v")) }},
		{"byteString", `k=""`, func(e zapcore.Encoder) { e.AddByteString("k", []byte{}) }},
		{"byteString", `k=""`, func(e zapcore.Encoder) { e.AddByteString("k", nil) }},
		{"complex128", `k="1+2i"`, func(e zapcore.Encoder) { e.AddComplex128("k", 1+2i) }},
		{"complex64", `k="1+2i"`, func(e zapcore.Encoder) { e.AddComplex64("k", 1+2i) }},
		{"duration", `k=0.000000001`, func(e zapcore.Encoder) { e.AddDuration("k", 1) }},
		{"float64", `k=1`, func(e zapcore.Encoder) { e.AddFloat64("k", 1.0) }},
		{"float64", `k=10000000000`, func(e zapcore.Encoder) { e.AddFloat64("k", 1e10) }},
		{"float64", `k=NaN`, func(e zapcore.Encoder) { e.AddFloat64("k", math.NaN()) }},
		{"float64", `k=+Inf`, func(e zapcore.Encoder) { e.AddFloat64("k", math.Inf(1)) }},
		{"float64", `k=-Inf`, func(e zapcore.Encoder) { e.AddFloat64("k", math.Inf(-1)) }},
		{"float32", `k=1`, func(e zapcore.Encoder) { e.AddFloat32("k", 1.0) }},
		{"float32", `k=10000000000`, func(e zapcore.Encoder) { e.AddFloat32("k", 1e10) }},
		{"float32", `k=NaN`, func(e zapcore.Encoder) { e.AddFloat32("k", float32(math.NaN())) }},
		{"float32", `k=+Inf`, func(e zapcore.Encoder) { e.AddFloat32("k", float32(math.Inf(1))) }},
		{"float32", `k=-Inf`, func(e zapcore.Encoder) { e.AddFloat32("k", float32(math.Inf(-1))) }},
		{"int", `k=42`, func(e zapcore.Encoder) { e.AddInt("k", 42) }},
		{"int64", `k=42`, func(e zapcore.Encoder) { e.AddInt64("k", 42) }},
		{"int32", `k=42`, func(e zapcore.Encoder) { e.AddInt32("k", 42) }},
		{"int16", `k=42`, func(e zapcore.Encoder) { e.AddInt16("k", 42) }},
		{"int8", `k=42`, func(e zapcore.Encoder) { e.AddInt8("k", 42) }},
		{"string", `k="v\\"`, func(e zapcore.Encoder) { e.AddString(`k`, `v\`) }},
		{"string", `k="v"`, func(e zapcore.Encoder) { e.AddString("k", "v") }},
		{"string", `k=""`, func(e zapcore.Encoder) { e.AddString("k", "") }},
		{"time", `k=1`, func(e zapcore.Encoder) { e.AddTime("k", time.Unix(1, 0)) }},
		{"uint", `k=42`, func(e zapcore.Encoder) { e.AddUint("k", 42) }},
		{"uint64", `k=42`, func(e zapcore.Encoder) { e.AddUint64("k", 42) }},
		{"uint32", `k=42`, func(e zapcore.Encoder) { e.AddUint32("k", 42) }},
		{"uint16", `k=42`, func(e zapcore.Encoder) { e.AddUint16("k", 42) }},
		{"uint8", `k=42`, func(e zapcore.Encoder) { e.AddUint8("k", 42) }},
		{"uintptr", `k=42`, func(e zapcore.Encoder) { e.AddUintptr("k", 42) }},
		{
			desc:     "reflect (success)",
			expected: `k={"escape":"\u003c\u0026\u003e","loggable":"yes"}`,
			f: func(e zapcore.Encoder) {
				assert.NoError(t, e.AddReflected("k", map[string]string{"escape": "<&>", "loggable": "yes"}), "Unexpected error JSON-serializing a map.")
			},
		},
		{
			desc:     "reflect (failure)",
			expected: "",
			f: func(e zapcore.Encoder) {
				assert.Error(t, e.AddReflected("k", noJSON{}), "Unexpected success JSON-serializing a noJSON.")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			assertOutput(t, _defaultEncoderConfig, tt.expected, tt.f)
		})
	}
}

func TestTextEncoderArrays(t *testing.T) {
	tests := []struct {
		desc     string
		expected string // expect f to be called twice
		f        func(zapcore.ArrayEncoder)
	}{
		{"bool", `[true,true]`, func(e zapcore.ArrayEncoder) { e.AppendBool(true) }},
		{"byteString", `["k","k"]`, func(e zapcore.ArrayEncoder) { e.AppendByteString([]byte("k")) }},
		{"byteString", `["k\\","k\\"]`, func(e zapcore.ArrayEncoder) { e.AppendByteString([]byte(`k\`)) }},
		{"complex128", `["1+2i","1+2i"]`, func(e zapcore.ArrayEncoder) { e.AppendComplex128(1 + 2i) }},
		{"complex64", `["1+2i","1+2i"]`, func(e zapcore.ArrayEncoder) { e.AppendComplex64(1 + 2i) }},
		{"durations", `[0.000000002,0.000000002]`, func(e zapcore.ArrayEncoder) { e.AppendDuration(2) }},
		{"float64", `[3.14,3.14]`, func(e zapcore.ArrayEncoder) { e.AppendFloat64(3.14) }},
		{"float32", `[3.14,3.14]`, func(e zapcore.ArrayEncoder) { e.AppendFloat32(3.14) }},
		{"int", `[42,42]`, func(e zapcore.ArrayEncoder) { e.AppendInt(42) }},
		{"int64", `[42,42]`, func(e zapcore.ArrayEncoder) { e.AppendInt64(42) }},
		{"int32", `[42,42]`, func(e zapcore.ArrayEncoder) { e.AppendInt32(42) }},
		{"int16", `[42,42]`, func(e zapcore.ArrayEncoder) { e.AppendInt16(42) }},
		{"int8", `[42,42]`, func(e zapcore.ArrayEncoder) { e.AppendInt8(42) }},
		{"string", `["k","k"]`, func(e zapcore.ArrayEncoder) { e.AppendString("k") }},
		{"string", `["k\\","k\\"]`, func(e zapcore.ArrayEncoder) { e.AppendString(`k\`) }},
		{"times", `[1,1]`, func(e zapcore.ArrayEncoder) { e.AppendTime(time.Unix(1, 0)) }},
		{"uint", `[42,42]`, func(e zapcore.ArrayEncoder) { e.AppendUint(42) }},
		{"uint64", `[42,42]`, func(e zapcore.ArrayEncoder) { e.AppendUint64(42) }},
		{"uint32", `[42,42]`, func(e zapcore.ArrayEncoder) { e.AppendUint32(42) }},
		{"uint16", `[42,42]`, func(e zapcore.ArrayEncoder) { e.AppendUint16(42) }},
		{"uint8", `[42,42]`, func(e zapcore.ArrayEncoder) { e.AppendUint8(42) }},
		{"uintptr", `[42,42]`, func(e zapcore.ArrayEncoder) { e.AppendUintptr(42) }},
		{
			desc:     "arrays (success)",
			expected: `[[true],[true]]`,
			f: func(arr zapcore.ArrayEncoder) {
				assert.NoError(t, arr.AppendArray(zapcore.ArrayMarshalerFunc(func(inner zapcore.ArrayEncoder) error {
					inner.AppendBool(true)
					return nil
				})), "Unexpected error appending an array.")
			},
		},
		{
			desc:     "arrays (error)",
			expected: `[[true],[true]]`,
			f: func(arr zapcore.ArrayEncoder) {
				assert.Error(t, arr.AppendArray(zapcore.ArrayMarshalerFunc(func(inner zapcore.ArrayEncoder) error {
					inner.AppendBool(true)
					return errors.New("fail")
				})), "Expected an error appending an array.")
			},
		},
		{
			desc:     "reflect (success)",
			expected: `[{"foo":5},{"foo":5}]`,
			f: func(arr zapcore.ArrayEncoder) {
				assert.NoError(
					t,
					arr.AppendReflected(map[string]int{"foo": 5}),
					"Unexpected an error appending an object with reflection.",
				)
			},
		},
		{
			desc:     "reflect (error)",
			expected: `[]`,
			f: func(arr zapcore.ArrayEncoder) {
				assert.Error(
					t,
					arr.AppendReflected(noJSON{}),
					"Unexpected an error appending an object with reflection.",
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			f := func(enc zapcore.Encoder) error {
				return enc.AddArray("array", zapcore.ArrayMarshalerFunc(func(arr zapcore.ArrayEncoder) error {
					tt.f(arr)
					tt.f(arr)
					return nil
				}))
			}
			assertOutput(t, _defaultEncoderConfig, `array=`+tt.expected, func(enc zapcore.Encoder) {
				err := f(enc)
				assert.NoError(t, err, "Unexpected error adding array to JSON encoder.")
			})
		})
	}
}
func assertText(t *testing.T, expected string, enc *textEncoder) {
	assert.Equal(t, expected, enc.buf.String(), "Encoded text didn't match expectations.")
}

func assertOutput(t testing.TB, cfg zapcore.EncoderConfig, expected string, f func(zapcore.Encoder)) {
	enc := &textEncoder{buf: bufferPool.Get(), EncoderConfig: &cfg}
	f(enc)
	assert.Equal(t, expected, enc.buf.String(), "Unexpected encoder output after adding.")

	enc.truncate()
	enc.AddString("foo", "bar")
	f(enc)
	expectedPrefix := `foo="bar"`
	if expected != "" {
		// If we expect output, it should be comma-separated from the previous
		// field.
		expectedPrefix += ""
	}
	assert.Equal(t, expectedPrefix+expected, enc.buf.String(), "Unexpected encoder output after adding as a second field.")
}

type noJSON struct{}

func (nj noJSON) MarshalJSON() ([]byte, error) {
	return nil, errors.New("no")
}

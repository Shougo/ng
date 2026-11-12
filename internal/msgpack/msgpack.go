// Package msgpack implements the subset of MessagePack that the Neovim
// msgpack-rpc API uses. It depends only on the standard library.
//
// Decoded values use these Go types:
//
//	nil, bool, int64, uint64 (only above MaxInt64), float64, string,
//	[]byte (bin), []any (array), map[string]any (map with string keys),
//	map[any]any (map with other keys), Ext (extension types such as
//	Buffer, Window and Tabpage).
package msgpack

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
)

// Ext is a MessagePack extension value.
type Ext struct {
	Type int8
	Data []byte
}

// Int returns the integer stored in the extension payload. Neovim encodes
// handles (buffers, windows, tabpages) as a msgpack integer inside the ext.
func (e Ext) Int() (int64, error) {
	d := NewDecoder(bytesReader(e.Data))
	v, err := d.Decode()
	if err != nil {
		return 0, err
	}
	return ToInt(v)
}

// ToInt converts a decoded integer value to int64.
func ToInt(v any) (int64, error) {
	switch n := v.(type) {
	case int64:
		return n, nil
	case uint64:
		if n > math.MaxInt64 {
			return 0, fmt.Errorf("msgpack: %d overflows int64", n)
		}
		return int64(n), nil
	case float64:
		return int64(n), nil
	}
	return 0, fmt.Errorf("msgpack: %T is not an integer", v)
}

// ---------------------------------------------------------------- encoding

// Encoder writes MessagePack values.
type Encoder struct {
	w   io.Writer
	buf []byte
}

// NewEncoder returns an encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder { return &Encoder{w: w} }

// Encode writes v. The buffered bytes are written with a single Write call,
// so one message is never interleaved with another on the stream.
func (e *Encoder) Encode(v any) error {
	e.buf = e.buf[:0]
	var err error
	e.buf, err = Append(e.buf, v)
	if err != nil {
		return err
	}
	_, err = e.w.Write(e.buf)
	return err
}

// Marshal returns the encoding of v.
func Marshal(v any) ([]byte, error) { return Append(nil, v) }

// Append appends the encoding of v to b.
func Append(b []byte, v any) ([]byte, error) {
	switch x := v.(type) {
	case nil:
		return append(b, 0xc0), nil
	case bool:
		if x {
			return append(b, 0xc3), nil
		}
		return append(b, 0xc2), nil
	case int:
		return appendInt(b, int64(x)), nil
	case int8:
		return appendInt(b, int64(x)), nil
	case int16:
		return appendInt(b, int64(x)), nil
	case int32:
		return appendInt(b, int64(x)), nil
	case int64:
		return appendInt(b, x), nil
	case uint:
		return appendUint(b, uint64(x)), nil
	case uint8:
		return appendUint(b, uint64(x)), nil
	case uint16:
		return appendUint(b, uint64(x)), nil
	case uint32:
		return appendUint(b, uint64(x)), nil
	case uint64:
		return appendUint(b, x), nil
	case float32:
		return appendFloat(b, float64(x)), nil
	case float64:
		return appendFloat(b, x), nil
	case string:
		return appendString(b, x), nil
	case []byte:
		return appendBin(b, x), nil
	case Ext:
		return appendExt(b, x), nil
	case []any:
		b = appendArrayHeader(b, len(x))
		for _, e := range x {
			var err error
			if b, err = Append(b, e); err != nil {
				return nil, err
			}
		}
		return b, nil
	case []string:
		b = appendArrayHeader(b, len(x))
		for _, s := range x {
			b = appendString(b, s)
		}
		return b, nil
	case map[string]any:
		b = appendMapHeader(b, len(x))
		for k, e := range x {
			b = appendString(b, k)
			var err error
			if b, err = Append(b, e); err != nil {
				return nil, err
			}
		}
		return b, nil
	}
	// Fall back to reflection for other slices and maps.
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		b = appendArrayHeader(b, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			var err error
			if b, err = Append(b, rv.Index(i).Interface()); err != nil {
				return nil, err
			}
		}
		return b, nil
	case reflect.Map:
		b = appendMapHeader(b, rv.Len())
		it := rv.MapRange()
		for it.Next() {
			var err error
			if b, err = Append(b, it.Key().Interface()); err != nil {
				return nil, err
			}
			if b, err = Append(b, it.Value().Interface()); err != nil {
				return nil, err
			}
		}
		return b, nil
	}
	return nil, fmt.Errorf("msgpack: cannot encode %T", v)
}

func appendInt(b []byte, n int64) []byte {
	switch {
	case n >= 0:
		return appendUint(b, uint64(n))
	case n >= -32:
		return append(b, byte(n))
	case n >= math.MinInt8:
		return append(b, 0xd0, byte(n))
	case n >= math.MinInt16:
		return binary.BigEndian.AppendUint16(append(b, 0xd1), uint16(n))
	case n >= math.MinInt32:
		return binary.BigEndian.AppendUint32(append(b, 0xd2), uint32(n))
	}
	return binary.BigEndian.AppendUint64(append(b, 0xd3), uint64(n))
}

func appendUint(b []byte, n uint64) []byte {
	switch {
	case n <= 0x7f:
		return append(b, byte(n))
	case n <= math.MaxUint8:
		return append(b, 0xcc, byte(n))
	case n <= math.MaxUint16:
		return binary.BigEndian.AppendUint16(append(b, 0xcd), uint16(n))
	case n <= math.MaxUint32:
		return binary.BigEndian.AppendUint32(append(b, 0xce), uint32(n))
	}
	return binary.BigEndian.AppendUint64(append(b, 0xcf), n)
}

func appendFloat(b []byte, f float64) []byte {
	return binary.BigEndian.AppendUint64(append(b, 0xcb), math.Float64bits(f))
}

func appendString(b []byte, s string) []byte {
	n := len(s)
	switch {
	case n <= 31:
		b = append(b, 0xa0|byte(n))
	case n <= math.MaxUint8:
		b = append(b, 0xd9, byte(n))
	case n <= math.MaxUint16:
		b = binary.BigEndian.AppendUint16(append(b, 0xda), uint16(n))
	default:
		b = binary.BigEndian.AppendUint32(append(b, 0xdb), uint32(n))
	}
	return append(b, s...)
}

func appendBin(b []byte, p []byte) []byte {
	n := len(p)
	switch {
	case n <= math.MaxUint8:
		b = append(b, 0xc4, byte(n))
	case n <= math.MaxUint16:
		b = binary.BigEndian.AppendUint16(append(b, 0xc5), uint16(n))
	default:
		b = binary.BigEndian.AppendUint32(append(b, 0xc6), uint32(n))
	}
	return append(b, p...)
}

func appendExt(b []byte, e Ext) []byte {
	n := len(e.Data)
	switch n {
	case 1:
		b = append(b, 0xd4)
	case 2:
		b = append(b, 0xd5)
	case 4:
		b = append(b, 0xd6)
	case 8:
		b = append(b, 0xd7)
	case 16:
		b = append(b, 0xd8)
	default:
		switch {
		case n <= math.MaxUint8:
			b = append(b, 0xc7, byte(n))
		case n <= math.MaxUint16:
			b = binary.BigEndian.AppendUint16(append(b, 0xc8), uint16(n))
		default:
			b = binary.BigEndian.AppendUint32(append(b, 0xc9), uint32(n))
		}
	}
	b = append(b, byte(e.Type))
	return append(b, e.Data...)
}

func appendArrayHeader(b []byte, n int) []byte {
	switch {
	case n <= 15:
		return append(b, 0x90|byte(n))
	case n <= math.MaxUint16:
		return binary.BigEndian.AppendUint16(append(b, 0xdc), uint16(n))
	}
	return binary.BigEndian.AppendUint32(append(b, 0xdd), uint32(n))
}

func appendMapHeader(b []byte, n int) []byte {
	switch {
	case n <= 15:
		return append(b, 0x80|byte(n))
	case n <= math.MaxUint16:
		return binary.BigEndian.AppendUint16(append(b, 0xde), uint16(n))
	}
	return binary.BigEndian.AppendUint32(append(b, 0xdf), uint32(n))
}

// ---------------------------------------------------------------- decoding

// Decoder reads MessagePack values.
type Decoder struct {
	r *bufio.Reader
}

// NewDecoder returns a decoder that reads from r.
func NewDecoder(r io.Reader) *Decoder {
	if br, ok := r.(*bufio.Reader); ok {
		return &Decoder{r: br}
	}
	return &Decoder{r: bufio.NewReaderSize(r, 64*1024)}
}

// Unmarshal decodes a single value from b.
func Unmarshal(b []byte) (any, error) {
	return NewDecoder(bytesReader(b)).Decode()
}

// maxLen guards against corrupt length prefixes allocating huge buffers.
const maxLen = 1 << 30

var errTooLarge = errors.New("msgpack: length too large")

// Decode reads the next value.
func (d *Decoder) Decode() (any, error) {
	c, err := d.r.ReadByte()
	if err != nil {
		return nil, err
	}
	switch {
	case c <= 0x7f:
		return int64(c), nil
	case c >= 0xe0:
		return int64(int8(c)), nil
	case c&0xe0 == 0xa0:
		return d.str(int(c & 0x1f))
	case c&0xf0 == 0x90:
		return d.array(int(c & 0x0f))
	case c&0xf0 == 0x80:
		return d.mapN(int(c & 0x0f))
	}
	switch c {
	case 0xc0:
		return nil, nil
	case 0xc2:
		return false, nil
	case 0xc3:
		return true, nil
	case 0xcc:
		n, err := d.uint(1)
		return int64(n), err
	case 0xcd:
		n, err := d.uint(2)
		return int64(n), err
	case 0xce:
		n, err := d.uint(4)
		return int64(n), err
	case 0xcf:
		n, err := d.uint(8)
		if err != nil {
			return nil, err
		}
		if n > math.MaxInt64 {
			return n, nil
		}
		return int64(n), nil
	case 0xd0:
		n, err := d.uint(1)
		return int64(int8(n)), err
	case 0xd1:
		n, err := d.uint(2)
		return int64(int16(n)), err
	case 0xd2:
		n, err := d.uint(4)
		return int64(int32(n)), err
	case 0xd3:
		n, err := d.uint(8)
		return int64(n), err
	case 0xca:
		n, err := d.uint(4)
		return float64(math.Float32frombits(uint32(n))), err
	case 0xcb:
		n, err := d.uint(8)
		return math.Float64frombits(n), err
	case 0xd9, 0xda, 0xdb:
		n, err := d.uint(1 << (c - 0xd9))
		if err != nil {
			return nil, err
		}
		return d.str(int(n))
	case 0xc4, 0xc5, 0xc6:
		n, err := d.uint(1 << (c - 0xc4))
		if err != nil {
			return nil, err
		}
		return d.bytes(int(n))
	case 0xdc, 0xdd:
		n, err := d.uint(2 << (c - 0xdc))
		if err != nil {
			return nil, err
		}
		return d.array(int(n))
	case 0xde, 0xdf:
		n, err := d.uint(2 << (c - 0xde))
		if err != nil {
			return nil, err
		}
		return d.mapN(int(n))
	case 0xd4, 0xd5, 0xd6, 0xd7, 0xd8:
		return d.ext(1 << (c - 0xd4))
	case 0xc7, 0xc8, 0xc9:
		n, err := d.uint(1 << (c - 0xc7))
		if err != nil {
			return nil, err
		}
		return d.ext(int(n))
	}
	return nil, fmt.Errorf("msgpack: invalid byte 0x%02x", c)
}

func (d *Decoder) uint(size int) (uint64, error) {
	var p [8]byte
	if _, err := io.ReadFull(d.r, p[:size]); err != nil {
		return 0, unexpected(err)
	}
	var n uint64
	for _, b := range p[:size] {
		n = n<<8 | uint64(b)
	}
	return n, nil
}

func (d *Decoder) bytes(n int) ([]byte, error) {
	if n > maxLen {
		return nil, errTooLarge
	}
	p := make([]byte, n)
	if _, err := io.ReadFull(d.r, p); err != nil {
		return nil, unexpected(err)
	}
	return p, nil
}

func (d *Decoder) str(n int) (any, error) {
	p, err := d.bytes(n)
	if err != nil {
		return nil, err
	}
	return string(p), nil
}

func (d *Decoder) array(n int) (any, error) {
	if n > maxLen {
		return nil, errTooLarge
	}
	a := make([]any, 0, min(n, 4096))
	for i := 0; i < n; i++ {
		v, err := d.Decode()
		if err != nil {
			return nil, unexpected(err)
		}
		a = append(a, v)
	}
	return a, nil
}

func (d *Decoder) mapN(n int) (any, error) {
	if n > maxLen {
		return nil, errTooLarge
	}
	m := make(map[string]any, min(n, 4096))
	var other map[any]any
	for i := 0; i < n; i++ {
		k, err := d.Decode()
		if err != nil {
			return nil, unexpected(err)
		}
		v, err := d.Decode()
		if err != nil {
			return nil, unexpected(err)
		}
		if s, ok := k.(string); ok && other == nil {
			m[s] = v
			continue
		}
		if other == nil {
			other = make(map[any]any, n)
			for ks, kv := range m {
				other[ks] = kv
			}
		}
		if !reflect.TypeOf(k).Comparable() {
			return nil, fmt.Errorf("msgpack: unhashable map key %T", k)
		}
		other[k] = v
	}
	if other != nil {
		return other, nil
	}
	return m, nil
}

func (d *Decoder) ext(n int) (any, error) {
	t, err := d.r.ReadByte()
	if err != nil {
		return nil, unexpected(err)
	}
	p, err := d.bytes(n)
	if err != nil {
		return nil, err
	}
	return Ext{Type: int8(t), Data: p}, nil
}

func unexpected(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

type byteSliceReader struct {
	b []byte
}

func (r *byteSliceReader) Read(p []byte) (int, error) {
	if len(r.b) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.b)
	r.b = r.b[n:]
	return n, nil
}

func bytesReader(b []byte) io.Reader { return &byteSliceReader{b: b} }

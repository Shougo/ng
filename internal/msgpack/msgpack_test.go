package msgpack

import (
	"bytes"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	cases := []any{
		nil, true, false,
		int64(0), int64(1), int64(127), int64(128), int64(255), int64(256),
		int64(65535), int64(65536), int64(math.MaxInt32) + 1, int64(math.MaxInt64),
		int64(-1), int64(-32), int64(-33), int64(-128), int64(-129),
		int64(-32768), int64(-32769), int64(math.MinInt32) - 1, int64(math.MinInt64),
		1.5, -0.25,
		"", "a", strings.Repeat("x", 31), strings.Repeat("y", 32),
		strings.Repeat("z", 256), strings.Repeat("w", 70000), "あいう漢字▽",
		[]byte{}, []byte{1, 2, 3}, bytes.Repeat([]byte{9}, 300),
		[]any{}, []any{int64(1), "two", []any{nil}},
		map[string]any{}, map[string]any{"a": int64(1), "b": []any{"c"}},
		Ext{Type: 1, Data: []byte{5}}, Ext{Type: 2, Data: []byte{1, 2, 3}},
		Ext{Type: 0, Data: bytes.Repeat([]byte{7}, 16)},
	}
	for _, want := range cases {
		b, err := Marshal(want)
		if err != nil {
			t.Fatalf("Marshal(%#v): %v", want, err)
		}
		got, err := Unmarshal(b)
		if err != nil {
			t.Fatalf("Unmarshal(%#v): %v", want, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("round trip: got %#v, want %#v", got, want)
		}
	}
}

func TestLargeArrayAndMap(t *testing.T) {
	a := make([]any, 70000)
	m := make(map[string]any, 20)
	for i := range a {
		a[i] = int64(i)
	}
	for i := 0; i < 20; i++ {
		m[strings.Repeat("k", i+1)] = int64(i)
	}
	for _, want := range []any{a, m} {
		b, err := Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Unmarshal(b)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("mismatch for %T", want)
		}
	}
}

func TestUint64Large(t *testing.T) {
	b, _ := Marshal(uint64(math.MaxUint64))
	got, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}
	if got != uint64(math.MaxUint64) {
		t.Errorf("got %#v", got)
	}
}

func TestEncodeReflect(t *testing.T) {
	b, err := Marshal([]int{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := Unmarshal(b)
	if !reflect.DeepEqual(got, []any{int64(1), int64(2)}) {
		t.Errorf("got %#v", got)
	}
}

func TestNonStringMapKeys(t *testing.T) {
	// {1: "a", "b": 2}
	raw := []byte{0x82, 0x01, 0xa1, 'a', 0xa1, 'b', 0x02}
	got, err := Unmarshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := map[any]any{int64(1): "a", "b": int64(2)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v", got)
	}
}

func TestFloat32(t *testing.T) {
	got, err := Unmarshal([]byte{0xca, 0x3f, 0xc0, 0, 0}) // 1.5
	if err != nil || got != 1.5 {
		t.Errorf("got %#v, %v", got, err)
	}
}

func TestTruncated(t *testing.T) {
	b, _ := Marshal([]any{"hello", int64(1)})
	for i := 1; i < len(b); i++ {
		if _, err := Unmarshal(b[:i]); err == nil {
			t.Errorf("no error for truncated input of %d bytes", i)
		}
	}
}

func TestExtInt(t *testing.T) {
	e := Ext{Type: 0, Data: []byte{0xcd, 0x03, 0xe8}} // 1000
	n, err := e.Int()
	if err != nil || n != 1000 {
		t.Errorf("got %d, %v", n, err)
	}
}

func TestStream(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	for i := 0; i < 3; i++ {
		if err := enc.Encode([]any{int64(i), "x"}); err != nil {
			t.Fatal(err)
		}
	}
	dec := NewDecoder(&buf)
	for i := 0; i < 3; i++ {
		v, err := dec.Decode()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(v, []any{int64(i), "x"}) {
			t.Errorf("message %d: %#v", i, v)
		}
	}
}

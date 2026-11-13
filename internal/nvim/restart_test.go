package nvim

import (
	"reflect"
	"testing"
)

func TestParseRestart(t *testing.T) {
	tests := []struct {
		name   string
		params []any
		want   *Restart
	}{
		{"none", []any{[]any{"flush", []any{}}}, nil},
		{"0.12: address", []any{
			[]any{"grid_line", []any{int64(1)}},
			[]any{"restart", []any{"/run/user/1000/nvim.1.0"}},
		}, &Restart{Addr: "/run/user/1000/nvim.1.0"}},
		{"0.13: address and channel config", []any{
			[]any{"restart", []any{"/tmp/n.sock", map[string]any{"detach": false}}},
		}, &Restart{Addr: "/tmp/n.sock", ChanConfig: map[string]any{"detach": false}}},
		{"empty channel config", []any{
			[]any{"restart", []any{"/tmp/n.sock", map[string]any{}}},
		}, &Restart{Addr: "/tmp/n.sock"}},
		{"malformed", []any{[]any{"restart", []any{int64(3)}}}, nil},
	}
	for _, tt := range tests {
		if got := ParseRestart(tt.params); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: got %+v, want %+v", tt.name, got, tt.want)
		}
	}
}

package rpc

import (
	"errors"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/Shougo/ng/internal/msgpack"
)

// fakeServer answers "echo" with its params and "fail" with an error, and
// sends a notification when it receives "poke".
func fakeServer(t *testing.T, conn net.Conn) {
	t.Helper()
	dec := msgpack.NewDecoder(conn)
	enc := msgpack.NewEncoder(conn)
	go func() {
		for {
			v, err := dec.Decode()
			if err != nil {
				return
			}
			msg := v.([]any)
			kind, _ := msgpack.ToInt(msg[0])
			switch kind {
			case typeRequest:
				switch msg[2] {
				case "echo":
					enc.Encode([]any{int64(1), msg[1], nil, msg[3]})
				case "fail":
					enc.Encode([]any{int64(1), msg[1], []any{int64(0), "boom"}, nil})
				}
			case typeNotification:
				if msg[1] == "poke" {
					enc.Encode([]any{int64(2), "redraw", []any{[]any{"flush"}}})
				}
			case typeResponse:
				enc.Encode([]any{int64(2), "got-response", []any{msg[2], msg[3]}})
			}
		}
	}()
}

func newPair(t *testing.T, onNotify NotificationHandler, onRequest RequestHandler) (*Client, net.Conn) {
	a, b := net.Pipe()
	fakeServer(t, b)
	c := NewClient(a, onNotify, onRequest)
	go c.Serve()
	t.Cleanup(func() { c.Close(); b.Close() })
	return c, b
}

func TestCall(t *testing.T) {
	c, _ := newPair(t, nil, nil)
	got, err := c.Call("echo", "a", int64(1))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []any{"a", int64(1)}) {
		t.Errorf("got %#v", got)
	}
}

func TestCallError(t *testing.T) {
	c, _ := newPair(t, nil, nil)
	_, err := c.Call("fail")
	var e *Error
	if !errors.As(err, &e) || e.Message != "boom" {
		t.Errorf("got %v", err)
	}
}

func TestConcurrentCalls(t *testing.T) {
	c, _ := newPair(t, nil, nil)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got, err := c.Call("echo", int64(i))
			if err != nil || !reflect.DeepEqual(got, []any{int64(i)}) {
				t.Errorf("call %d: %#v %v", i, got, err)
			}
		}(i)
	}
	wg.Wait()
}

func TestNotification(t *testing.T) {
	ch := make(chan string, 1)
	c, _ := newPair(t, func(method string, params []any) { ch <- method }, nil)
	if err := c.Notify("poke"); err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-ch:
		if m != "redraw" {
			t.Errorf("got %q", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no notification")
	}
}

func TestCallAfterClose(t *testing.T) {
	c, server := newPair(t, nil, nil)
	server.Close()
	select {
	case <-c.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("client did not notice the closed connection")
	}
	if _, err := c.Call("echo"); err != ErrClosed {
		t.Errorf("got %v", err)
	}
}

func TestPendingCallFailsOnClose(t *testing.T) {
	a, b := net.Pipe()
	c := NewClient(a, nil, nil)
	go c.Serve()
	// Drain the request but never answer, then hang up.
	go func() {
		msgpack.NewDecoder(b).Decode()
		b.Close()
	}()
	if _, err := c.Call("never"); err != ErrClosed {
		t.Errorf("got %v", err)
	}
}

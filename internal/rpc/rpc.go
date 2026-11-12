// Package rpc implements a msgpack-rpc client for the Neovim API.
package rpc

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/Shougo/ng/internal/msgpack"
)

const (
	typeRequest      = 0
	typeResponse     = 1
	typeNotification = 2
)

// ErrClosed is returned by calls made after the connection has closed.
var ErrClosed = errors.New("rpc: connection closed")

// Error is an error returned by Neovim.
type Error struct {
	Type    int64
	Message string
}

func (e *Error) Error() string { return "nvim: " + e.Message }

// NotificationHandler receives notifications sent by Neovim. It runs on the
// connection's reader goroutine, so it must not block for long and must not
// make synchronous calls on the same client.
type NotificationHandler func(method string, params []any)

// RequestHandler answers requests sent by Neovim (rpcrequest()).
type RequestHandler func(method string, params []any) (any, error)

// Client is a msgpack-rpc client. It is safe for concurrent use.
type Client struct {
	conn io.ReadWriteCloser
	dec  *msgpack.Decoder

	wmu sync.Mutex
	enc *msgpack.Encoder

	mu      sync.Mutex
	nextID  uint32
	pending map[uint32]chan response
	closed  bool
	err     error

	onNotify  NotificationHandler
	onRequest RequestHandler
	done      chan struct{}
}

type response struct {
	result any
	err    error
}

// NewClient wraps conn. Call Serve (usually in a goroutine) to start
// reading messages.
func NewClient(conn io.ReadWriteCloser, onNotify NotificationHandler, onRequest RequestHandler) *Client {
	return &Client{
		conn:      conn,
		dec:       msgpack.NewDecoder(conn),
		enc:       msgpack.NewEncoder(conn),
		pending:   make(map[uint32]chan response),
		onNotify:  onNotify,
		onRequest: onRequest,
		done:      make(chan struct{}),
	}
}

// Done is closed when the connection ends.
func (c *Client) Done() <-chan struct{} { return c.done }

// Err reports why the connection ended, or nil if it ended with EOF.
func (c *Client) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// Serve reads messages until the connection fails or is closed.
func (c *Client) Serve() error {
	var err error
	for {
		var v any
		v, err = c.dec.Decode()
		if err != nil {
			break
		}
		if err = c.dispatch(v); err != nil {
			break
		}
	}
	if err == io.EOF {
		err = nil
	}
	c.shutdown(err)
	return err
}

func (c *Client) shutdown(err error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.err = err
	pending := c.pending
	c.pending = nil
	c.mu.Unlock()
	for _, ch := range pending {
		ch <- response{err: ErrClosed}
	}
	c.conn.Close()
	close(c.done)
}

// Close closes the connection.
func (c *Client) Close() error {
	c.shutdown(nil)
	return nil
}

func (c *Client) dispatch(v any) error {
	msg, ok := v.([]any)
	if !ok || len(msg) == 0 {
		return fmt.Errorf("rpc: malformed message %T", v)
	}
	kind, err := msgpack.ToInt(msg[0])
	if err != nil {
		return err
	}
	switch kind {
	case typeResponse:
		if len(msg) != 4 {
			return fmt.Errorf("rpc: malformed response")
		}
		id, err := msgpack.ToInt(msg[1])
		if err != nil {
			return err
		}
		c.mu.Lock()
		ch := c.pending[uint32(id)]
		delete(c.pending, uint32(id))
		c.mu.Unlock()
		if ch == nil {
			return nil // late response to an abandoned call
		}
		ch <- response{result: msg[3], err: toError(msg[2])}
	case typeNotification:
		if len(msg) != 3 {
			return fmt.Errorf("rpc: malformed notification")
		}
		method, _ := msg[1].(string)
		params, _ := msg[2].([]any)
		if c.onNotify != nil {
			c.onNotify(method, params)
		}
	case typeRequest:
		if len(msg) != 4 {
			return fmt.Errorf("rpc: malformed request")
		}
		id := msg[1]
		method, _ := msg[2].(string)
		params, _ := msg[3].([]any)
		// Answer on another goroutine so a slow handler cannot stall
		// the reader.
		go func() {
			var (
				result any
				rerr   any
			)
			if c.onRequest == nil {
				rerr = "ng: request handler not available: " + method
			} else if r, err := c.onRequest(method, params); err != nil {
				rerr = err.Error()
			} else {
				result = r
			}
			c.write([]any{int64(typeResponse), id, rerr, result})
		}()
	default:
		return fmt.Errorf("rpc: unknown message type %d", kind)
	}
	return nil
}

func toError(v any) error {
	switch e := v.(type) {
	case nil:
		return nil
	case []any:
		if len(e) == 2 {
			t, _ := msgpack.ToInt(e[0])
			m, _ := e[1].(string)
			return &Error{Type: t, Message: m}
		}
	case string:
		return &Error{Message: e}
	}
	return &Error{Message: fmt.Sprint(v)}
}

func (c *Client) write(msg []any) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	return c.enc.Encode(msg)
}

// Call invokes method and waits for the result.
func (c *Client) Call(method string, args ...any) (any, error) {
	ch := make(chan response, 1)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, ErrClosed
	}
	c.nextID++
	id := c.nextID
	c.pending[id] = ch
	c.mu.Unlock()

	if args == nil {
		args = []any{}
	}
	if err := c.write([]any{int64(typeRequest), int64(id), method, args}); err != nil {
		c.mu.Lock()
		if c.pending != nil {
			delete(c.pending, id)
		}
		c.mu.Unlock()
		return nil, err
	}
	r := <-ch
	return r.result, r.err
}

// Notify sends a notification without waiting for a reply.
func (c *Client) Notify(method string, args ...any) error {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return ErrClosed
	}
	if args == nil {
		args = []any{}
	}
	return c.write([]any{int64(typeNotification), method, args})
}

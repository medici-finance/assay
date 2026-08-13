package acp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"sync"
	"sync/atomic"
)

// wireMessage is the generic JSON-RPC 2.0 envelope. ACP frames exactly one
// message per line (newline-delimited JSON) over the agent subprocess's
// stdin/stdout -- there is no Content-Length header framing the way LSP
// uses. Confirmed against the live adapter during the Task 5 spike; see
// README.md.
//
// ACP is bidirectional: the agent sends both notifications (session/update)
// and its own requests (session/request_permission, fs/*, terminal/*) that
// the client must answer, so wireMessage covers all four shapes (our
// request, our notification, agent's request, agent's response) rather than
// splitting into separate request/response/notification types.
type wireMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *wireError      `json:"error,omitempty"`
}

type wireError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *wireError) Error() string {
	return fmt.Sprintf("acp: rpc error %d: %s", e.Code, e.Message)
}

// requestHandler serves a JSON-RPC request the agent sent us. Returns
// either a JSON-marshalable result or a *wireError, never both.
type requestHandler func(method string, params json.RawMessage) (result any, rpcErr *wireError)

// notifyHandler handles a JSON-RPC notification (no id) from the agent --
// only session/update in the flow this package drives.
type notifyHandler func(method string, params json.RawMessage)

// conn is the low-level bidirectional JSON-RPC 2.0 connection to the spawned
// agent process: encoding/json + bufio over the process's stdin/stdout
// pipes, stdlib only.
type conn struct {
	w   io.Writer
	wmu sync.Mutex

	nextID int64

	mu      sync.Mutex
	pending map[int64]chan wireMessage
	closed  bool
	err     error
	closeCh chan struct{}

	onRequest requestHandler
	onNotify  notifyHandler
}

func newConn(w io.Writer, r io.Reader, onRequest requestHandler, onNotify notifyHandler) *conn {
	c := &conn{
		w:         w,
		pending:   make(map[int64]chan wireMessage),
		closeCh:   make(chan struct{}),
		onRequest: onRequest,
		onNotify:  onNotify,
	}
	// bufio.Reader.ReadBytes has no line-length ceiling, unlike
	// bufio.Scanner's default 64KiB token limit -- observed ACP lines
	// (e.g. an available_commands_update carrying this environment's whole
	// skill roster) can run well past that.
	go c.readLoop(bufio.NewReaderSize(r, 64*1024))
	return c
}

func (c *conn) readLoop(r *bufio.Reader) {
	for {
		line, err := r.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			c.dispatch(line)
		}
		if err != nil {
			c.fail(err)
			return
		}
	}
}

func (c *conn) dispatch(line []byte) {
	var msg wireMessage
	if jerr := json.Unmarshal(bytes.TrimSpace(line), &msg); jerr != nil {
		// Spike-grade: a malformed line does not kill the connection, it is
		// dropped. A production client should count/log this.
		return
	}
	switch {
	case msg.ID != nil && msg.Method == "":
		// A response to one of our outgoing requests.
		c.deliver(msg)
	case msg.ID != nil && msg.Method != "":
		// A request FROM the agent (session/request_permission, fs/*, ...)
		// that we must answer.
		c.serve(msg)
	case msg.ID == nil && msg.Method != "":
		if c.onNotify != nil {
			c.onNotify(msg.Method, msg.Params)
		}
	}
}

func (c *conn) deliver(msg wireMessage) {
	var id int64
	if err := json.Unmarshal(msg.ID, &id); err != nil {
		return
	}
	c.mu.Lock()
	ch, ok := c.pending[id]
	if ok {
		delete(c.pending, id)
	}
	c.mu.Unlock()
	if ok {
		ch <- msg
	}
}

func (c *conn) serve(msg wireMessage) {
	if c.onRequest == nil {
		c.writeError(msg.ID, &wireError{Code: -32601, Message: "acp: no request handler installed"})
		return
	}
	result, rpcErr := c.onRequest(msg.Method, msg.Params)
	if rpcErr != nil {
		c.writeError(msg.ID, rpcErr)
		return
	}
	c.writeResult(msg.ID, result)
}

func (c *conn) writeResult(id json.RawMessage, result any) {
	rbytes, err := json.Marshal(result)
	if err != nil {
		c.writeError(id, &wireError{Code: -32603, Message: fmt.Sprintf("acp: marshal result: %v", err)})
		return
	}
	_ = c.write(wireMessage{JSONRPC: "2.0", ID: id, Result: rbytes})
}

func (c *conn) writeError(id json.RawMessage, e *wireError) {
	_ = c.write(wireMessage{JSONRPC: "2.0", ID: id, Error: e})
}

// call sends a JSON-RPC request and blocks for its response, the agent's
// close, or ctx cancellation, whichever comes first.
func (c *conn) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := atomic.AddInt64(&c.nextID, 1)
	pbytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("acp: marshal %s params: %w", method, err)
	}
	req := wireMessage{JSONRPC: "2.0", ID: json.RawMessage(strconv.FormatInt(id, 10)), Method: method, Params: pbytes}

	ch := make(chan wireMessage, 1)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, c.err
	}
	c.pending[id] = ch
	c.mu.Unlock()

	if err := c.write(req); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("acp: write %s: %w", method, err)
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	case <-c.closeCh:
		return nil, c.err
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	}
}

// notify sends a JSON-RPC notification (no id, no response expected).
func (c *conn) notify(method string, params any) error {
	pbytes, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("acp: marshal %s params: %w", method, err)
	}
	return c.write(wireMessage{JSONRPC: "2.0", Method: method, Params: pbytes})
}

func (c *conn) write(msg wireMessage) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if _, err := c.w.Write(b); err != nil {
		return err
	}
	_, err = c.w.Write([]byte("\n"))
	return err
}

// fail tears the connection down: every future call() fails fast, and every
// call() currently blocked in its select unblocks via closeCh.
func (c *conn) fail(err error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	if err == nil {
		err = ErrClosed
	}
	c.err = err
	c.pending = nil
	c.mu.Unlock()
	close(c.closeCh)
}

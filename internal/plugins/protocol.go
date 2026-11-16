package plugins

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mygo-player/pluginapi"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type request struct {
	Version int    `json:"version"`
	ID      uint64 `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
	Cancel  uint64 `json:"cancel,omitempty"`
}
type response struct {
	Version int             `json:"version"`
	ID      uint64          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *wireError      `json:"error,omitempty"`
}

// Client multiplexes requests over JSON lines. Close kills and reaps its worker.
type Client struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	mu        sync.Mutex
	next      uint64
	pending   map[uint64]chan response
	failure   error
	done      chan struct{}
	writes    chan request
	waited    chan struct{}
	closeOnce sync.Once
}

// Start discovers an installed package and launches its declared executable.
func Start(ctx context.Context, id, dir string) (*Client, error) {
	m, err := LoadManager(dir)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	manifest, ok := m.manifests[id]
	root := m.paths[id]
	m.mu.Unlock()
	if !ok {
		return nil, ErrUnsupported
	}
	return startPlugin(ctx, manifest, root, dir)
}
func Open(ctx context.Context, id, dir string) (*Client, error) { return Start(ctx, id, dir) }
func startPlugin(ctx context.Context, m Manifest, root, dataDir string) (*Client, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	executable, err := safeExecutable(root, m.Runtime.Executable)
	if err != nil {
		return nil, err
	}
	state := filepath.Join(dataDir, "plugin-data", m.ID)
	if err = os.MkdirAll(state, 0700); err != nil {
		return nil, err
	}
	cmd := exec.Command(executable, m.Runtime.Args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "MYGO_PLUGIN_DATA_DIR="+state)
	c, err := startCommand(cmd)
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var version int
	if err = c.call(pingCtx, "Ping", params{}, &version); err == nil && version != pluginapi.ProtocolVersion {
		err = fmt.Errorf("%w: Ping version %d", ErrProtocol, version)
	}
	if err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}
func startCommand(cmd *exec.Cmd) (*Client, error) {
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		in.Close()
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		in.Close()
		out.Close()
		return nil, err
	}
	c := &Client{cmd: cmd, stdin: in, pending: map[uint64]chan response{}, done: make(chan struct{}), writes: make(chan request, 128), waited: make(chan struct{})}
	go c.read(out)
	go func() {
		enc := json.NewEncoder(in)
		for {
			select {
			case <-c.done:
				return
			case r := <-c.writes:
				if err := enc.Encode(r); err != nil {
					c.fail(fmt.Errorf("%w: write: %v", ErrWorkerExited, err))
					return
				}
			}
		}
	}()
	go func() { err := cmd.Wait(); c.fail(fmt.Errorf("%w: %v", ErrWorkerExited, err)); close(c.waited) }()
	return c, nil
}
func (c *Client) fail(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failure != nil {
		return
	}
	c.failure = err
	close(c.done)
	c.pending = map[uint64]chan response{}
	_ = c.stdin.Close()
	_ = c.cmd.Process.Kill()
}
func (c *Client) read(r io.Reader) {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 4096), MaxMessageSize)
	for s.Scan() {
		var v response
		if err := json.Unmarshal(s.Bytes(), &v); err != nil || v.Version != ProtocolVersion || v.ID == 0 {
			c.fail(ErrProtocol)
			return
		}
		c.mu.Lock()
		ch := c.pending[v.ID]
		delete(c.pending, v.ID)
		c.mu.Unlock()
		if ch != nil {
			ch <- v
		}
	}
	if err := s.Err(); err != nil {
		c.fail(fmt.Errorf("%w: %v", ErrProtocol, err))
	} else {
		c.fail(ErrWorkerExited)
	}
}

// Invoke calls an optional extension without adding source-specific dependencies.
func (c *Client) Invoke(ctx context.Context, method string, p any, out any) error {
	return c.call(ctx, method, p, out)
}
func (c *Client) call(ctx context.Context, method string, p any, out any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	if c.failure != nil {
		err := c.failure
		c.mu.Unlock()
		return err
	}
	c.next++
	id := c.next
	ch := make(chan response, 1)
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() { c.mu.Lock(); delete(c.pending, id); c.mu.Unlock() }()
	req := request{Version: ProtocolVersion, ID: id, Method: method, Params: p}
	raw, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if len(raw) >= MaxMessageSize {
		return fmt.Errorf("%w: request too large", ErrProtocol)
	}
	select {
	case c.writes <- req:
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return c.failed()
	}
	select {
	case r := <-ch:
		if out != nil && len(r.Result) > 0 {
			if err := json.Unmarshal(r.Result, out); err != nil {
				return fmt.Errorf("%w: %v", ErrProtocol, err)
			}
		}
		return decodeError(r.Error)
	case <-ctx.Done():
		// Cancellation must survive a full outgoing queue, while the caller
		// still returns immediately. Closing the client releases this sender.
		go func() {
			select {
			case c.writes <- request{Version: ProtocolVersion, Cancel: id}:
			case <-c.done:
			}
		}()
		return ctx.Err()
	case <-c.done:
		return c.failed()
	}
}
func (c *Client) failed() error { c.mu.Lock(); defer c.mu.Unlock(); return c.failure }

// Close first delivers EOF so the plugin can cancel work and release sessions.
// Unresponsive plugins are killed after a bounded grace period and always reaped.
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		if c.failure == nil {
			c.failure = ErrClosed
			close(c.done)
			c.pending = map[uint64]chan response{}
		}
		c.mu.Unlock()
		_ = c.stdin.Close()
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case <-c.waited:
		case <-timer.C:
			_ = c.cmd.Process.Kill()
			<-c.waited
		}
	})
	<-c.waited
	return nil
}

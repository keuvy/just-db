package proc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type RunOptions struct {
	Path   string
	Args   []string
	Env    map[string]string
	Stdin  io.Reader
	Stdout io.Writer
}

type CommandError struct {
	Tool   string
	Err    error
	Stderr string
}

func (e *CommandError) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	if msg == "" {
		return fmt.Sprintf("%s: %v", e.Tool, e.Err)
	}
	if len(msg) > 4096 {
		msg = msg[len(msg)-4096:]
	}
	return fmt.Sprintf("%s: %v: %s", e.Tool, e.Err, msg)
}

func (e *CommandError) Unwrap() error { return e.Err }

func Run(ctx context.Context, opts RunOptions) error {
	if opts.Path == "" {
		return fmt.Errorf("executable path is empty")
	}
	cmd := exec.CommandContext(ctx, opts.Path, opts.Args...)
	cmd.Env = mergeEnv(opts.Env)
	cmd.Stdin = opts.Stdin
	if opts.Stdout != nil {
		cmd.Stdout = opts.Stdout
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return &CommandError{Tool: opts.Path, Err: err, Stderr: stderr.String()}
	}
	return nil
}

func mergeEnv(extra map[string]string) []string {
	base := os.Environ()
	if len(extra) == 0 {
		return base
	}
	index := make(map[string]int, len(base))
	for i, kv := range base {
		key, _, _ := strings.Cut(kv, "=")
		index[key] = i
	}
	for key, value := range extra {
		entry := key + "=" + value
		if i, ok := index[key]; ok {
			base[i] = entry
			continue
		}
		base = append(base, entry)
	}
	return base
}

// CountingWriter counts bytes written to w.
type CountingWriter struct {
	W     io.Writer
	mu    sync.Mutex
	Bytes int64
}

func (c *CountingWriter) Write(p []byte) (int, error) {
	n, err := c.W.Write(p)
	c.mu.Lock()
	c.Bytes += int64(n)
	c.mu.Unlock()
	return n, err
}

func (c *CountingWriter) Count() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Bytes
}

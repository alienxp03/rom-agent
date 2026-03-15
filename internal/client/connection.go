package client

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// Connection handles TCP connection to the game server
type Connection struct {
	conn     net.Conn
	mu       sync.RWMutex
	readMu   sync.Mutex
	writeMu  sync.Mutex
	readBuf  []byte
	writeBuf []byte
	closed   bool
	delayMs  uint32
}

// NewConnection creates a new connection instance
func NewConnection() *Connection {
	return &Connection{
		readBuf:  make([]byte, 65536),
		writeBuf: make([]byte, 65536),
		closed:   false,
	}
}

// Connect establishes a TCP connection to the specified address
func (c *Connection) Connect(ctx context.Context, addr string) error {
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}

	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	c.mu.Lock()
	c.conn = conn
	c.closed = false
	c.delayMs = uint32(time.Since(start).Milliseconds())
	c.mu.Unlock()

	return nil
}

// Delay returns the measured connect latency in milliseconds.
func (c *Connection) Delay() uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.delayMs
}

// Send writes data to the connection
func (c *Connection) Send(data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	c.mu.RLock()
	conn := c.conn
	closed := c.closed
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("connection is nil")
	}

	if closed {
		return fmt.Errorf("connection is closed")
	}

	n, err := conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	if n != len(data) {
		return fmt.Errorf("incomplete write: wrote %d bytes, expected %d", n, len(data))
	}

	return nil
}

// Recv reads data from the connection
func (c *Connection) Recv() ([]byte, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	c.mu.RLock()
	conn := c.conn
	closed := c.closed
	c.mu.RUnlock()

	if conn == nil {
		return nil, fmt.Errorf("connection is nil")
	}

	if closed {
		return nil, fmt.Errorf("connection is closed")
	}

	// Set read deadline to prevent indefinite blocking
	if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}

	n, err := conn.Read(c.readBuf)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return nil, nil // Timeout is not an error, just return nil
		}
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	if n == 0 {
		return nil, nil
	}

	// Return a copy to avoid buffer reuse issues
	result := make([]byte, n)
	copy(result, c.readBuf[:n])
	return result, nil
}

// Close closes the connection
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}

	c.closed = true
	err := c.conn.Close()
	c.conn = nil
	return err
}

// IsClosed returns true if the connection is closed
func (c *Connection) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// SetReadDeadline sets the read deadline for the connection
func (c *Connection) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("connection is nil")
	}

	return c.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline for the connection
func (c *Connection) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("connection is nil")
	}

	return c.conn.SetWriteDeadline(t)
}

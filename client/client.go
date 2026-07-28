package client

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"

	"github.com/negasus/haproxy-spoe-go/frame"
)

// Client is a simple client for spop protocol, this should only be used for testing purpose
type Client struct {
	conn         net.Conn
	reader       io.Reader
	maxFrameSize uint32
}

// NewClient create a new Client for an established connection
func NewClient(conn net.Conn) Client {
	return Client{
		conn:         conn,
		reader:       bufio.NewReader(conn),
		maxFrameSize: frame.DefaultMaxFrameSize,
	}
}

// Init initialize the client by sending the HaproxyHello frame
func (c *Client) Init() error {
	f := frame.AcquireFrame()
	defer frame.ReleaseFrame(f)
	f.Type = frame.TypeHaproxyHello
	f.StreamID = 0
	f.FrameID = 0
	f.KV.Add("supported-versions", "2")
	f.KV.Add("max-frame-size", c.maxFrameSize)
	f.KV.Add("capabilities", "")

	err := c.send(f)
	if err != nil {
		return err
	}

	responseFrame := frame.AcquireFrame()
	defer frame.ReleaseFrame(responseFrame)
	if err := responseFrame.ReadWithLimit(c.reader, c.maxFrameSize); err != nil {
		return err
	}

	switch responseFrame.Type {
	case frame.TypeAgentHello:
		if responseFrame.FrameID != uint64(0) || responseFrame.StreamID != uint64(0) {
			return fmt.Errorf("FrameID or StreamID mismatch")
		}
		if responseFrame.MaxFrameSize > 0 && responseFrame.MaxFrameSize < c.maxFrameSize {
			c.maxFrameSize = responseFrame.MaxFrameSize
		}
	default:
		return fmt.Errorf("unexpected frame type: %v", responseFrame.Type)
	}

	return nil

}

func (c *Client) send(f *frame.Frame) error {
	buf := bytes.NewBuffer(make([]byte, 0))
	n, err := f.EncodeWithLimit(buf, c.maxFrameSize)
	if err != nil {
		return err
	}
	written := 0
	for written < n {
		nn, err := c.conn.Write(buf.Bytes()[written:])
		if err != nil {
			return err
		}
		if nn <= 0 {
			return fmt.Errorf("short write")
		}
		written += nn
	}
	return nil
}

// Notify send an empty Notify frame
func (c *Client) Notify() error {
	f := frame.AcquireFrame()
	defer frame.ReleaseFrame(f)
	f.Type = frame.TypeNotify
	f.StreamID = 1
	f.FrameID = 1

	err := c.send(f)
	if err != nil {
		return err
	}

	responseFrame := frame.AcquireFrame()
	defer frame.ReleaseFrame(responseFrame)
	return responseFrame.ReadWithLimit(c.reader, c.maxFrameSize)
}

// Stop the client by sending HaproxyDisconnect frame
func (c *Client) Stop() error {
	f := frame.AcquireFrame()
	defer frame.ReleaseFrame(f)
	f.Type = frame.TypeHaproxyDisconnect
	f.StreamID = 0
	f.FrameID = 0
	f.KV.Add("status-code", uint32(0))
	f.KV.Add("message", "normal")

	err := c.send(f)
	if err != nil {
		return err
	}

	responseFrame := frame.AcquireFrame()
	defer frame.ReleaseFrame(responseFrame)
	return responseFrame.ReadWithLimit(c.reader, c.maxFrameSize)
}

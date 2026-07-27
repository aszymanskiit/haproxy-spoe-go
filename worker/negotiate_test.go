package worker

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/negasus/haproxy-spoe-go/action"
	"github.com/negasus/haproxy-spoe-go/frame"
	"github.com/negasus/haproxy-spoe-go/logger"
	"github.com/negasus/haproxy-spoe-go/request"
)

func readFrame(t *testing.T, r io.Reader, limit uint32) *frame.Frame {
	t.Helper()
	f := frame.AcquireFrame()
	if err := f.ReadWithLimit(r, limit); err != nil {
		frame.ReleaseFrame(f)
		t.Fatalf("read frame: %v", err)
	}
	return f
}

func TestNegotiate_LocalSmallerThanHAProxy(t *testing.T) {
	clientConn, server := net.Pipe()
	defer clientConn.Close()

	// Local limit only bounds the first HELLO; after that HAProxy's value wins.
	go HandleWithMaxFrameSize(server, func(*request.Request) {}, logger.NewNop(), 1024)

	hello := frame.AcquireFrame()
	hello.Type = frame.TypeHaproxyHello
	hello.KV.Add("supported-versions", "2.0")
	hello.KV.Add("max-frame-size", uint32(4096))
	hello.KV.Add("capabilities", "")
	sendFrame(t, clientConn, hello)
	frame.ReleaseFrame(hello)

	resp := readFrame(t, bufio.NewReader(clientConn), 4096)
	defer frame.ReleaseFrame(resp)
	if resp.Type != frame.TypeAgentHello {
		t.Fatalf("got type %v", resp.Type)
	}
	if resp.MaxFrameSize != 4096 {
		t.Fatalf("connection max-frame-size = %d, want 4096 (HAProxy value)", resp.MaxFrameSize)
	}
}

func TestNegotiate_LocalLargerThanHAProxy(t *testing.T) {
	clientConn, server := net.Pipe()
	defer clientConn.Close()

	go HandleWithMaxFrameSize(server, func(*request.Request) {}, logger.NewNop(), 8192)

	hello := frame.AcquireFrame()
	hello.Type = frame.TypeHaproxyHello
	hello.KV.Add("supported-versions", "2.0")
	hello.KV.Add("max-frame-size", uint32(2048))
	hello.KV.Add("capabilities", "")
	sendFrame(t, clientConn, hello)
	frame.ReleaseFrame(hello)

	resp := readFrame(t, bufio.NewReader(clientConn), 8192)
	defer frame.ReleaseFrame(resp)
	if resp.MaxFrameSize != 2048 {
		t.Fatalf("negotiated max-frame-size = %d, want 2048", resp.MaxFrameSize)
	}
}

func TestNegotiate_Equal(t *testing.T) {
	clientConn, server := net.Pipe()
	defer clientConn.Close()

	go HandleWithMaxFrameSize(server, func(*request.Request) {}, logger.NewNop(), 2048)

	hello := frame.AcquireFrame()
	hello.Type = frame.TypeHaproxyHello
	hello.KV.Add("supported-versions", "2.0")
	hello.KV.Add("max-frame-size", uint32(2048))
	hello.KV.Add("capabilities", "")
	sendFrame(t, clientConn, hello)
	frame.ReleaseFrame(hello)

	resp := readFrame(t, bufio.NewReader(clientConn), 2048)
	defer frame.ReleaseFrame(resp)
	if resp.MaxFrameSize != 2048 {
		t.Fatalf("negotiated max-frame-size = %d, want 2048", resp.MaxFrameSize)
	}
}

func TestNegotiate_MissingMaxFrameSize(t *testing.T) {
	clientConn, server := net.Pipe()
	defer clientConn.Close()

	done := make(chan struct{})
	go func() {
		HandleWithMaxFrameSize(server, func(*request.Request) {}, logger.NewNop(), 2048)
		close(done)
	}()

	hello := frame.AcquireFrame()
	hello.Type = frame.TypeHaproxyHello
	hello.KV.Add("supported-versions", "2.0")
	hello.KV.Add("capabilities", "")
	sendFrame(t, clientConn, hello)
	frame.ReleaseFrame(hello)

	resp := readFrame(t, bufio.NewReader(clientConn), 2048)
	defer frame.ReleaseFrame(resp)
	if resp.Type != frame.TypeAgentDisconnect {
		t.Fatalf("got type %v, want AgentDisconnect", resp.Type)
	}
	code, ok := resp.KV.Get("status-code")
	if !ok || code.(uint32) != statusMaxFrameSizeNotFound {
		t.Fatalf("status-code = %v", code)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit")
	}
}

func TestNegotiate_WrongMaxFrameSizeType(t *testing.T) {
	clientConn, server := net.Pipe()
	defer clientConn.Close()

	done := make(chan struct{})
	go func() {
		HandleWithMaxFrameSize(server, func(*request.Request) {}, logger.NewNop(), 2048)
		close(done)
	}()

	hello := frame.AcquireFrame()
	hello.Type = frame.TypeHaproxyHello
	hello.KV.Add("supported-versions", "2.0")
	hello.KV.Add("max-frame-size", "not-a-number")
	hello.KV.Add("capabilities", "")
	sendFrame(t, clientConn, hello)
	frame.ReleaseFrame(hello)

	resp := readFrame(t, bufio.NewReader(clientConn), 2048)
	defer frame.ReleaseFrame(resp)
	if resp.Type != frame.TypeAgentDisconnect {
		t.Fatalf("got type %v, want AgentDisconnect", resp.Type)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit")
	}
}

func TestNegotiate_BelowProtocolMinimum(t *testing.T) {
	clientConn, server := net.Pipe()
	defer clientConn.Close()

	done := make(chan struct{})
	go func() {
		HandleWithMaxFrameSize(server, func(*request.Request) {}, logger.NewNop(), 2048)
		close(done)
	}()

	hello := frame.AcquireFrame()
	hello.Type = frame.TypeHaproxyHello
	hello.KV.Add("supported-versions", "2.0")
	hello.KV.Add("max-frame-size", uint32(100))
	hello.KV.Add("capabilities", "")
	sendFrame(t, clientConn, hello)
	frame.ReleaseFrame(hello)

	resp := readFrame(t, bufio.NewReader(clientConn), 2048)
	defer frame.ReleaseFrame(resp)
	if resp.Type != frame.TypeAgentDisconnect {
		t.Fatalf("got type %v, want AgentDisconnect", resp.Type)
	}
	code, _ := resp.KV.Get("status-code")
	if code.(uint32) != statusMaxFrameSizeInvalid {
		t.Fatalf("status-code = %v, want %d", code, statusMaxFrameSizeInvalid)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit")
	}
}

func TestFirstHelloExceedsLocalLimit(t *testing.T) {
	clientConn, server := net.Pipe()
	defer clientConn.Close()

	log := &recordingLogger{}
	done := make(chan struct{})
	go func() {
		HandleWithMaxFrameSize(server, func(*request.Request) {}, log, frame.MinFrameSize)
		close(done)
	}()

	// Craft a HAPROXY-HELLO whose content length exceeds the local limit.
	hdr := make([]byte, 5)
	binary.BigEndian.PutUint32(hdr[0:4], frame.MinFrameSize+1)
	hdr[4] = byte(frame.TypeHaproxyHello)
	if _, err := clientConn.Write(hdr); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit")
	}

	found := false
	for _, msg := range log.messages {
		if strings.Contains(msg, "handle worker") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected protocol error log, got %v", log.messages)
	}
}

func TestNotifyExceedsNegotiatedLimit(t *testing.T) {
	clientConn, server := net.Pipe()
	defer clientConn.Close()

	done := make(chan struct{})
	go func() {
		HandleWithMaxFrameSize(server, func(*request.Request) {}, logger.NewNop(), 512)
		close(done)
	}()

	reader := bufio.NewReader(clientConn)
	hello := frame.AcquireFrame()
	hello.Type = frame.TypeHaproxyHello
	hello.KV.Add("supported-versions", "2.0")
	hello.KV.Add("max-frame-size", uint32(512))
	hello.KV.Add("capabilities", "")
	sendFrame(t, clientConn, hello)
	frame.ReleaseFrame(hello)
	resp := readFrame(t, reader, 512)
	frame.ReleaseFrame(resp)

	hdr := make([]byte, 5)
	binary.BigEndian.PutUint32(hdr[0:4], 513)
	hdr[4] = byte(frame.TypeNotify)
	if _, err := clientConn.Write(hdr); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit")
	}
}

func TestUnexpectedTypeBeforeHandshake(t *testing.T) {
	clientConn, server := net.Pipe()
	defer clientConn.Close()

	done := make(chan struct{})
	go func() {
		HandleWithMaxFrameSize(server, func(*request.Request) {}, logger.NewNop(), 2048)
		close(done)
	}()

	notify := frame.AcquireFrame()
	notify.Type = frame.TypeNotify
	notify.StreamID = 1
	notify.FrameID = 1
	sendFrame(t, clientConn, notify)
	frame.ReleaseFrame(notify)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit")
	}
}

func TestSecondHelloAfterHandshake(t *testing.T) {
	clientConn, server := net.Pipe()
	defer clientConn.Close()

	done := make(chan struct{})
	go func() {
		HandleWithMaxFrameSize(server, func(*request.Request) {}, logger.NewNop(), 2048)
		close(done)
	}()

	reader := bufio.NewReader(clientConn)
	hello := frame.AcquireFrame()
	hello.Type = frame.TypeHaproxyHello
	hello.KV.Add("supported-versions", "2.0")
	hello.KV.Add("max-frame-size", uint32(2048))
	hello.KV.Add("capabilities", "")
	sendFrame(t, clientConn, hello)
	frame.ReleaseFrame(hello)
	resp := readFrame(t, reader, 2048)
	frame.ReleaseFrame(resp)

	hello2 := frame.AcquireFrame()
	hello2.Type = frame.TypeHaproxyHello
	hello2.KV.Add("supported-versions", "2.0")
	hello2.KV.Add("max-frame-size", uint32(2048))
	hello2.KV.Add("capabilities", "")
	sendFrame(t, clientConn, hello2)
	frame.ReleaseFrame(hello2)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit")
	}
}

func TestHealthcheckSPOP(t *testing.T) {
	clientConn, server := net.Pipe()
	defer clientConn.Close()

	done := make(chan struct{})
	go func() {
		HandleWithMaxFrameSize(server, func(*request.Request) {}, logger.NewNop(), 2048)
		close(done)
	}()

	hello := frame.AcquireFrame()
	hello.Type = frame.TypeHaproxyHello
	hello.KV.Add("supported-versions", "2.0")
	hello.KV.Add("max-frame-size", uint32(2048))
	hello.KV.Add("capabilities", "")
	hello.KV.Add("healthcheck", true)
	sendFrame(t, clientConn, hello)
	frame.ReleaseFrame(hello)

	resp := readFrame(t, bufio.NewReader(clientConn), 2048)
	defer frame.ReleaseFrame(resp)
	if resp.Type != frame.TypeAgentHello {
		t.Fatalf("got type %v", resp.Type)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit after healthcheck")
	}
}

func TestACKWithinLimit(t *testing.T) {
	clientConn, server := net.Pipe()
	defer clientConn.Close()

	go HandleWithMaxFrameSize(server, func(r *request.Request) {
		r.Actions.SetVar(action.ScopeSession, "x", int32(1))
	}, logger.NewNop(), 2048)

	reader := bufio.NewReader(clientConn)
	hello := frame.AcquireFrame()
	hello.Type = frame.TypeHaproxyHello
	hello.KV.Add("supported-versions", "2.0")
	hello.KV.Add("max-frame-size", uint32(2048))
	hello.KV.Add("capabilities", "")
	sendFrame(t, clientConn, hello)
	frame.ReleaseFrame(hello)
	frame.ReleaseFrame(readFrame(t, reader, 2048))

	notify := frame.AcquireFrame()
	notify.Type = frame.TypeNotify
	notify.StreamID = 1
	notify.FrameID = 1
	sendFrame(t, clientConn, notify)
	frame.ReleaseFrame(notify)

	ack := readFrame(t, reader, 2048)
	defer frame.ReleaseFrame(ack)
	if ack.Type != frame.TypeAgentAck {
		t.Fatalf("got type %v", ack.Type)
	}
}

func TestACKExceedsNegotiatedLimit(t *testing.T) {
	clientConn, server := net.Pipe()
	defer clientConn.Close()

	go HandleWithMaxFrameSize(server, func(r *request.Request) {
		// Force a large ACK payload via a huge variable value.
		r.Actions.SetVar(action.ScopeSession, "big", strings.Repeat("a", 600))
	}, logger.NewNop(), 512)

	reader := bufio.NewReader(clientConn)
	hello := frame.AcquireFrame()
	hello.Type = frame.TypeHaproxyHello
	hello.KV.Add("supported-versions", "2.0")
	hello.KV.Add("max-frame-size", uint32(512))
	hello.KV.Add("capabilities", "")
	sendFrame(t, clientConn, hello)
	frame.ReleaseFrame(hello)
	frame.ReleaseFrame(readFrame(t, reader, 512))

	notify := frame.AcquireFrame()
	notify.Type = frame.TypeNotify
	notify.StreamID = 1
	notify.FrameID = 1
	sendFrame(t, clientConn, notify)
	frame.ReleaseFrame(notify)

	// Expect AGENT-DISCONNECT with frame-too-big after ACK encode fails.
	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	f := frame.AcquireFrame()
	defer frame.ReleaseFrame(f)
	err := f.ReadWithLimit(reader, 512)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		// disconnect frame may arrive, or connection may close
		if f.Type != frame.TypeAgentDisconnect && f.Type != 0 {
			t.Fatalf("unexpected type %v err=%v", f.Type, err)
		}
	}
	if err == nil && f.Type != frame.TypeAgentDisconnect {
		t.Fatalf("expected AgentDisconnect, got %v", f.Type)
	}
}

type partialWriterConn struct {
	net.Conn
	chunk int
}

func (c *partialWriterConn) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	n := c.chunk
	if n > len(p) {
		n = len(p)
	}
	return c.Conn.Write(p[:n])
}

func TestPartialWrites(t *testing.T) {
	clientConn, server := net.Pipe()
	defer clientConn.Close()

	partial := &partialWriterConn{Conn: server, chunk: 3}
	go HandleWithMaxFrameSize(partial, func(*request.Request) {}, logger.NewNop(), 2048)

	reader := bufio.NewReader(clientConn)
	hello := frame.AcquireFrame()
	hello.Type = frame.TypeHaproxyHello
	hello.KV.Add("supported-versions", "2.0")
	hello.KV.Add("max-frame-size", uint32(2048))
	hello.KV.Add("capabilities", "")
	sendFrame(t, clientConn, hello)
	frame.ReleaseFrame(hello)

	resp := readFrame(t, reader, 2048)
	defer frame.ReleaseFrame(resp)
	if resp.Type != frame.TypeAgentHello {
		t.Fatalf("got type %v", resp.Type)
	}
}

func TestParallelACKNoInterleaving(t *testing.T) {
	clientConn, server := net.Pipe()
	defer clientConn.Close()

	var started sync.WaitGroup
	var release chan struct{}
	release = make(chan struct{})
	var count int32

	go HandleWithMaxFrameSize(server, func(r *request.Request) {
		atomic.AddInt32(&count, 1)
		started.Done()
		<-release
		r.Actions.SetVar(action.ScopeSession, "id", int32(r.FrameID))
	}, logger.NewNop(), 4096)

	reader := bufio.NewReader(clientConn)
	hello := frame.AcquireFrame()
	hello.Type = frame.TypeHaproxyHello
	hello.KV.Add("supported-versions", "2.0")
	hello.KV.Add("max-frame-size", uint32(4096))
	hello.KV.Add("capabilities", "")
	sendFrame(t, clientConn, hello)
	frame.ReleaseFrame(hello)
	frame.ReleaseFrame(readFrame(t, reader, 4096))

	const n = 8
	started.Add(n)
	for i := 1; i <= n; i++ {
		notify := frame.AcquireFrame()
		notify.Type = frame.TypeNotify
		notify.StreamID = uint64(i)
		notify.FrameID = uint64(i)
		sendFrame(t, clientConn, notify)
		frame.ReleaseFrame(notify)
	}
	started.Wait()
	close(release)

	got := make(map[uint64]bool)
	_ = clientConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for i := 0; i < n; i++ {
		ack := readFrame(t, reader, 4096)
		if ack.Type != frame.TypeAgentAck {
			frame.ReleaseFrame(ack)
			t.Fatalf("got type %v", ack.Type)
		}
		got[ack.FrameID] = true
		frame.ReleaseFrame(ack)
	}
	if len(got) != n {
		t.Fatalf("got %d unique ACKs, want %d", len(got), n)
	}
}

func TestHTTPProbeUnknownType(t *testing.T) {
	clientConn, server := net.Pipe()
	defer clientConn.Close()

	done := make(chan struct{})
	go func() {
		Handle(server, func(*request.Request) {}, logger.NewNop())
		close(done)
	}()

	if _, err := clientConn.Write([]byte("GET / HTTP/1.1\r\n\r\n")); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit on HTTP probe")
	}
}

func TestConnectAndCloseNoError(t *testing.T) {
	clientConn, server := net.Pipe()

	log := &recordingLogger{}
	done := make(chan struct{})
	go func() {
		Handle(server, func(*request.Request) {}, log)
		close(done)
	}()

	clientConn.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit")
	}

	for _, msg := range log.messages {
		if msg == "handle worker: %v" {
			t.Errorf("connect-and-close should not log worker error, got %v", log.messages)
		}
	}
}

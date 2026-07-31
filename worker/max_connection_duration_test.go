package worker

import (
	"bufio"
	"net"
	"testing"
	"time"

	"github.com/aszymanskiit/haproxy-spoe-go/frame"
	"github.com/aszymanskiit/haproxy-spoe-go/logger"
	"github.com/aszymanskiit/haproxy-spoe-go/request"
)

func TestJitteredMaxConnectionDuration_Range(t *testing.T) {
	base := 10 * time.Second
	min := base
	max := 13 * time.Second

	for i := 0; i < 200; i++ {
		got := jitteredMaxConnectionDuration(base)
		if got < min || got > max {
			t.Fatalf("duration out of range: got %s, want [%s, %s]", got, min, max)
		}
	}

	if got := jitteredMaxConnectionDuration(0); got != 0 {
		t.Fatalf("got %s, want 0 for base=0", got)
	}
}

func TestWorker_MaxConnectionDuration_GracefulDisconnectWithInFlightNotify(t *testing.T) {
	clientConn, server := net.Pipe()
	defer clientConn.Close()

	processedCh := make(chan struct{}, 1)
	handler := func(r *request.Request) {
		time.Sleep(250 * time.Millisecond)
		processedCh <- struct{}{}
	}

	baseLifetime := 120 * time.Millisecond
	startedAt := time.Now()
	go HandleWithOptions(server, handler, Options{
		MaxFrameSize:          16 * 1024,
		MaxConnectionDuration: baseLifetime,
		Logger:                logger.NewNop(),
	})

	reader := bufio.NewReader(clientConn)

	hello := frame.AcquireFrame()
	hello.Type = frame.TypeHaproxyHello
	hello.StreamID = 0
	hello.FrameID = 0
	hello.KV.Add("supported-versions", "2")
	hello.KV.Add("max-frame-size", uint32(16*1024))
	hello.KV.Add("capabilities", "")
	sendFrame(t, clientConn, hello)
	frame.ReleaseFrame(hello)

	resp := frame.AcquireFrame()
	if err := resp.ReadWithLimit(reader, 16*1024); err != nil {
		t.Fatalf("read AgentHello: %v", err)
	}
	if resp.Type != frame.TypeAgentHello {
		t.Fatalf("unexpected response type: got %v, want AgentHello", resp.Type)
	}
	frame.ReleaseFrame(resp)

	notify := frame.AcquireFrame()
	notify.Type = frame.TypeNotify
	notify.StreamID = 1
	notify.FrameID = 1
	sendFrame(t, clientConn, notify)
	frame.ReleaseFrame(notify)

	gotAck := false
	gotDisconnect := false
	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for i := 0; i < 3; i++ {
		f := frame.AcquireFrame()
		if err := f.ReadWithLimit(reader, 16*1024); err != nil {
			frame.ReleaseFrame(f)
			break
		}
		switch f.Type {
		case frame.TypeAgentAck:
			gotAck = true
		case frame.TypeAgentDisconnect:
			gotDisconnect = true
		}
		frame.ReleaseFrame(f)
		if gotAck && gotDisconnect {
			break
		}
	}

	if !gotDisconnect {
		t.Fatal("expected AgentDisconnect after max connection duration")
	}
	if !gotAck {
		t.Fatal("expected AgentAck for in-flight Notify before connection close")
	}

	select {
	case <-processedCh:
	case <-time.After(1 * time.Second):
		t.Fatal("notify handler did not finish")
	}

	if elapsed := time.Since(startedAt); elapsed < baseLifetime {
		t.Fatalf("connection closed too early: elapsed %s, min %s", elapsed, baseLifetime)
	}
}

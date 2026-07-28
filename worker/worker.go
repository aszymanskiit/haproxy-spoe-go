package worker

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"syscall"

	"github.com/negasus/haproxy-spoe-go/frame"
	"github.com/negasus/haproxy-spoe-go/logger"
	"github.com/negasus/haproxy-spoe-go/request"
)

const (
	capabilities = "pipelining,async"
)

// SPOP error status codes (HAProxy SPOE.txt section "Errors & timeouts")
const (
	statusNormal               uint32 = 0
	statusFrameTooBig          uint32 = 3
	statusMaxFrameSizeNotFound uint32 = 6
	statusMaxFrameSizeInvalid  uint32 = 9
)

// Handle listens on conn and processes SPOP frames using frame.DefaultMaxFrameSize
// as the local safety limit for the first HAPROXY-HELLO only. After HELLO, the
// peer's max-frame-size is used for the connection
func Handle(conn net.Conn, handler func(*request.Request), logger logger.Logger) {
	HandleWithMaxFrameSize(conn, handler, logger, frame.DefaultMaxFrameSize)
}

// HandleWithMaxFrameSize is like Handle but uses an explicit local max-frame-size
// for reading the first HAPROXY-HELLO. Invalid values fall back to
// frame.DefaultMaxFrameSize
func HandleWithMaxFrameSize(conn net.Conn, handler func(*request.Request), logger logger.Logger, maxFrameSize uint32) {
	if maxFrameSize == 0 || frame.ValidateMaxFrameSize(maxFrameSize) != nil {
		maxFrameSize = frame.DefaultMaxFrameSize
	}

	w := &worker{
		conn:              conn,
		handler:           handler,
		logger:            logger,
		localMaxFrameSize: maxFrameSize,
		maxFrameSize:      maxFrameSize,
	}

	if err := w.run(); err != nil {
		logger.Errorf("handle worker: %v", err)
	}
}

type writeRequest struct {
	data []byte
	errc chan error
}

type worker struct {
	conn     net.Conn
	ready    bool
	engineID string
	handler  func(*request.Request)

	logger logger.Logger

	localMaxFrameSize uint32
	maxFrameSize      uint32

	// writeCh serializes all outbound frames on this connection so concurrent
	// ACK goroutines cannot interleave bytes on the TCP stream
	writeCh chan writeRequest
	wg      sync.WaitGroup
}

func (w *worker) close() {
	if err := w.conn.Close(); err != nil {
		w.logger.Errorf("close connection: %v", err)
	}
}

func (w *worker) writeLoop(done chan struct{}) {
	defer close(done)
	for req := range w.writeCh {
		req.errc <- writeFull(w.conn, req.data)
	}
}

func (w *worker) run() error {
	w.writeCh = make(chan writeRequest)
	writerDone := make(chan struct{})
	go w.writeLoop(writerDone)

	defer func() {
		// Wait for in-flight notify handlers (and their writes) to finish,
		// then stop the writer before closing the connection
		w.wg.Wait()
		close(w.writeCh)
		<-writerDone
		w.close()
	}()

	var f *frame.Frame

	buf := bufio.NewReader(w.conn)

	for {
		f = frame.AcquireFrame()

		if err := f.ReadWithLimit(buf, w.maxFrameSize); err != nil {
			frame.ReleaseFrame(f)
			if isConnectionClose(err) {
				return nil
			}
			return fmt.Errorf("error read frame: %w", err)
		}

		switch f.Type {
		case frame.TypeHaproxyHello:
			if w.ready {
				frame.ReleaseFrame(f)
				return fmt.Errorf("worker already ready, but got HaproxyHello frame")
			}

			if err := w.sendAgentHello(f); err != nil {
				frame.ReleaseFrame(f)
				return fmt.Errorf("error send AgentHello frame: %w", err)
			}

			if f.Healthcheck {
				frame.ReleaseFrame(f)
				return nil
			}

			w.engineID = f.EngineID
			w.ready = true
			frame.ReleaseFrame(f)
			continue

		case frame.TypeHaproxyDisconnect:
			if !w.ready {
				frame.ReleaseFrame(f)
				return fmt.Errorf("worker not ready, but got HaproxyDisconnect frame")
			}

			if err := w.sendAgentDisconnect(0, 0, statusNormal, "connection closed by server"); err != nil {
				frame.ReleaseFrame(f)
				return fmt.Errorf("error send AgentDisconnect frame: %w", err)
			}
			frame.ReleaseFrame(f)
			return nil

		case frame.TypeNotify:
			if !w.ready {
				frame.ReleaseFrame(f)
				return fmt.Errorf("worker not ready, but got Notify frame")
			}

			w.wg.Add(1)
			go w.processNotifyFrame(f)

		default:
			ft := f.Type
			frame.ReleaseFrame(f)
			if !w.ready {
				return fmt.Errorf("unexpected frame type before handshake: %v", ft)
			}
			w.logger.Errorf("unexpected frame type: %v", ft)
		}
	}
}

// isConnectionClose reports whether err indicates the peer closed the
// connection. HAProxy 3.x's mux_spop tears down TCP connections without
// sending a DISCONNECT frame when all SPOP streams are done, resulting
// in ECONNRESET or EPIPE instead of io.EOF.
func isConnectionClose(err error) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) {
		return true
	}
	var netErr *net.OpError
	return errors.As(err, &netErr) && !netErr.Temporary()
}

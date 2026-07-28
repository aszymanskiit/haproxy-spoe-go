package worker

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/negasus/haproxy-spoe-go/frame"
	"github.com/negasus/haproxy-spoe-go/logger"
	"github.com/negasus/haproxy-spoe-go/request"
)

const (
	capabilities = "pipelining,async"
)

type Options struct {
	// MaxFrameSize is a local safety limit used only for reading the first
	// HAPROXY-HELLO. After HELLO negotiation, peer's max-frame-size is used.
	MaxFrameSize uint32

	// MaxConnectionDuration enables graceful worker-side connection rotation.
	// When > 0, worker keeps the connection for at least this duration, plus
	// a random jitter of up to 30% of the base value to avoid synchronized
	// reconnect storms.
	//
	// When 0, connection lifetime is unlimited (current behavior).
	MaxConnectionDuration time.Duration

	// Logger is used for error reporting. When nil, a no-op logger is used.
	Logger logger.Logger
}

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
func Handle(conn net.Conn, handler func(*request.Request), log logger.Logger) {
	HandleWithOptions(conn, handler, Options{MaxFrameSize: frame.DefaultMaxFrameSize, Logger: log})
}

// HandleWithMaxFrameSize is like Handle but uses an explicit local max-frame-size
// for reading the first HAPROXY-HELLO. Invalid values fall back to
// frame.DefaultMaxFrameSize
func HandleWithMaxFrameSize(conn net.Conn, handler func(*request.Request), log logger.Logger, maxFrameSize uint32) {
	HandleWithOptions(conn, handler, Options{MaxFrameSize: maxFrameSize, Logger: log})
}

// HandleWithOptions is like Handle but allows configuring all options including
// the initial HELLO safety limit and optional max connection duration.
func HandleWithOptions(conn net.Conn, handler func(*request.Request), opts Options) {
	log := opts.Logger
	if log == nil {
		log = logger.NewNop()
	}

	maxFrameSize := opts.MaxFrameSize
	if maxFrameSize == 0 || frame.ValidateMaxFrameSize(maxFrameSize) != nil {
		maxFrameSize = frame.DefaultMaxFrameSize
	}

	var disconnectAt time.Time
	if opts.MaxConnectionDuration > 0 {
		disconnectAt = time.Now().Add(jitteredMaxConnectionDuration(opts.MaxConnectionDuration))
	}

	w := &worker{
		conn:              conn,
		handler:           handler,
		logger:            log,
		localMaxFrameSize: maxFrameSize,
		maxFrameSize:      maxFrameSize,
		disconnectAt:      disconnectAt,
	}

	if err := w.run(); err != nil {
		log.Errorf("handle worker: %v", err)
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
	disconnectAt      time.Time

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

	if !w.disconnectAt.IsZero() {
		if err := w.conn.SetReadDeadline(w.disconnectAt); err != nil {
			return fmt.Errorf("set read deadline: %w", err)
		}
	}

	for {
		f = frame.AcquireFrame()

		if err := f.ReadWithLimit(buf, w.maxFrameSize); err != nil {
			frame.ReleaseFrame(f)

			if w.ready && w.isConnectionLifetimeExceeded(err) {
				if err := w.sendAgentDisconnect(0, 0, statusNormal, "max connection lifetime reached"); err != nil {
					return fmt.Errorf("error send AgentDisconnect frame: %w", err)
				}
				return nil
			}

			if isConnectionClose(err) {
				return nil
			}
			return fmt.Errorf("error read frame: %w", err)
		}

		switch f.Type {
		case frame.TypeHaproxyHello:
			if ok, err := w.processHaproxyHello(f); !ok {
				return err
			}
			continue

		case frame.TypeHaproxyDisconnect:
			return w.processHaproxyDisconnect(f)

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

func (w *worker) isConnectionLifetimeExceeded(err error) bool {
	if w.disconnectAt.IsZero() || time.Now().Before(w.disconnectAt) {
		return false
	}
	// Deadline may fire mid-frame: ReadWithLimit can return a timeout error
	// (clean case) or io.ErrUnexpectedEOF / io.EOF if bufio already buffered
	// a partial frame header before the deadline was reached.
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF)
}

func jitteredMaxConnectionDuration(base time.Duration) time.Duration {
	if base <= 0 {
		return 0
	}

	extra := time.Duration(rand.Int64N(int64(base*30/100) + 1))

	return base + extra
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

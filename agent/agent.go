package agent

import (
	"fmt"
	"net"
	"time"

	"github.com/aszymanskiit/haproxy-spoe-go/frame"
	"github.com/aszymanskiit/haproxy-spoe-go/logger"
	"github.com/aszymanskiit/haproxy-spoe-go/request"
	"github.com/aszymanskiit/haproxy-spoe-go/worker"
)

type Options struct {
	// MaxFrameSize is the local safety limit used only when reading the first
	// HAPROXY-HELLO frame (excluding the 4-byte length prefix). It must be
	// >= frame.MinFrameSize. After a valid HELLO, the connection uses the
	// max-frame-size announced by HAProxy for all subsequent frames.
	//
	// Size the local limit so a legitimate HAPROXY-HELLO fits (HELLOs are
	// small; the default is usually enough). Align HAProxy with:
	//
	//   spoe-agent <name>
	//       max-frame-size <value>
	//
	// A value of 0 selects the library default; there is no "unlimited" mode.
	MaxFrameSize uint32

	// MaxConnectionDuration configures graceful worker-side connection rotation.
	// When > 0, a connection is kept for at least this duration plus a random
	// jitter of up to 30% of the base value. When 0, connection lifetime is unlimited.
	MaxConnectionDuration time.Duration

	// Logger is used for error reporting. When nil, a no-op logger is used.
	Logger logger.Logger
}

func New(handler func(*request.Request), logger logger.Logger) *Agent {
	return &Agent{
		handler:      handler,
		logger:       logger,
		maxFrameSize: frame.DefaultMaxFrameSize,
	}
}

func NewWithOptions(handler func(*request.Request), opts Options) (*Agent, error) {
	if opts.MaxFrameSize == 0 {
		opts.MaxFrameSize = frame.DefaultMaxFrameSize
	}
	if err := frame.ValidateMaxFrameSize(opts.MaxFrameSize); err != nil {
		return nil, fmt.Errorf("agent options: %w", err)
	}
	if handler == nil {
		return nil, fmt.Errorf("agent options: handler must not be nil")
	}
	if opts.Logger == nil {
		return nil, fmt.Errorf("agent options: logger must not be nil")
	}

	return &Agent{
		handler:               handler,
		logger:                opts.Logger,
		maxFrameSize:          opts.MaxFrameSize,
		maxConnectionDuration: opts.MaxConnectionDuration,
	}, nil
}

type Agent struct {
	handler               func(*request.Request)
	logger                logger.Logger
	maxFrameSize          uint32
	maxConnectionDuration time.Duration
}

// MaxFrameSize returns the configured local maximum frame size.
func (agent *Agent) MaxFrameSize() uint32 {
	return agent.maxFrameSize
}

// MaxConnectionDuration returns the configured minimum connection lifetime.
func (agent *Agent) MaxConnectionDuration() time.Duration {
	return agent.maxConnectionDuration
}

func (agent *Agent) Serve(listener net.Listener) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				continue
			}
			return err
		}

		go worker.HandleWithOptions(conn, agent.handler, worker.Options{
			MaxFrameSize:          agent.maxFrameSize,
			MaxConnectionDuration: agent.maxConnectionDuration,
			Logger:                agent.logger,
		})
	}
}

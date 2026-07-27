package agent

import (
	"fmt"
	"net"

	"github.com/negasus/haproxy-spoe-go/frame"
	"github.com/negasus/haproxy-spoe-go/logger"
	"github.com/negasus/haproxy-spoe-go/request"
	"github.com/negasus/haproxy-spoe-go/worker"
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
}

func New(handler func(*request.Request), logger logger.Logger) *Agent {
	return &Agent{
		handler:      handler,
		logger:       logger,
		maxFrameSize: frame.DefaultMaxFrameSize,
	}
}

func NewWithOptions(handler func(*request.Request), logger logger.Logger, opts Options) (*Agent, error) {
	if opts.MaxFrameSize == 0 {
		opts.MaxFrameSize = frame.DefaultMaxFrameSize
	}
	if err := frame.ValidateMaxFrameSize(opts.MaxFrameSize); err != nil {
		return nil, fmt.Errorf("agent options: %w", err)
	}
	if handler == nil {
		return nil, fmt.Errorf("agent options: handler must not be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("agent options: logger must not be nil")
	}

	return &Agent{
		handler:      handler,
		logger:       logger,
		maxFrameSize: opts.MaxFrameSize,
	}, nil
}

type Agent struct {
	handler      func(*request.Request)
	logger       logger.Logger
	maxFrameSize uint32
}

// MaxFrameSize returns the configured local maximum frame size.
func (agent *Agent) MaxFrameSize() uint32 {
	return agent.maxFrameSize
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

		go worker.HandleWithMaxFrameSize(conn, agent.handler, agent.logger, agent.maxFrameSize)
	}
}

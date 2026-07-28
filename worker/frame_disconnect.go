package worker

import (
	"fmt"

	"github.com/negasus/haproxy-spoe-go/frame"
)

func (w *worker) processHaproxyDisconnect(f *frame.Frame) error {
	defer frame.ReleaseFrame(f)

	if !w.ready {
		return fmt.Errorf("worker not ready, but got HaproxyDisconnect frame")
	}

	if err := w.sendAgentDisconnect(0, 0, statusNormal, "connection closed by server"); err != nil {
		return fmt.Errorf("error send AgentDisconnect frame: %w", err)
	}

	return nil
}

func (w *worker) sendAgentDisconnect(streamID, frameID uint64, statusCode uint32, message string) error {
	agentDisconnectFrame := frame.AcquireFrame()
	defer frame.ReleaseFrame(agentDisconnectFrame)

	agentDisconnectFrame.Type = frame.TypeAgentDisconnect
	agentDisconnectFrame.FrameID = frameID
	agentDisconnectFrame.StreamID = streamID

	// Keep the disconnect message short so the disconnect frame itself stays
	// within the current max-frame-size (local limit before negotiation, or
	// negotiated afterwards)
	const maxMsgLen = 64
	if len(message) > maxMsgLen {
		message = message[:maxMsgLen]
	}

	agentDisconnectFrame.KV.Add("status-code", statusCode)
	agentDisconnectFrame.KV.Add("message", message)

	if err := w.writeFrame(agentDisconnectFrame); err != nil {
		return fmt.Errorf("error write AgentDisconnect: %w", err)
	}

	return nil
}

package main

import (
	"log"
	"math/rand"
	"net"
	"os"

	"github.com/negasus/haproxy-spoe-go/action"
	"github.com/negasus/haproxy-spoe-go/agent"
	"github.com/negasus/haproxy-spoe-go/logger"
	"github.com/negasus/haproxy-spoe-go/request"
)

// Example SPOA from HAProxy SPOE specification section 2.5 (IP reputation).
// Pair with the HAProxy / SPOE configs in this directory.
func main() {
	log.Print("listen 127.0.0.1:3000")

	listener, err := net.Listen("tcp4", "127.0.0.1:3000")
	if err != nil {
		log.Printf("error create listener: %v", err)
		os.Exit(1)
	}
	defer func() { _ = listener.Close() }()

	a := agent.New(handler, logger.NewDefaultLog())

	if err := a.Serve(listener); err != nil {
		log.Printf("error agent serve: %v", err)
		os.Exit(1)
	}
}

func handler(req *request.Request) {
	log.Printf(
		"handle request EngineID=%q StreamID=%d FrameID=%d messages=%d",
		req.EngineID, req.StreamID, req.FrameID, req.Messages.Len(),
	)

	mes, err := req.Messages.GetByName("get-ip-reputation")
	if err != nil {
		log.Printf("message get-ip-reputation not found: %v", err)
		return
	}

	ipValue, ok := mes.KV.Get("ip")
	if !ok {
		log.Printf("var 'ip' not found in message")
		return
	}

	ip, ok := ipValue.(net.IP)
	if !ok {
		log.Printf("var 'ip' has wrong type; expected net.IP")
		return
	}

	ipScore := rand.Intn(100)
	log.Printf("IP: %s, send score %d", ip.String(), ipScore)

	req.Actions.SetVar(action.ScopeSession, "ip_score", ipScore)
}

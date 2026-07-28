# Haproxy SPOE Golang Agent Library [![Go Report Card](https://goreportcard.com/badge/github.com/negasus/haproxy-spoe-go)](https://goreportcard.com/report/github.com/negasus/haproxy-spoe-go) ![](https://github.com/negasus/haproxy-spoe-go/workflows/Test/badge.svg)

Terms from [Haproxy SPOE specification](https://www.haproxy.org/download/1.9/doc/SPOE.txt)

```
* SPOE : Stream Processing Offload Engine.

    A SPOE is a filter talking to servers managed ba a SPOA to offload the
    stream processing. An engine is attached to a proxy. A proxy can have
    several engines. Each engine is linked to an agent and only one.

* SPOA : Stream Processing Offload Agent.

    A SPOA is a service that will receive info from a SPOE to offload the
    stream processing. An agent manages several servers. It uses a backend to
    reference all of them. By extension, these servers can also be called
    agents.

* SPOP : Stream Processing Offload Protocol, used by SPOEs to talk to SPOA
         servers.

    This protocol is used by engines to talk to agents. It is an in-house
    binary protocol described in this documentation.
```


This library implements SPOA for Golang applications

## Install

```
go get -u github.com/negasus/haproxy-spoe-go
```

## Example

Example from Section 2.5 [SPOE specification](https://www.haproxy.org/download/1.9/doc/SPOE.txt) describes a simple IP reputation service.

> Here is a simple but complete example that sends client-ip address to a ip
  reputation service. This service can set the variable "ip_score" which is an
  integer between 0 and 100, indicating its reputation (100 means totally safe
  and 0 a blacklisted IP with no doubt).

Golang backend application for this example

```go
package main

import (
	"github.com/negasus/haproxy-spoe-go/action"
	"github.com/negasus/haproxy-spoe-go/agent"
	"github.com/negasus/haproxy-spoe-go/request"
        "github.com/negasus/haproxy-spoe-go/logger"
	"log"
	"math/rand"
	"net"
	"os"
)

func main() {

	log.Print("listen 3000")

	listener, err := net.Listen("tcp4", "127.0.0.1:3000")
	if err != nil {
		log.Printf("error create listener, %v", err)
		os.Exit(1)
	}
	defer listener.Close()

	// MaxFrameSize is optional. agent.New(...) is enough for most setups and
	// uses the default (16380). Use NewWithOptions only when you need a custom
	// local limit for the first HAPROXY-HELLO. After HELLO, HAProxy's announced
	// max-frame-size is used for the connection:
	//
	//   spoe-agent myagent
	//       max-frame-size 16380
	//
	a, err := agent.NewWithOptions(handler, logger.NewDefaultLog(), agent.Options{
		MaxFrameSize: 16380, // optional; 0 or omitted via agent.New → default 16380; must be >= 256 if set
	})
	if err != nil {
		log.Printf("error create agent: %v", err)
		os.Exit(1)
	}

	if err := a.Serve(listener); err != nil {
		log.Printf("error agent serve: %+v\n", err)
	}
}

func handler(req *request.Request) {

	log.Printf("handle request EngineID: '%s', StreamID: '%d', FrameID: '%d' with %d messages\n", req.EngineID, req.StreamID, req.FrameID, req.Messages.Len())

	messageName := "get-ip-reputation"

	mes, err := req.Messages.GetByName(messageName)
	if err != nil {
		log.Printf("message %s not found: %v", messageName, err)
		return
	}

	ipValue, ok := mes.KV.Get("ip")
	if !ok {
		log.Printf("var 'ip' not found in message")
		return
	}

	ip, ok := ipValue.(net.IP)
	if !ok {
		log.Printf("var 'ip' has wrong type. expect IP addr")
		return
	}

	ipScore := rand.Intn(100)

	log.Printf("IP: %s, send score '%d'", ip.String(), ipScore)

	req.Actions.SetVar(action.ScopeSession, "ip_score", ipScore)
}
```

## Frame size limits (security)

SPOP frames are prefixed with an untrusted 4-byte length. This library always
applies a finite maximum before allocating or reading a payload:

1. **Local limit** — optional `Options.MaxFrameSize` (default `16380`, minimum
   `256`) bounds only the first `HAPROXY-HELLO`. Omit it / use `agent.New` for
   the default. There is no unlimited mode; `0` also selects the default.
   Size a custom value so a legitimate HELLO fits.
2. **HAProxy limit** — after a valid HELLO, `AGENT-HELLO` echoes HAProxy's
   `max-frame-size`, and that value is used for all later inbound and outbound
   frames on the connection. Invalid or missing peer values cause a controlled
   disconnect.

Pointing an **HTTP** Kubernetes/load-balancer probe at the binary SPOP port
produces errors such as `unexpected frame type 47` (`'G'` from `GET /`). That
is expected: use a **TCP** probe on the SPOP port, or expose a separate HTTP
port for readiness/liveness. Correct probe configuration is operational
hygiene; the library itself rejects oversized frames even when the frame type
is a valid SPOP type (for example a crafted `NOTIFY` with a huge length).

## API

### Agent options

`MaxFrameSize` is **optional**. Most applications can keep using `agent.New`:

```go
a := agent.New(handler, log) // uses default MaxFrameSize (16380)
```

Set it only when you need a custom local limit for the first `HAPROXY-HELLO`:

```go
a, err := agent.NewWithOptions(handler, log, agent.Options{
    MaxFrameSize: 16380, // optional; 0 selects the default; if set, must be >= 256
})
```

### Messages

Getting request data is possible through `Request.Messages`

#### Len() int

Returns count of messages in request

```
count := request.Messages.Len()
```

#### GetByName(name string) (*Message, error)

Returns message by name and error if not exists

```
mes, err := request.Messages.GetByName("get-ip-reputation")
```

#### GetByIndex(idx int) (*Message, error)

Returns message by index and error if not exists

```
mes, err := request.Messages.GetByIndex(0)
```

### Message

Represents one message, sent by Haproxy

Message has two fields
- Name string
- KV   *kv.KV

KV contains key-value data of message

### Actions

Actions is used for sending a response to Haproxy

#### SetVar(scope Scope, name string, value interface{})

Set variable with `name` to `value` in specific `scope` (see bellow)

```
request.Actions.SetVar(action.ScopeSession, "ip_score", 10)
```

#### UnsetVar(scope Scope, name string)

Unset variable with `name` in specific `scope`

```
request.Actions.UnsetVar(action.ScopeSession, "ip_score")
```

#### Actions Scopes
- ScopeProcess
- ScopeSession
- ScopeTransaction
- ScopeRequest
- ScopeResponse

### KV (key-value)

Contains message key-value data sent by Haproxy

#### Get(key string) (interface{}, bool)

Returns value by name. If key doesn't exist, last returned value will be set to False

```
ipValue, ok := message.KV.Get("ip")
```

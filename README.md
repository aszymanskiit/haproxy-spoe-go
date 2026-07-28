# haproxy-spoe-go

Go library implementing a **SPOA** (Stream Processing Offload Agent) for
[HAProxy SPOE](https://www.haproxy.org/download/2.8/doc/SPOE.txt).

[![CI](https://github.com/aszymanskiit/haproxy-spoe-go/actions/workflows/ci.yml/badge.svg)](https://github.com/aszymanskiit/haproxy-spoe-go/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/aszymanskiit/haproxy-spoe-go)](https://goreportcard.com/report/github.com/aszymanskiit/haproxy-spoe-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-%3E%3D%201.19-00ADD8?logo=go)](go.mod)

## Maintenance status / fork notice

This repository is a **maintained fork** of
[`negasus/haproxy-spoe-go`](https://github.com/negasus/haproxy-spoe-go).
It was created to continue maintenance, dependency updates, compatibility
improvements, bug fixes, and further development, as the upstream project is no
longer actively maintained.

| | URL |
| --- | --- |
| Maintained fork | https://github.com/aszymanskiit/haproxy-spoe-go |
| Upstream project | https://github.com/negasus/haproxy-spoe-go |

**Module path:** for import compatibility with existing applications, `go.mod`
still declares `github.com/negasus/haproxy-spoe-go`. Applications should keep
that import path and point the module to this fork with a `replace` directive
(see [Installation](#installation)).

This fork remains API-compatible with typical upstream usage (`agent.New`,
request handlers, actions, messages). It also includes hardening and
reliability work such as bounded SPOP frame reads, safer binary decoding,
connection-reset handling for HAProxy 3.x teardown behaviour, and waiting for
in-flight notify handlers on disconnect.

## Overview

HAProxy can offload stream processing to external agents using **SPOE** (Stream
Processing Offload Engine) over the binary **SPOP** protocol. A common pattern
is: HAProxy extracts request data (for example the client IP), sends it to an
agent, and applies ACLs based on variables the agent returns.

This library helps you implement that agent in Go:

1. Listen on a TCP address configured as an HAProxy SPOA backend.
2. Negotiate SPOP (`HAPROXY-HELLO` / `AGENT-HELLO`).
3. Receive `NOTIFY` frames with named messages and typed key/value arguments.
4. Run your handler and reply with an `AGENT-ACK` containing `set-var` /
   `unset-var` actions.

It is intended for Go developers building SPOA services (reputation, authz,
bot scoring, custom policy, and similar offloads).

### Terms (from the SPOE specification)

```text
* SPOE : Stream Processing Offload Engine.
         A filter talking to SPOA servers to offload stream processing.

* SPOA : Stream Processing Offload Agent.
         A service that receives info from a SPOE and returns actions/variables.

* SPOP : Stream Processing Offload Protocol (binary), used between SPOE and SPOA.
```

## Features

Based on the current code:

- SPOA server via `agent.Serve` (one goroutine per connection)
- SPOP handshake (`HAPROXY-HELLO` / `AGENT-HELLO`) with version `2.0`
- Advertised capabilities: `pipelining`, `async`
- Concurrent `NOTIFY` handling with serialized writes per connection
- Graceful drain of in-flight notify handlers before connection close
- Request handlers with decoded messages and typed KV arguments
- Response actions: `SetVar` / `UnsetVar` across HAProxy variable scopes
- Typed data encode/decode (null, bool, integers, IPv4/IPv6, string, binary)
- Pluggable `logger.Logger` (default, custom, nop, channel)
- Optional local max-frame-size for the first HELLO; peer limit afterwards
- Rejection of oversized / malformed frames and hardened parsers
- SPOP healthcheck hello support
- Treats peer connection reset / pipe errors as normal close (HAProxy 3.x)
- Test-oriented `client` package and fuzz tests for parsers
- No TLS termination inside the library (place TLS at the network edge if needed)

## Requirements

| Requirement | Notes |
| --- | --- |
| Go | **1.19+** (`go` directive in [`go.mod`](go.mod)). CI tests 1.19, oldstable, and stable. |
| HAProxy | Any HAProxy build that supports SPOE/SPOP as documented in the SPOE spec. This repository does **not** run HAProxy in CI; validate against your HAProxy version in staging. |
| OS | Portable Go networking (Linux/macOS/Windows). Developed and CI-tested on Linux. |
| Docker | Not required; no Docker assets are shipped. |

## Installation

Add the module using the upstream import path, then replace it with this fork:

```bash
go get github.com/negasus/haproxy-spoe-go@latest
```

In your application's `go.mod`:

```go
require github.com/negasus/haproxy-spoe-go v1.0.7 // or another known version/pseudo-version

replace github.com/negasus/haproxy-spoe-go => github.com/aszymanskiit/haproxy-spoe-go <version-or-commit>
```

Example pin to a branch tip:

```bash
go get github.com/aszymanskiit/haproxy-spoe-go@master
```

then set the `replace` line to the resolved pseudo-version printed by `go get`.

Imports in application code stay unchanged:

```go
import (
    "github.com/negasus/haproxy-spoe-go/agent"
    "github.com/negasus/haproxy-spoe-go/request"
)
```

## Quick start

```go
package main

import (
	"log"
	"net"
	"os"

	"github.com/negasus/haproxy-spoe-go/action"
	"github.com/negasus/haproxy-spoe-go/agent"
	"github.com/negasus/haproxy-spoe-go/logger"
	"github.com/negasus/haproxy-spoe-go/request"
)

func main() {
	listener, err := net.Listen("tcp4", "127.0.0.1:3000")
	if err != nil {
		log.Printf("listen: %v", err)
		os.Exit(1)
	}
	defer func() { _ = listener.Close() }()

	a := agent.New(handler, logger.NewDefaultLog())
	if err := a.Serve(listener); err != nil {
		log.Printf("serve: %v", err)
		os.Exit(1)
	}
}

func handler(req *request.Request) {
	mes, err := req.Messages.GetByName("get-ip-reputation")
	if err != nil {
		return
	}
	if ip, ok := mes.KV.Get("ip"); ok {
		_ = ip // use the value (often net.IP)
	}
	req.Actions.SetVar(action.ScopeSession, "ip_score", 100)
}
```

A complete runnable example (with sample HAProxy/SPOE configs) lives in
[`examples/ip-reputation`](examples/ip-reputation):

```bash
go run ./examples/ip-reputation
```

Custom local frame limit (optional):

```go
a, err := agent.NewWithOptions(handler, logger.NewDefaultLog(), agent.Options{
	MaxFrameSize: 16380, // 0 selects default 16380; must be >= 256 if set
})
```

## HAProxy configuration

See **[docs/haproxy.md](docs/haproxy.md)** for a full minimal frontend/backend +
SPOE file example, variable naming (`var-prefix`), validation commands, and
probe guidance.

## Examples

| Path | Description |
| --- | --- |
| [`examples/ip-reputation`](examples/ip-reputation) | IP reputation SPOA + sample `haproxy.cfg` / SPOE config |

## Documentation

| Doc | Contents |
| --- | --- |
| [docs/haproxy.md](docs/haproxy.md) | HAProxy / SPOE configuration |
| [docs/api.md](docs/api.md) | API reference (agent, messages, actions, logger, worker) |
| [docs/frame-size-limits.md](docs/frame-size-limits.md) | Frame size limits and related operational notes |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Contributor workflow |
| [SECURITY.md](SECURITY.md) | Vulnerability reporting |

## Development

```bash
git clone https://github.com/aszymanskiit/haproxy-spoe-go.git
cd haproxy-spoe-go
go mod download
go test ./...
go vet ./...
```

Useful Make targets:

```bash
make test      # race + coverage profile (coverage.txt)
make cover     # HTML coverage report
make build     # go build ./...
make examples  # build example programs
make lint      # golangci-lint (optional local install)
make fmt       # gofmt
```

## Testing

Unit and protocol tests run without HAProxy or Docker:

```bash
go test ./...
go test -race ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

Fuzz targets exist under `frame`, `message`, `typeddata`, `payload/kv`, and
`varint` (Go fuzzing). There is no automated end-to-end HAProxy integration
suite in this repository.

## Compatibility

| Area | Status |
| --- | --- |
| Go | Declared `go 1.19`; verified in development/CI against 1.19 through current stable toolchains |
| SPOP | Agent hello advertises version `2.0` |
| HAProxy | Compatible with SPOE as specified; not pinned to a single HAProxy release in CI |
| Upstream API | Import path preserved; see fork notice for behavioural/security differences |

## Security

Please report vulnerabilities privately — see [SECURITY.md](SECURITY.md).
Do not open public issues for security reports.

Frame-length handling is documented in
[docs/frame-size-limits.md](docs/frame-size-limits.md).

## License and attribution

This project is licensed under the [MIT License](LICENSE).

Copyright (c) 2019 Andrew Privalov and contributors.

This maintained fork is based on the original work in
[`negasus/haproxy-spoe-go`](https://github.com/negasus/haproxy-spoe-go).
The original copyright notice and permission notice are included in
[LICENSE](LICENSE) as required by the MIT license.

## Acknowledgements

This project builds upon the original work by the maintainers and contributors
of [`negasus/haproxy-spoe-go`](https://github.com/negasus/haproxy-spoe-go).
Their work provided the foundation for this maintained fork.

Thanks also to everyone who has contributed fixes and improvements in upstream
history and in this fork.

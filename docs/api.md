# API reference

Public packages used by typical SPOA applications:

| Package | Purpose |
| --- | --- |
| [`agent`](../agent) | Accept connections and dispatch notify handlers |
| [`request`](../request) | Per-notify request context (messages + actions) |
| [`message`](../message) | Decoded SPOE messages |
| [`action`](../action) | `set-var` / `unset-var` responses |
| [`payload/kv`](../payload/kv) | Key/value accessors for message arguments |
| [`logger`](../logger) | Pluggable error logger |
| [`worker`](../worker) | Lower-level SPOP connection handler |
| [`client`](../client) | Minimal SPOP client intended for tests |
| [`frame`](../frame), [`typeddata`](../typeddata), [`varint`](../varint) | Protocol encoding helpers |

Module import path (unchanged from upstream for compatibility):

```text
github.com/negasus/haproxy-spoe-go/...
```

See [Installation](../README.md#installation) for consuming this fork via
`replace`.

## Agent

### `agent.New`

```go
a := agent.New(handler, log)
```

Creates an agent using the default local max-frame-size (`16380`) for the first
`HAPROXY-HELLO` only.

### `agent.NewWithOptions`

```go
a, err := agent.NewWithOptions(handler, log, agent.Options{
    MaxFrameSize: 16380, // optional; 0 selects the default; if set, must be >= 256
})
```

`MaxFrameSize` is the **local** safety limit used only when reading the first
`HAPROXY-HELLO`. After a valid HELLO, HAProxy's announced `max-frame-size` is
used for the rest of the connection. There is no unlimited mode.

`NewWithOptions` returns an error if `MaxFrameSize` is invalid (when non-zero
and `< 256`), or if `handler` / `logger` is nil.

### `(*Agent).Serve`

```go
err := a.Serve(listener)
```

Accepts TCP connections on `listener` and handles each connection in a new
goroutine via the worker. Returns when `Accept` fails with a non-temporary
error (for example after the listener is closed).

### `(*Agent).MaxFrameSize`

Returns the configured local maximum frame size.

## Handler and request

Handlers have the signature:

```go
func(req *request.Request)
```

`request.Request` fields:

| Field | Description |
| --- | --- |
| `EngineID` | Engine id from the SPOE HELLO negotiation |
| `StreamID` | SPOP stream id |
| `FrameID` | SPOP frame id |
| `Messages` | Decoded messages from the `NOTIFY` frame |
| `Actions` | Actions to encode into the `AGENT-ACK` |

## Messages

```go
count := req.Messages.Len()
mes, err := req.Messages.GetByName("get-ip-reputation")
mes, err := req.Messages.GetByIndex(0)
```

`GetByName` / `GetByIndex` return `message.ErrMessageNotFound` when missing.

Each `message.Message` has:

- `Name string`
- `KV *kv.KV`

## KV (key-value)

```go
ipValue, ok := mes.KV.Get("ip")
```

`Get` returns `(value, false)` when the key is absent.

Decoded typed-data values commonly appear as Go types such as `bool`, integer
types, `string`, `[]byte`, and `net.IP` (IPv4/IPv6).

## Actions

```go
req.Actions.SetVar(action.ScopeSession, "ip_score", 10)
req.Actions.UnsetVar(action.ScopeSession, "ip_score")
```

Scopes:

- `action.ScopeProcess`
- `action.ScopeSession`
- `action.ScopeTransaction`
- `action.ScopeRequest`
- `action.ScopeResponse`

HAProxy exposes set variables using the SPOE `var-prefix`. See
[HAProxy configuration](haproxy.md).

## Logging

`logger.Logger` is a small interface:

```go
type Logger interface {
    Errorf(format string, args ...interface{})
}
```

Built-in implementations:

- `logger.NewDefaultLog()` — standard library default logger
- `logger.NewLog(l *log.Logger)` — wrap a custom `log.Logger`
- `logger.NewNop()` — discard
- `logger.NewChannel(ch)` — send messages on a channel (can block if full)

## Worker package

Advanced users can handle a single connection without `agent.Serve`:

```go
worker.Handle(conn, handler, log)
worker.HandleWithMaxFrameSize(conn, handler, log, maxFrameSize)
```

Capabilities advertised to HAProxy: `pipelining,async`. Notify frames are
processed concurrently; outbound ACKs are serialized per connection. On
disconnect, the worker waits for in-flight notify handlers before closing.

## Client package

`client.Client` is a minimal SPOP peer for tests (`Init`, `Notify`, `Stop`).
It is not intended as a production HAProxy replacement.

# Frame size limits (security)

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

Related constants live in package `frame`:

- `frame.MinFrameSize` (`256`)
- `frame.DefaultMaxFrameSize` (`16380`)

HAProxy side alignment example:

```haproxy
spoe-agent myagent
    max-frame-size 16380
```

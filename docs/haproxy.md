# HAProxy configuration

This guide shows a minimal HAProxy + SPOE setup that matches the
[IP reputation example](../examples/ip-reputation) included in this repository.
It is adapted from section 2.5 of the
[HAProxy SPOE specification](https://www.haproxy.org/download/2.8/doc/SPOE.txt).

## Layout

| File | Role |
| --- | --- |
| HAProxy main config | Frontends/backends, SPOE filter line, agent backend |
| SPOE config file | `spoe-agent` / `spoe-message` definitions |
| Go SPOA | This library listening on the agent backend address |

The SPOE configuration **must** live in a dedicated file referenced by
`filter spoe ... config <path>`. The TCP backend used by the agent is declared
in the main HAProxy configuration.

## Example: main HAProxy config

```haproxy
frontend www
    mode http
    bind *:80

    filter spoe engine ip-reputation config /etc/haproxy/spoe-ip-reputation.conf

    # Variable name is <scope>.<var-prefix>.<name-set-by-agent>
    # Here: sess.iprep.ip_score  (ScopeSession + var-prefix iprep + ip_score)
    tcp-request content reject if { var(sess.iprep.ip_score) -m int lt 20 }

    default_backend http-servers

backend http-servers
    mode http
    server http 127.0.0.1:8080

backend iprep-servers
    mode tcp
    balance roundrobin

    # connect timeout should be greater than SPOE hello timeout
    timeout connect 5s
    # server timeout should be greater than SPOE idle timeout
    timeout server  3m

    server iprep1 127.0.0.1:3000
```

## Example: SPOE config (`spoe-ip-reputation.conf`)

```haproxy
[ip-reputation]

spoe-agent iprep-agent
    messages get-ip-reputation
    option var-prefix iprep
    timeout hello      2s
    timeout idle       2m
    timeout processing 10ms
    use-backend iprep-servers

spoe-message get-ip-reputation
    args ip=src
    event on-client-session
```

Notes:

- The engine name on `filter spoe engine ip-reputation` must match the
  `[ip-reputation]` scope in the SPOE file.
- `args ip=src` sends the client source address as typed data named `ip`.
  The Go handler reads it with `mes.KV.Get("ip")` and type-asserts to `net.IP`.
- `option var-prefix iprep` prefixes variables set by the agent. A Go call to
  `req.Actions.SetVar(action.ScopeSession, "ip_score", score)` is visible in
  HAProxy as `var(sess.iprep.ip_score)`.
- Align `max-frame-size` in the SPOE agent section with your deployment if you
  customize frame sizes. See [Frame size limits](frame-size-limits.md).

## How the request flows

1. A client connects to the HAProxy frontend.
2. The SPOE filter sends a `NOTIFY` frame containing message
   `get-ip-reputation` with argument `ip`.
3. The Go agent runs your handler, which may set/unset HAProxy variables via
   `req.Actions`.
4. The library replies with an `AGENT-ACK` carrying those actions.
5. HAProxy applies ACLs / rules that read the resulting variables.

## Validating configuration

```bash
haproxy -c -f /etc/haproxy/haproxy.cfg
```

Start the Go agent first (listening on the agent backend address), then reload
or start HAProxy.

## Health checks and probes

SPOP is a binary protocol. An HTTP readiness probe against the SPOA port is not
valid and may produce errors such as `unexpected frame type 47` (`'G'` from
`GET /`). Use a TCP probe on the SPOA port, or expose a separate HTTP endpoint
in your application for Kubernetes/load-balancer health checks.

The library also supports SPOP healthcheck hellos: when HAProxy sends
`HAPROXY-HELLO` with the healthcheck flag, the agent answers `AGENT-HELLO` and
closes the connection without processing notify traffic.

## Related files

- Example agent: [`examples/ip-reputation`](../examples/ip-reputation)
- Frame size / security notes: [frame-size-limits.md](frame-size-limits.md)
- API reference: [api.md](api.md)

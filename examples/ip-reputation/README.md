# Example: IP reputation SPOA

Minimal agent implementing the IP reputation scenario from the HAProxy SPOE
specification (section 2.5).

## Run the agent

From the repository root:

```bash
go run ./examples/ip-reputation
```

The process listens on `127.0.0.1:3000`.

## HAProxy

Sample configs:

- [`haproxy.cfg`](haproxy.cfg) — frontend, SPOE filter, backends
- [`spoe-ip-reputation.conf`](spoe-ip-reputation.conf) — SPOE agent/message

Adjust paths and server addresses for your environment, then validate:

```bash
haproxy -c -f examples/ip-reputation/haproxy.cfg
```

See also [docs/haproxy.md](../../docs/haproxy.md).

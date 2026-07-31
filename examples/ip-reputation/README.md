# Example: IP reputation SPOA

Minimal agent implementing the IP reputation scenario from the HAProxy SPOE
specification (section 2.5).

In this setup, HAProxy calls the SPOE agent and enriches forwarded requests
with the `x-ip-rep` header.

## Run with Docker Compose

From `examples/ip-reputation`:

```bash
docker compose up --build
```

This starts:
- `haproxy` on `localhost:8080`
- `echo-server` (OpenResty) behind HAProxy
- `iprep-agent` (SPOA)

## Test with curl

```bash
curl -i http://localhost:8080/some/path
```

Example echoed request body from backend:

```http
GET /some/path HTTP/1.1
host: localhost:8080
user-agent: curl/8.7.1
accept: */*
x-ip-rep: 75



bfd0b3317317
```

HAProxy adds `x-ip-rep` in `frontend www` using:

`http-request set-header x-ip-rep %[var(txn.iprep.ip_score)]`

The score is produced by the SPOE agent (`get-ip-reputation`) and may vary
between requests.

See also [docs/haproxy.md](../../docs/haproxy.md).

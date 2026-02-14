# CascadeGuard

Static analyzer for microservice retry/timeout topologies. Catches cascading failure anti-patterns **before** they cause a 3 AM pager storm.

## Detected Anti-Patterns

| Rule | Severity | Description |
|------|----------|-------------|
| `timeout-inversion` | error | Downstream timeout > upstream timeout |
| `retry-amplification` | error | Multiplicative retry factor >10x along a path |
| `retry-without-cb` | warning | Retries configured without circuit breaker |
| `non-idempotent-retry` | error | Retrying POST/PATCH/DELETE requests |
| `backoff-no-jitter` | warning | Retries without jitter (thundering herd) |

## Install

```bash
go install github.com/cascadeguard/cascadeguard@latest
```

Or build from source:

```bash
git clone https://github.com/cascadeguard/cascadeguard && cd cascadeguard
go build -o cascadeguard .
```

## Usage

Create `topology.yaml`:

```yaml
services:
  gateway:
    calls:
      - target: user-svc
        timeout: 3s
        retries: 3
        circuit_breaker: false
        method: GET
  user-svc:
    calls:
      - target: db-svc
        timeout: 5s
        retries: 2
        circuit_breaker: false
        method: POST
```

Run analysis:

```bash
cascadeguard topology.yaml
```

Exit code `0` = clean, `1` = findings detected, `2` = input error.

## CI Integration

```yaml
- run: go install github.com/cascadeguard/cascadeguard@latest
- run: cascadeguard topology.yaml
```

## License

MIT

# Klonch Development Notes

## Building

Always build with explicit output to avoid stale binary issues:

```bash
go build -o klonch ./cmd/klonch
```

Do NOT use `go build ./...` as it builds to cache without updating the local binary.

## Debug Logging

Enable debug logging with:

```bash
KLONCH_DEBUG=1 ./klonch
```

Logs are written to:
- `/tmp/klonch-debug.log` - ListView debug
- `/tmp/klonch-root-debug.log` - Root model debug

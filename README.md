# drand‑poc

A training service providing **time‑lock encryption** on top of the distributed randomness beacon [drand](https://drand.love/).

The idea is similar to a Pastebin with a time‑lock: you save a text snippet, specify the moment when it should become readable, and the service encrypts the snippet using the drand public key. Decryption is only possible **after** the chosen time, after which anyone who possesses the direct URL can read the note.

## Features

- Create a note with `unlock_at` (RFC‑3339 UTC).
- Encrypt/decrypt via the public drand network.
- URLs of the form  
  `https://<BASE_DOMAIN>/<id>/<hash>` — only the exact link grants access; there is no public index.
- Storage in **BadgerDB** with TTL =`unlock_at + 7 days`.
- Minimal frontend (vanilla JS + micro‑CSS).
- Single Docker image, runnable through Podman/docker.
- Unit **and** integration tests with total coverage **> 50 %**.
- GitHub Actions: build, test, push image to `ghcr.io`.

## Architecture

```text
/cmd/server         – runnable binary (HTTP API + static files)
/http               – net/http handlers
/storage            – Badger data access layer
/frontend           – index.html, js, css
/internal/crypt     – separate Go module wrapping drand
  /drand            – drand client (gRPC/HTTP)
  /crypto           – encryption/decryption
```

> **All cryptography‑related code lives in `internal/crypt`
> (with its own `go.mod`) so it can be reused in other projects.**

## Quick start

```bash
git clone https://github.com/korjavin/drand-poc
cd drand-poc
podman build -t ghcr.io/<user>/drand-poc:latest .
podman run  -p 8080:8080 \
  -e BASE_DOMAIN=https://example.com \
  ghcr.io/<user>/drand-poc:latest
```

Open `http://localhost:8080`, create a note, obtain the URL and verify that it stays inaccessible until `unlock_at`.

## Configuration (environment variables)

| Variable      | Default                | Description                       |
|---------------|------------------------|-----------------------------------|
| `BASE_DOMAIN` | `http://localhost`     | Base domain for generated URLs    |
| `BADGER_DIR`  | `./data`               | Path to Badger directory          |
| `ADDR`        | `:8080`                | HTTP server bind address          |
| `DRAND_CHAIN` | `https://api.drand.sh` | Public drand HTTP endpoint        |
| `LOG_LEVEL`   | `info`                 | `debug`, `info`, `warn`, `error`  |

## Testing

```bash
go test ./... -cover
# overall coverage should be > 50 %
```

Unit tests cover:

- correct encryption / decryption;
- storage layer (in‑memory backend);
- input validation.

Integration tests start an in‑memory Badger instance, launch the HTTP server on a random port, and exercise the full create‑read flow.

## CI / CD

`.github/workflows/ci.yml` runs:

1. `go vet`, `golangci-lint run`, `go test -cover`.
2. Multi‑arch Docker build.
3. Login to `ghcr.io` and push `latest` + commit‑SHA tags.

## License

MIT

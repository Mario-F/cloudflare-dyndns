# AGENTS.md

Go CLI that updates **existing** Cloudflare DNS records with the current external IP.
Port of `github.com/Mario-F/hetzner-dyndns`; keep structure and behaviour close to it, only the DNS provider differs. No over-engineering.

## Layout

- `main.go` → `cmd.Execute()`
- `cmd/` – Cobra commands: `root.go` (global flags, config, token lookup), `myip.go`, `records.go`, `update.go` (+ `update_test.go`), `version.go`
- `internal/cloudflare/` – API client, `net/http` only (+ `httptest` based tests)
- `internal/externalip`, `internal/providers`, `internal/network`, `internal/logger` – taken over from the template; keep logic, provider list and regexes unchanged
- Tooling: `Makefile`, `Dockerfile`, `.goreleaser.yml`, `.golangci.yml`, `.github/workflows/{test,release,docker}.yml`, `install.sh`

## Commands

```shell
make test     # go test -race with coverage, must stay green
make lint     # golangci-lint (v2 config)
make build    # bin/cloudflare-dyndns
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run ./...   # if the installed linter is too old
docker run --rm -v "$PWD":/w -w /w goreleaser/goreleaser:latest check            # validate .goreleaser.yml
```

## Toolchain pitfalls

- `go.mod` requires a recent Go. If an older local Go auto-downloads the toolchain (`GOTOOLCHAIN=auto`), `make test` fails with `go: no such tool "covdata"` – that toolchain module lacks it. Use a full SDK (`go install golang.org/dl/goX.Y.Z@latest && goX.Y.Z download`, put `~/sdk/goX.Y.Z/bin` first in `PATH`, `GOTOOLCHAIN=local`).
- A golangci-lint built with an older Go fails with `unsupported version` / `missing type in composite literal`; run the current v2 via `go run` as above.
- `errcheck` excludes `(io.ReadCloser).Close` in `.golangci.yml` so the template's provider code stays unchanged; new code uses `_ =` explicitly.

## Behaviour rules

- Records are only **updated**, never created or deleted. Missing or ambiguous record → clear error, no write.
- Update via `PATCH /zones/{zone}/dns_records/{id}` with **only** `{"content": "<ip>"}`. Never PUT (resets `proxied`, `ttl`, `comment`). Never touch `proxied`.
- Token check: `GET /zones?per_page=1` and `success == true`. Do not use `/user/tokens/verify` (fails for account tokens).
- Zones and records are paginated via `result_info.total_pages`; zones are cached per client run.
- Record reference: FQDN (zone = longest matching zone name, type from `--version`: A/AAAA) or `<zone_id>/<record_id>` as printed by `records`.
- HTTP timeout 15 s; retry exactly once on 429/5xx (`Retry-After`, capped at 5 s). On `success: false` report `errors[].code/message`.
- `update`: external IP determined once, in parallel (WaitGroup) with resolving all `--record`s. Same IP (compared with `net.IP.Equal`) → log "No IP diff detected", no PATCH. One failing record does not stop the others; exit code is non-zero at the end.
- Errors go through `RunE` → one line on stderr, exit 1. No `panic` in normal error paths (cron friendly).
- Token lookup order: `--token`, `CLOUDFLARE_API_TOKEN`, `token` in the config file. No `viper.AutomaticEnv()`. The token must never be logged, also not with `--debug`.
- Do not reintroduce the template's interactive `setup`/`cronjob` command.

## Tests

- Cloudflare client tests use `httptest.Server` (set `Client.BaseURL`, `RetryDelay = 0`). Never call the real Cloudflare API from tests.
- `cmd/update_test.go` uses a fake `recordClient`; keep the "same IP → no PATCH" test.

## Privacy and secrets

- No real infrastructure in the repo: no own domains, hostnames, IPs, zone/record IDs or tokens – not in README, tests, examples or commit messages.
- Use reserved placeholders only: `example.com` / `example.net` / `example.org`, IPv4 `192.0.2.0/24`, `198.51.100.0/24`, `203.0.113.0/24`, IPv6 `2001:db8::/32`, token `your-api-token`.
- Before committing, grep the diff for anything that looks like real data. Once pushed, removing it needs a history rewrite plus GitHub-side cleanup (dangling commits stay reachable by SHA).

## Workflow

- Commit messages follow Conventional Commits (`feat:`, `fix:`, `docs:`, …).
- Commit, push, tags and history rewrites only with explicit approval of the maintainer.
- Releases: tag `v*` → goreleaser (linux/windows/darwin; 386/amd64/arm/arm64, goarm 6/7); the same tag push → multi-arch image (`latest` + semver tags) `ghcr.io/mario-f/cloudflare-dyndns`.

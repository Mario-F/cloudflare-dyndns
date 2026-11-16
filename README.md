# Cloudflare DynDNS

[![Conventional Commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-yellow.svg)](https://conventionalcommits.org)
![GitHub issues](https://img.shields.io/github/issues/Mario-F/cloudflare-dyndns)
![GitHub all releases](https://img.shields.io/github/downloads/Mario-F/cloudflare-dyndns/total)

A tool for updating existing Cloudflare DNS records with your external IP.

Records are only updated, never created or deleted. `proxied`, `ttl` and `comment` stay untouched.

## IP Scraping

This program uses a random list of 'what-is-my-ip' providers to cut down on errors; an IP is accepted once two providers confirm it.

## Install

Install binaries for your system from the release page or use the install script:

```shell
# Local (current directory)
curl -s https://raw.githubusercontent.com/Mario-F/cloudflare-dyndns/main/install.sh | BINDIR=./ bash

# Global
curl -s https://raw.githubusercontent.com/Mario-F/cloudflare-dyndns/main/install.sh | sudo BINDIR=/usr/local/bin bash
```

## API Token

Create an API token in the Cloudflare dashboard (My Profile -> API Tokens, or an account token) with these permissions:

- **Zone : Zone : Read**
- **Zone : DNS : Edit**

Limit the zone resources to the zones you want to update.

The token is read from (first match wins):

1. `--token`
2. the environment variable `CLOUDFLARE_API_TOKEN` (preferred: keeps the token out of the process list and crontabs)
3. `token:` in the config file (`$HOME/.cloudflare-dyndns.yaml` or `--config`)

## Usage

1. Test external IP with: `cloudflare-dyndns myip`
2. List your records: `CLOUDFLARE_API_TOKEN='your-api-token' cloudflare-dyndns records`
3. Update one or more records:

```shell
export CLOUDFLARE_API_TOKEN='your-api-token'
cloudflare-dyndns update --record host.example.com --record host.example.net
```

A record is given as FQDN (the zone is found by the longest matching zone name) or as `<zone_id>/<record_id>` as printed by `records`. The external IP is determined once and every record is checked and updated on its own; if one fails, the others are still processed and the exit code is non-zero.

Example config file:

```yaml
token: your-api-token
record:
  - host.example.com
  - host.example.net
```

### Cron

```shell
*/5 * * * * CLOUDFLARE_API_TOKEN=your-api-token cloudflare-dyndns --quiet update --record host.example.com --record host.example.net
```

With `--quiet` only errors are logged, so cron mails you only when something fails.

### Docker

```shell
docker run --rm -e CLOUDFLARE_API_TOKEN=your-api-token ghcr.io/mario-f/cloudflare-dyndns:latest update --record host.example.com
```

### IPv6 (Experimental)

IPv6 support can be activated with `--version ipv6`; all commands then work with AAAA records.

## Development / Debugging

`make test` runs the tests, `make build` builds `bin/cloudflare-dyndns`.

There are several ways to start development with live debugging provided by [delve](https://github.com/go-delve/delve).

### VSCode integrated Console

The provided launch.json has a debug task `Launch File` predefined, just hit start and it should run with the args provided in launch.json.

### VSCode external Terminal

A more advanced way to test in an external Terminal is provided by the `External Debugging` launch config and `./debug` script:

1. Execute debug script with optional arguments: `CLOUDFLARE_API_TOKEN=your-api-token ./debug records`
2. Start the `External Debugging` session in vscode

The order is important because vscode does not automatically retry to connect.

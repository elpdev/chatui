# pando

<picture>
  <source srcset="docs/images/pando.webp" type="image/webp">
  <img src="docs/images/pando.webp" alt="Pando banner" width="100%">
</picture>

First vertical slice for a terminal-native chat client and relay in Go.

Current scope:

- WebSocket relay with durable mailbox delivery
- Bubble Tea shell app with a dedicated chat route model
- Invite-based contact exchange and verified device bundles
- Encrypted 1:1 message envelopes between mailbox IDs
- Encrypted local chat history per peer conversation
- Relay size, TTL, and rate-limit controls
- Client reconnect with backoff after relay disconnects
- Contact fingerprint display and explicit verification flow
- Optional relay auth tokens for private deployments
- Trusted device enrollment, listing, and revocation
- Automatic contact device-bundle refresh during normal messaging
- Duplicate suppression for replayed relay envelopes
- Delivery acknowledgements reflected in local chat history
- Shareable invite codes for simpler secure onboarding

## Project Shape

- `cmd/pando`: thin client entrypoint
- `cmd/pando-relay`: thin relay entrypoint
- `cmd/pandoctl`: local identity and contact management
- `internal/clientcmd`: client startup and flag wiring
- `internal/ctlcmd`: control command wiring
- `internal/relaycmd`: relay startup and flag wiring
- `internal/config`: shared runtime config types and validation
- `internal/identity`: account and device key material
- `internal/logging`: shared logger setup
- `internal/messaging`: client-side message preparation and decode logic
- `internal/protocol`: relay/client wire types
- `internal/session`: encrypted message envelope helpers
- `internal/store`: local identity and contact persistence
- `internal/transport`: transport interface boundary
- `internal/transport/ws`: WebSocket transport implementation
- `internal/ui`: shell-level Bubble Tea app
- `internal/ui/chat`: chat route model
- `internal/relay`: relay server and mailbox behavior

## Run

Start the relay:

```bash
go run ./cmd/pando-relay
```

Relay runtime settings can also come from environment variables, which is useful for container deployments:

```bash
export PANDO_RELAY_AUTH_TOKEN="your-shared-secret"
export PANDO_RELAY_ADDR=":8080"
go run ./cmd/pando-relay
```

Supported relay environment variables:

- `PANDO_RELAY_AUTH_TOKEN`
- `PANDO_RELAY_ADDR`
- `PANDO_RELAY_STORE_PATH`
- `PANDO_RELAY_QUEUE_TTL`
- `PANDO_RELAY_MAX_MESSAGE_BYTES`
- `PANDO_RELAY_RATE_LIMIT_PER_MINUTE`

CLI flags still work and take precedence over environment variables.

If an environment variable has an invalid value, the relay now fails startup with an explicit error instead of silently falling back to defaults.

## ONCE Packaging

The relay is packaged to fit ONCE's expected runtime shape.

Container behavior:

- serves a landing page at `/`
- serves HTTP on port `80`
- exposes a healthcheck endpoint at `/up`
- stores durable relay state in `/storage/relay.db`

Build the image:

```bash
docker build -t pando-relay .
```

The Docker build uses the repo's vendored Go dependencies, so it does not need to fetch modules during image build.

On GitHub, every push to `main` also publishes the relay image automatically to GHCR with these tags:

- `ghcr.io/elpdev/pando-relay:latest`
- `ghcr.io/elpdev/pando-relay:main`
- `ghcr.io/elpdev/pando-relay:vX.Y.Z`
- `ghcr.io/elpdev/pando-relay:sha-<commit>`

Every push to `main` also creates:

- a semver git tag such as `v0.1.0`, `v0.2.0`, `v1.0.0`
- a matching GitHub Release
- attached release binaries for Linux, macOS, and Windows

GitHub Release notes are generated automatically from the commits since the previous version tag.

Version bumps follow conventional commits since the previous tag:

- `feat:` bumps the minor version
- `fix:`, `docs:`, `chore:`, `refactor:`, `test:` and other non-breaking commits bump the patch version
- `!` in the type line or `BREAKING CHANGE:` in the body bumps the major version

Doc-only and repository-metadata-only pushes are skipped and do not create a release.

The relay image is published as a multi-arch container for:

- `linux/amd64`
- `linux/arm64`

Run it locally like ONCE would:

```bash
docker run --rm -p 8080:80 -v "$PWD/storage:/storage" pando-relay
```

Test the healthcheck:

```bash
curl http://localhost:8080/up
```

The relay WebSocket endpoint will then be:

```text
ws://localhost:8080/ws
```

If you deploy it at `relay.lbp.dev`, your clients should use:

```text
wss://relay.lbp.dev/ws
```

If your ONCE deployment supports custom command arguments, you can still override defaults such as auth token or queue limits, for example:

```text
--auth-token <shared-token> --ttl 24h --max-message-bytes 65536 --rate-limit-per-minute 120
```

Optional hardening flags:

```bash
go run ./cmd/pando-relay --ttl 24h --max-message-bytes 65536 --rate-limit-per-minute 120 --auth-token secret-token
```

Or set the auth token through the environment so every connection is required to present it:

```bash
export PANDO_RELAY_AUTH_TOKEN="secret-token"
go run ./cmd/pando-relay
```

Initialize Alice and Bob locally and exchange invite codes:

```bash
go run ./cmd/pandoctl --help
go run ./cmd/pandoctl init --mailbox alice
go run ./cmd/pandoctl init --mailbox bob
go run ./cmd/pandoctl invite-code --mailbox alice --copy
go run ./cmd/pandoctl invite-code --mailbox bob --copy
go run ./cmd/pandoctl add-contact --mailbox alice --code '<bob-invite-code>'
go run ./cmd/pandoctl add-contact --mailbox bob --code '<alice-invite-code>'
go run ./cmd/pandoctl list-contacts --mailbox alice
go run ./cmd/pandoctl verify-contact --mailbox alice --contact bob --fingerprint <bob-fingerprint>
```

If you still want file-based exchange, `export-invite` and `import-contact` still work.

For full usage instructions — local and remote testing, contact management, device enrollment, and relay configuration — see the **[Wiki](https://github.com/elpdev/pando/wiki)**.

## Current Limitations

- No automatic history sync to newly enrolled devices
- Delivery state is message-level only; no per-device breakdown or read receipts yet

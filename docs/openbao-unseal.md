# OpenBao Auto-Unseal Guide

## Overview

OpenBao is the secret manager that backs cb-tumblebug's encrypted credential store. It is **initialized once** (an unseal key and a root token are generated and saved), and after that it comes up **sealed on every restart** — it must be unsealed before it can serve secrets.

`mayfly` provides two ways to handle this:

- **Explicit commands** (the stable baseline): `mayfly setup openbao init | unseal | status`.
- **Automatic unsealing** (the convenience layer for dev/test): the `openbao-unseal` sidecar polls OpenBao and re-unseals it whenever it is found sealed.

By default the sidecar is enabled, so in normal use you do not need to run any unseal command yourself. This guide documents what happens in each situation, how to diagnose and recover when something unexpected goes wrong, and the security trade-off you must understand before using this in anything other than a dev/test environment.

> **⚠ For development and testing only.** See the [Security](#security--development--testing-only) section. For production, switch to KMS auto-unseal or manual unseal.

## Commands

| Command | What it does |
|---------|--------------|
| `mayfly setup openbao status` | One-screen summary: OpenBao reachability / initialized / sealed, the `.env` `VAULT_TOKEN` (masked), whether cb-tumblebug and mc-terrarium picked up the token, and notes that suggest the matching fix. **Start here when something looks wrong.** |
| `mayfly setup openbao unseal` | Read the saved unseal key and unseal OpenBao. No-op if already unsealed. |
| `mayfly setup openbao init` | One-time initialization (writes `VAULT_TOKEN` into `.env`). `mayfly infra run` calls this automatically on a clean install, so you rarely run it by hand. Re-initializing requires `--force` and **destroys access to existing encrypted data**. |

## Behavior by situation (verified)

The table below describes what happens to OpenBao in each lifecycle event with the default (sidecar enabled) configuration.

| Situation | OpenBao after the event | Auto-recovery | Action needed |
|-----------|-------------------------|:-------------:|---------------|
| Clean install (`mayfly infra run`) | initialized and unsealed | yes — init + unseal run automatically | none |
| `mayfly infra stop` then `mayfly infra run` | recreated, sealed | yes — sidecar unseals | none |
| `docker compose down` / `up` | recreated, sealed | yes — sidecar unseals | none |
| OpenBao container restart | sealed | yes — sidecar re-unseals within the poll interval | none |
| **Host reboot / EC2 stop-start** | sealed | yes — sidecar re-unseals after the containers come back | none |
| OpenBao image tag update (container recreated) | sealed | yes — sidecar unseals | none |
| `openbao-init.json` missing or unreadable | sealed; cannot unseal | no — the sidecar keeps running and logs the reason (no crash) | restore the file (or re-run `setup openbao init` on a truly fresh setup); it auto-unseals once the file is back |
| Sidecar disabled (manual mode) | sealed after any restart | no | run `mayfly setup openbao unseal` |

In short: with the sidecar enabled, **every restart path — including a host reboot or EC2 stop-start — recovers on its own.** The only case that needs a human is a missing/disabled unseal key, and that is exactly what the commands above are for.

## Recovery scenarios

If credential operations, cb-tumblebug, or terrarium start behaving oddly, OpenBao being sealed is a common cause. Diagnose first, then apply the matching fix.

1. **Diagnose** — `mayfly setup openbao status`.
2. **`sealed=true`** → `mayfly setup openbao unseal`. (Normally the sidecar already did this; run it if the sidecar is disabled or you don't want to wait for the next poll.)
3. **`initialized=false` on an existing setup** → OpenBao lost its data or was never initialized. On a brand-new environment run `mayfly setup openbao init`. Do **not** run `init` against an environment that already has encrypted data unless you have wiped the volume — it would generate new keys and orphan the old data.
4. **`.env` has a token but a container shows it empty** → that container started before `.env` was populated. Recreate just that container (the `status` notes print the exact command).

## Polling interval

The sidecar's poll cadence is configured in a single place — `conf/docker/.env`:

```
OPENBAO_UNSEAL_POLL_INTERVAL=30
```

This is the maximum number of seconds OpenBao can stay sealed after a restart before the sidecar unseals it again. The check is a lightweight status request, so a short interval has negligible cost.

## Disabling auto-unseal (manual mode)

To run without automatic unsealing — for example while trialing KMS auto-unseal, or to keep full manual control — comment out the whole `openbao-unseal` service in `conf/docker/docker-compose.yaml`. After each restart, unseal by hand:

```
mayfly setup openbao unseal
```

All the commands above work the same whether the sidecar runs or not, so disabling it costs you only the convenience of automatic recovery.

## Security — development / testing only

The default auto-unseal is **file-based**: during initialization the unseal key and root token are written **in plaintext** to

```
conf/docker/data/openbao/secrets/openbao-init.json   (chmod 600)
```

and both the sidecar and the `setup openbao unseal` command read that file to unseal. This is convenient but it defeats the intent of OpenBao's Shamir key sharing, so it is for **development and testing only**.

What this does and does not expose:

- It **cannot be stolen over the network alone** — the key file is not served on any port, and the OpenBao API never returns the unseal key.
- It **is** exposed by host compromise, a backup/snapshot leak, or an accidental commit of the secrets file — anyone who can read that file gets full access.

This trade-off is a property of file-based auto-unseal itself; it is **the same whether the sidecar polls continuously or unseals once**.

For production, remove the on-disk plaintext key by using one of:

- **KMS auto-unseal** (recommended) — add a `seal "awskms"` (or GCP/Azure) stanza to OpenBao's config. At startup OpenBao asks the KMS to decrypt its seal key via IAM, so **there is no plaintext unseal key on disk**; even if the host is compromised there is no key file to read.
- **Manual unseal** — keep the unseal key off the host (operator enters it after each restart). Most secure, at the cost of availability and operational effort.
- **HSM** — strongest, with higher cost and complexity.

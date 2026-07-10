# OpenBao in cm-mayfly — Architecture, Lifecycle & Operations

OpenBao is the secret manager that backs cb-tumblebug's encrypted credential store (CSP credentials, namespaces, etc.). This document is the single source of truth for **how `mayfly` handles OpenBao end to end**: how it is initialized and unsealed, how the root token and unseal key flow through the system, how the token is injected into the dependent frameworks, how the state-consistency preflight diagnoses every situation, and how to recover when something looks wrong.

> **⚠ The default auto-unseal is for development and testing only.** It keeps the unseal key in plaintext on disk. See [Security](#security--development--testing-only) before using it anywhere else.

- [1. Overview & mental model](#1-overview--mental-model)
- [2. Architecture — the dependency chain](#2-architecture--the-dependency-chain)
- [3. Token & key handling](#3-token--key-handling)
- [4. How each stage handles OpenBao](#4-how-each-stage-handles-openbao)
- [5. State-consistency preflight](#5-state-consistency-preflight)
- [6. Commands](#6-commands)
- [7. Behavior by situation (verified)](#7-behavior-by-situation-verified)
- [8. Recovery & troubleshooting](#8-recovery--troubleshooting)
- [9. FAQ](#9-faq)
- [10. Relationship with cb-tumblebug's openbao-init](#10-relationship-with-cb-tumblebugs-openbao-init)
- [11. Auto-unseal sidecar & polling](#11-auto-unseal-sidecar--polling)
- [12. Security — development / testing only](#12-security--development--testing-only)

---

## 1. Overview & mental model

OpenBao is **initialized once** — an unseal key and a root token are generated and saved — and after that it comes up **sealed on every restart**. A sealed OpenBao answers most API calls with `503` and cannot serve secrets until it is **unsealed**. Once unsealed it also needs a brief moment to become **active** (mount table loaded, leadership settled) before it truly serves requests.

Three facts drive everything below:

1. **Two secrets, two jobs.** The **unseal key** turns a sealed OpenBao into an unsealed one. The **root token** authenticates API calls. Both are produced at init time and saved in `openbao-init.json`.
2. **Sealed ≠ down, and unsealed ≠ active.** `sealed:false` from `seal-status` does not yet mean "ready to serve" — the correct readiness signal is `GET /v1/sys/health` returning `200` (initialized + unsealed + active).
3. **The dependent frameworks need the token, not the key.** cb-tumblebug and mc-terrarium receive the **root token** via the `VAULT_TOKEN` environment variable; they never see the unseal key.

`mayfly` provides two layers:

- **Explicit commands** (stable baseline): `mayfly setup openbao init | unseal | status`.
- **Automatic handling** (dev/test convenience): `mayfly infra run` auto-initializes on a clean install, and the `openbao-unseal` sidecar re-unseals OpenBao whenever it is found sealed. In normal use you run no unseal command yourself.

---

## 2. Architecture — the dependency chain

`mayfly infra run` mirrors the upstream cb-tumblebug `make up` **staged flow** so the dependent containers never freeze with an empty token:

```
1. docker compose up -d openbao        # openbao alone (depends_on flows dependent→dependency,
                                        #  so nothing else is pulled in)
2. wait for the OpenBao API             # seal-status responds
3. run cb-tumblebug's openbao-init.sh   # writes VAULT_TOKEN into the shared .env
4. docker compose up -d                 # everything else — now they see a populated VAULT_TOKEN
```

The compose dependency chain and its health semantics:

```
openbao                        healthcheck: `bao status` (rc ≤ 2)  → HEALTHY even while SEALED
  └─ openbao-unseal (sidecar)  healthcheck: seal-status "sealed:false" → HEALTHY only when UNSEALED
       ├─ mc-terrarium         depends_on openbao-unseal: service_healthy   + VAULT_TOKEN=${VAULT_TOKEN}
       └─ cb-tumblebug         depends_on openbao-unseal: service_healthy   + VAULT_TOKEN=${VAULT_TOKEN}
                               (also depends on cb-tumblebug-postgres, etcd, cb-spider, mc-terrarium)
             ├─ cm-beetle      depends_on cb-tumblebug: service_healthy
             └─ cm-ant, cm-cicada, …   require cb-tumblebug to be *initialized* (see §10)
```

Two health-semantic subtleties that matter:

- **`openbao` is reported healthy even when sealed** (`bao status` exits `2` when sealed, and the check accepts `rc ≤ 2`). That is deliberate: it lets the sidecar depend on `openbao: service_healthy` and then do the unsealing itself.
- **`openbao-unseal` is healthy only when OpenBao is actually unsealed** (`sealed:false`). This is why `mc-terrarium` and `cb-tumblebug` depend on the *sidecar*, not on `openbao` — depending on the sidecar guarantees they only start once OpenBao is unsealed.

---

## 3. Token & key handling

### 3.1 Where each secret lives

| Secret | Produced by | Stored at | Used by |
|--------|-------------|-----------|---------|
| **Unseal key** (`keys[]` / `keys_base64[]`) | `POST /v1/sys/init` at init time | `conf/docker/data/openbao/secrets/openbao-init.json` (chmod 600) | `mayfly setup openbao unseal` and the `openbao-unseal` sidecar, to unseal |
| **Root token** (`root_token`) | same init call | same `openbao-init.json` **and** copied into `conf/docker/.env` as `VAULT_TOKEN` | API authentication; injected into cb-tumblebug / mc-terrarium |

Secrets are always **masked** in any human-facing output (first 8 characters + `***`). The unseal key is never returned by the API and never printed.

### 3.2 Token flow (init → .env → containers)

```
POST /v1/sys/init
   → { keys[], keys_base64[], root_token }
        │
        ├─ saved to openbao-init.json         (unseal key + root token)
        └─ root_token written to .env as VAULT_TOKEN   (by openbao-init.sh)
                 │
                 └─ docker compose interpolates ${VAULT_TOKEN}
                       → cb-tumblebug   environment: VAULT_TOKEN=<root token>
                       → mc-terrarium   environment: VAULT_TOKEN=<root token>
```

The dependent containers read `VAULT_TOKEN` **from their environment at container-create time**. A container created *before* `.env` was populated will hold an empty token even though `.env` now has one — the fix is to recreate just that container (`mayfly setup openbao status` prints the exact command).

### 3.3 Key injection into the frameworks

Only the **root token** is injected, and only into the two frameworks that talk to OpenBao:

- **cb-tumblebug** — uses the token to read/write encrypted CSP credentials in the secret store.
- **mc-terrarium** — uses the token for its own secret access.

No other framework receives the token, and none of them ever receive the unseal key. If a framework shows an empty `VAULT_TOKEN` while `.env` has one, it is a container-recreate problem, not a key problem.

---

## 4. How each stage handles OpenBao

| Stage | What it does with OpenBao |
|-------|---------------------------|
| **`mayfly infra run`** (full stack) | Runs the [state-consistency preflight](#5-state-consistency-preflight) first. On a clean install (no token, no data) it auto-runs `init` (staged flow above). On a consistent existing setup it proceeds. On any inconsistency it prints the specific remediation and **stops before starting the rest**, so the stack never deadlocks half-up. |
| **`mayfly infra run -s <svc>`** (targeted) | **Does not** run the preflight — a targeted start should not auto-init/auto-start OpenBao. OpenBao readiness is still enforced by compose (`depends_on: openbao-unseal service_healthy`). Token *validity* is not re-checked on this path; use the full `mayfly infra run` for the guarded path. |
| **`mayfly setup openbao init`** | One-time init (staged: openbao alone → wait API → `openbao-init.sh`). Refuses without `--force` if `VAULT_TOKEN` is already set (re-init would orphan existing encrypted data). |
| **`mayfly setup openbao unseal`** | Reads the saved unseal key and unseals. No-op if already unsealed. |
| **`mayfly setup openbao status`** | Read-only diagnosis (never starts OpenBao). Prints the full signal set + verdict. |
| **`mayfly setup tumblebug-init`** | Requires cb-tumblebug running + the preflight verdict to be OK (openbao consistent) before registering credentials — otherwise `multi-init.sh` silently fails with "VAULT_TOKEN is not set". |
| **`mayfly infra info`** | Embeds a compact read-only OpenBao consistency summary (same verdict as `status`). |
| **`mayfly infra remove --clean-db`** | Removes containers/images/volumes and DB host data, **but preserves the OpenBao host data and the `.env` VAULT_TOKEN**. A subsequent `infra run` reuses the existing OpenBao (consistent path). |
| **`mayfly infra remove --clean-all`** | Everything `--clean-db` does **plus** the OpenBao host data, and it **clears `VAULT_TOKEN` from `.env`** so the next `infra run` performs a clean auto-init instead of failing on a stale token. |
| **Host reboot / container restart / SIGKILL** | OpenBao comes back sealed; the sidecar re-unseals it. No `mayfly` action is involved — recovery is compose + sidecar. |

---

## 5. State-consistency preflight

Before `infra run`, `tumblebug-init`, and `openbao status` act, a shared **preflight** in `internal/openbao` collects the state signals and returns one authoritative verdict, so all entry points share one judgement. It is **detection/diagnosis only — it never writes `.env` and never destroys data.** When it finds a mismatch it returns a masked, actionable remediation message.

### 5.1 Signals

| Signal | Meaning | Source |
|--------|---------|--------|
| **T** | `.env` has a non-empty `VAULT_TOKEN` | `conf/docker/.env` (no network) |
| **J** | `openbao-init.json` present with a usable unseal key | disk |
| **D** | OpenBao storage directory holds data | disk (unreadable → *unknown*, never assumed empty) |
| **A** | API reports `initialized` | `GET /v1/sys/seal-status` |
| **Sealed** | API reports `sealed` | `GET /v1/sys/seal-status` |
| **Active** | API reached **`health 200`** (initialized + unsealed + **active**) | `GET /v1/sys/health` |
| **V** | Token validity — **valid / invalid / unknown** | `GET /v1/auth/token/lookup-self` |

**Readiness gate (the key ordering).** After bringing OpenBao up and unsealing it, the preflight waits for **`health 200` (active)** *before* it checks the token. Right after an unseal, OpenBao spends a short window (seconds) still answering `503` while it loads its mount table / settles leadership — see the measured ~14 s window in [§7](#7-behavior-by-situation-verified). Gating on `health 200` absorbs that window as **infrastructure readiness**, so a transition-window `503` can never be mistaken for a bad token. Only after readiness is confirmed does the token check run.

**Token is tri-state.** Because readiness is guaranteed upstream, the token probe returns:

- **valid** — `200`.
- **invalid** — `401` / `403` (a genuine wrong/stale token).
- **unknown** — a residual transient (`5xx` / timeout / connection error) that persists after a short retry. Unknown is **non-blocking**: the stack proceeds with an informational note rather than being flagged as a wrong token.

### 5.2 Verdict cases

| Case | Meaning | `infra run` behavior |
|------|---------|----------------------|
| **C1 fresh** | no token, no data | auto-init, then start the rest |
| **C2 consistent** | token + data + initialized + unsealed + **active** + **valid** token | proceed |
| **C3 orphaned-token** | token present but storage wiped | stop; guidance to clear `VAULT_TOKEN` (or `--clean-all` then re-init) |
| **C4 stale-init.json** | `init.json` present but storage wiped | stop; guidance to clear token + remove stale `init.json` |
| **C5 lost-token** | storage + `init.json` intact, only `.env` token missing | stop; guidance to restore `root_token` into `.env` |
| **C6 corrupt** | data on disk but API says not-initialized | stop; check mount / re-init if truly unusable |
| **C7 wrong-token** | token present but authentication **confirmed** `401/403` | stop; guidance to restore the correct `root_token` |
| **C8 stuck-sealed** | initialized but stays sealed after an unseal attempt | stop; unseal key likely does not match the data |
| **not-ready** | unsealed but the API never reached `health 200` within the bound | stop; transient — wait a moment and retry |
| **unknown** *(disk path)* | OpenBao down and disk signals ambiguous | non-fatal; points to `setup openbao status` |
| **token-unknown** *(reachable)* | active, but token validity could not be confirmed (residual transient) | **proceed** with a note (not treated as wrong-token) |

> Only **C1** and **C2** (and the non-blocking token-unknown variant of C2) are "OK to proceed". Every other case carries a specific remediation and stops the caller before the stack can deadlock.

---

## 6. Commands

| Command | What it does |
|---------|--------------|
| `mayfly setup openbao status` | One-screen summary: API reachable / initialized / sealed / **active**, token validity (**valid / unknown / INVALID**), `.env` token (masked), `init.json` + data-volume presence, whether cb-tumblebug and mc-terrarium picked up the token, the overall consistency verdict, and notes that suggest the matching fix. **Start here when something looks wrong.** |
| `mayfly setup openbao unseal` | Read the saved unseal key and unseal OpenBao. No-op if already unsealed. |
| `mayfly setup openbao init` | One-time initialization (writes `VAULT_TOKEN` into `.env`). `mayfly infra run` calls this automatically on a clean install, so you rarely run it by hand. Re-initializing requires `--force` and **destroys access to existing encrypted data**. |
| `mayfly infra info` | Among other things, shows the compact OpenBao consistency block (read-only). |

---

## 7. Behavior by situation (verified)

With the default (sidecar enabled) configuration. The last column notes real-environment verification.

| Situation | OpenBao after the event | Auto-recovery | Action needed | Verified |
|-----------|-------------------------|:-------------:|---------------|----------|
| Clean install (`infra run`) | initialized + unsealed + active | yes — init + unseal run automatically | none | ✅ C1 fresh → `consistent`/`valid` |
| `infra run` on an existing setup | consistent | yes | none | ✅ C2 |
| `infra stop` / `docker compose down` then `infra run` | recreated, sealed → unsealed | yes — sidecar / staged run | none | ✅ proceeds through the transition |
| **OpenBao container restart** | sealed → unsealed | yes — sidecar re-unseals; **the API answers `503` for a bounded window** (measured ~14 s) | none | ✅ readiness gate waits through it; `infra run` proceeds (~13 s), **no false wrong-token** |
| **Host reboot / EC2 stop-start** | sealed → unsealed | yes — sidecar re-unseals after containers return | none | ✅ `health 200` ~3 s after boot; status `consistent/valid` |
| **Forced crash (`SIGKILL`)** | down (no auto-restart after a manual kill) | recover with `infra run` | `mayfly infra run` | ✅ status shows `unknown / not running` (correct), then recovers to `consistent` |
| Wrong / stale `.env` token | active but token `403` | no | restore the correct `root_token` | ✅ `INVALID (401/403)` / `wrong-token` — **not** masked as transient |
| `.env` token empty, data intact | active, token empty | no | restore `root_token` into `.env` | ✅ `lost-token` with restore advice |
| Remove `-s cb-tumblebug --clean-db` then reinstall | OpenBao preserved | yes | none | ✅ cb-tumblebug healthy in ~12 s; OpenBao stays `consistent` |
| `init.json` missing / unreadable | sealed; cannot unseal | no — sidecar logs the reason, no crash | restore the file | (unit-tested) |
| Sidecar disabled (manual mode) | sealed after any restart | no | `mayfly setup openbao unseal` | — |

In short: with the sidecar enabled, **every restart path — container restart, host reboot, EC2 stop-start — recovers on its own**, and the preflight waits out the post-unseal `503` window instead of misreading it. The cases that need a human are a missing/disabled unseal key (C8) or a genuinely wrong token (C7), and the commands above are exactly for those.

---

## 8. Recovery & troubleshooting

Always **diagnose first**: `mayfly setup openbao status`, then apply the matching fix.

| `status` shows | Cause | Fix |
|----------------|-------|-----|
| `sealed=true` | OpenBao came up sealed and the sidecar hasn't unsealed yet (or is disabled) | `mayfly setup openbao unseal` (or wait one poll interval) |
| `consistency: not-ready` | unsealed but the API is still becoming active | transient — wait a few seconds and re-run |
| `token validity: INVALID (401/403)` (`wrong-token`) | `.env` `VAULT_TOKEN` doesn't match the current OpenBao | restore the `root_token` from `openbao-init.json` into `.env`, then `infra run` |
| `token validity: unknown` | OpenBao returned a transient error; validity couldn't be confirmed | usually harmless — re-run `status`; the stack is not blocked on this |
| `consistency: lost-token` | data + `init.json` intact, `.env` token empty | copy `root_token` from `init.json` into `VAULT_TOKEN`, then `infra run` |
| `consistency: orphaned-token` / `stale-init.json` | token/`init.json` present but storage was wiped | clear `VAULT_TOKEN` (and remove stale `init.json`), then `infra run` — or `infra remove --clean-all` then re-init |
| `initialized=false` with data on disk (`corrupt`) | possible mount misconfig | check the bind mount; if the data is truly unusable, `infra remove --clean-all` then re-init |
| a container shows `VAULT_TOKEN (empty)` while `.env` has one | that container started before `.env` was populated | recreate just that container (the `status` note prints the command) |

---

## 9. FAQ

**Q. Do I ever need to run an unseal command by hand?**
No, not in normal use. `infra run` initializes on a clean install and the sidecar re-unseals after every restart. You only run `setup openbao unseal` if the sidecar is disabled.

**Q. `infra run` says "wrong token" but my token is fine — what happened?**
Earlier versions had this behavior: a `503` from the post-unseal transition window was misread as a bad token. The readiness gate now waits for `health 200` before checking the token, so a transient `503`/timeout no longer trips a false `wrong-token`. A `wrong-token` verdict now means a genuine `401/403`.

**Q. After `infra run`, `cm-ant` keeps restarting and `cm-cicada` / `airflow-server` stay `Created`. Is that a bug?**
No — that is the documented ordering. `cm-ant` exits with `cb-tumblebug not initialized (Ready=true, Initialized=false)` and waits for `mayfly setup tumblebug-init` to register credentials (init.py). Run `tumblebug-init` and those services start.

**Q. Does `infra remove` delete my OpenBao data?**
`--clean-db` preserves OpenBao (data + `.env` token). `--clean-all` removes OpenBao data **and** clears `VAULT_TOKEN` so the next install re-inits cleanly. Plain `remove` (no flag) removes containers only.

**Q. `sealed:false` — why isn't that "ready"?**
Unsealing and becoming *active* are two steps. Right after unseal, OpenBao still answers `503` for a short window while it loads. The correct readiness signal is `GET /v1/sys/health == 200`.

**Q. Which secret goes to cb-tumblebug / mc-terrarium?**
The **root token** (as `VAULT_TOKEN`). Never the unseal key.

---

## 10. Relationship with cb-tumblebug's openbao-init

`mayfly` does **not** re-implement OpenBao initialization — it calls **cb-tumblebug's own** `init/openbao/openbao-init.sh` (cloned to match the running cb-tumblebug image tag). That script does the `POST /v1/sys/init`, unseals, saves the init output, and writes `VAULT_TOKEN` into the `.env`. `mayfly` invokes it as:

```
ENV_FILE=<abs .env>  INIT_OUTPUT=<abs openbao-init.json>  ./init/openbao/openbao-init.sh
```

**What `mayfly` depends on from that script** (the contract to watch when cb-tumblebug changes it):

1. **It writes `VAULT_TOKEN` into `ENV_FILE`.** `Init()` fails if `VAULT_TOKEN` is not set afterward. If upstream changes the env-file key name or stops writing it, our init detection breaks.
2. **The init output JSON shape.** We parse `openbao-init.json` for the unseal key under any of `keys` / `keys_base64` / `unseal_keys_hex` / `unseal_keys_b64`, and the token under `root_token`. The REST `POST /v1/sys/init` flow the script uses emits `keys[]` + `keys_base64[]` + `root_token`; the `bao operator init -format=json` CLI would instead emit `unseal_keys_hex`/`unseal_keys_b64`. We already accept both, but a **new key layout** would need a parser update (`internal/openbao.initFileShape`).
3. **`docker compose up -d openbao` brings up OpenBao alone** (depends_on flows dependent→dependency). If upstream restructures the compose so OpenBao pulls other services in, the staged flow changes.
4. **OpenBao image / unseal semantics.** A different OpenBao image or seal type (e.g. KMS auto-unseal) changes the sealed-on-restart assumption and the file-based unseal path.

**When cb-tumblebug changes its OpenBao init logic, check the four points above** and adjust `internal/openbao` (`Init`, `initFileShape`, `UnsealWith`, the preflight signals) plus this document accordingly. Because we call their script rather than duplicating it, most upstream changes flow through automatically — the risk is in the **contract** (env-file key, JSON shape, staged compose, seal type), not in the init steps themselves.

---

## 11. Auto-unseal sidecar & polling

The `openbao-unseal` sidecar polls OpenBao and re-unseals it whenever it is found sealed. Its cadence is set in one place — `conf/docker/.env`:

```
OPENBAO_UNSEAL_POLL_INTERVAL=30
```

This is the maximum number of seconds OpenBao can stay sealed after a restart before the sidecar unseals it again. The check is a lightweight status request, so a short interval has negligible cost.

**Disabling auto-unseal (manual mode).** To run without automatic unsealing — for example while trialing KMS auto-unseal — comment out the whole `openbao-unseal` service in `conf/docker/docker-compose.yaml`. After each restart, unseal by hand with `mayfly setup openbao unseal`. All the commands above work the same whether the sidecar runs or not.

---

## 12. Security — development / testing only

The default auto-unseal is **file-based**: during initialization the unseal key and root token are written **in plaintext** to

```
conf/docker/data/openbao/secrets/openbao-init.json   (chmod 600)
```

and both the sidecar and `setup openbao unseal` read that file to unseal. This is convenient but it defeats the intent of OpenBao's Shamir key sharing, so it is for **development and testing only**.

What this does and does not expose:

- It **cannot be stolen over the network alone** — the key file is not served on any port, and the OpenBao API never returns the unseal key.
- It **is** exposed by host compromise, a backup/snapshot leak, or an accidental commit of the secrets file — anyone who can read that file gets full access.

This trade-off is a property of file-based auto-unseal itself; it is the same whether the sidecar polls continuously or unseals once.

For production, remove the on-disk plaintext key by using one of:

- **KMS auto-unseal** (recommended) — add a `seal "awskms"` (or GCP/Azure) stanza to OpenBao's config. At startup OpenBao asks the KMS to decrypt its seal key via IAM, so **there is no plaintext unseal key on disk**.
- **Manual unseal** — keep the unseal key off the host (operator enters it after each restart). Most secure, at the cost of availability and operational effort.
- **HSM** — strongest, with higher cost and complexity.

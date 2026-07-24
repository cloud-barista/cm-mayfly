# Environment Configuration (`.env`)

The whole Cloud-Migrator docker-compose stack reads its settings from **one file**: `conf/docker/.env`. That file is gitignored and is **not** shipped in the repo — you create it from the template `conf/docker/.env.example` and fill in the secret values before running any `mayfly infra` command.

Because these values are externalized into a single file and **shared across containers** (the same credential is often consumed by two or three services), `mayfly` treats the file as required and validates it before starting anything. This document explains that policy and lists every variable and where it is used.

- [1. Create and fill `.env`](#1-create-and-fill-env)
- [2. Required vs optional](#2-required-vs-optional)
- [3. Variable reference (and where each is used)](#3-variable-reference-and-where-each-is-used)
- [4. Runtime-generated values (VAULT_TOKEN, AIRFLOW_JWT_SECRET)](#4-runtime-generated-values-vault_token-airflow_jwt_secret)

---

## 1. Create and fill `.env`

```
$ cp conf/docker/.env.example conf/docker/.env
# then edit conf/docker/.env and fill in the blank secret values (passwords, cb-spider REST auth, …)
```

`.env.example` already carries sensible defaults for every **non-secret** value (log levels, DB user/name, API usernames, `VAULT_ADDR`, the unseal poll interval, …), so in practice you only need to fill the blank **secret** entries. `mayfly` guards this in two steps, both **before** running docker compose:

- **File existence** — every `mayfly infra` subcommand except the bare `infra` (which only prints help) checks that `conf/docker/.env` exists and stops with a clear error if it does not.
- **Required values** — only the commands that actually start containers (`run` and `update`) go on to check that every required value is filled in. Tear-down and read-only subcommands (`remove`, `stop`, `install`, `info`, `logs`) skip this, so a missing key never blocks the very command you would use to fix or clean up the environment.

That way you never start the stack with unset variables, but you are also never locked out of removing one.

---

## 2. Required vs optional

**Every variable declared in `.env.example` is required (must be non-empty) — except the following, which are allowed to be blank:**

| Allowed to be blank | Why |
|---------------------|-----|
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_MAIL_FROM` | Email notifications are optional; the stack runs fine without them. |
| `VAULT_TOKEN` | Intentionally blank on a fresh install — it is generated and written by the OpenBao init flow during `mayfly infra run` (see [§4](#4-runtime-generated-values-vault_token-airflow_jwt_secret)). |
| `OPENBAO_UNSEAL_POLL_INTERVAL` | The compose file already substitutes a default (`${OPENBAO_UNSEAL_POLL_INTERVAL:-30}`), so a blank value never reaches the sidecar — `30` does. It is not a secret either. |

Everything else must be set. This is stronger than "just fill the passwords": it means a value you **accidentally blank or delete** — even a non-secret one that ships with a default — is caught before the stack starts. The reason is that a container which reads a blank value has **no safe built-in default of its own**, and because values are passed between containers, a downstream container cannot be assumed to know the right default for an upstream one. Requiring everything (except the explicit exceptions listed above) removes that whole class of silent, hard-to-diagnose failures.

The required-value check runs automatically as part of the container-starting commands (`run` and `update`) — not the tear-down or read-only subcommands. Example when something is blank or missing:

```
Error: required values are missing or blank in conf/docker/.env:
  - TUMBLEBUG_DB_USER

Starting the stack needs every variable in conf/docker/.env.example, because a container that reads
a blank value has no safe built-in default (e.g. cb-spider 0.12.17+ exits on blank REST auth; a postgres
healthcheck `pg_isready -U ${*_DB_USER}` fails on a blank user and deadlocks every dependent service).
Only the cm-cicada SMTP_* settings, VAULT_TOKEN (auto-generated on first run) and
OPENBAO_UNSEAL_POLL_INTERVAL (compose defaults it to 30) may be left blank.
Copy the defaults from conf/docker/.env.example and fill in the secret values, then re-run.

This check only guards the commands that start containers. If you are trying to tear the environment
down or look at it, `infra remove`, `stop`, `info` and `logs` run without it.
```

> **Changing what is optional (maintainers).** The required set is derived at runtime as *"every key in `.env.example` minus an exclusion list"*. The exclusion list is `optionalEnvKeys` in [`cmd/docker/root.go`](../cmd/docker/root.go); to make a newly added variable blank-allowed, add it there. Any variable **not** listed becomes required automatically — new variables are safe by default.

---

## 3. Variable reference (and where each is used)

`Req` = must be non-empty. `Default` shows the value shipped in `.env.example` (`(secret)` = blank, you must fill it). `Used by` lists the containers that consume the variable.

### cb-spider
| Variable | Req | Default | Used by |
|----------|:---:|---------|---------|
| `SPIDER_USERNAME` | ✅ | (secret) | cb-spider, cm-ant |
| `SPIDER_PASSWORD` | ✅ | (secret) | cb-spider, cm-ant |
| `SPIDER_LOG_LEVEL` | ✅ | `error` | cb-spider |
| `SPIDER_HISCALL_LOG_LEVEL` | ✅ | `error` | cb-spider |

### cb-tumblebug (+ its DB) and its API consumers
| Variable | Req | Default | Used by |
|----------|:---:|---------|---------|
| `TUMBLEBUG_DB_USER` | ✅ | `tumblebug` | cb-tumblebug, cb-tumblebug-postgres |
| `TUMBLEBUG_DB_PASSWORD` | ✅ | (secret) | cb-tumblebug, cb-tumblebug-postgres |
| `TUMBLEBUG_DB_NAME` | ✅ | `tumblebug` | cb-tumblebug, cb-tumblebug-postgres |
| `TB_API_USERNAME` | ✅ | `default` | cb-tumblebug, cm-cicada |
| `TB_API_PASSWORD` | ✅ | `default` | cb-tumblebug, cm-cicada |
| `TB_LOGLEVEL` | ✅ | `info` | cb-tumblebug |
| `TUMBLEBUG_API_USERNAME` | ✅ | `${TB_API_USERNAME}` | cm-ant (calls cb-tumblebug) |
| `TUMBLEBUG_API_PASSWORD` | ✅ | `${TB_API_PASSWORD}` | cm-ant (calls cb-tumblebug) |

### mc-terrarium / OpenBao
| Variable | Req | Default | Used by |
|----------|:---:|---------|---------|
| `TERRARIUM_API_USERNAME` | ✅ | `default` | mc-terrarium, cb-tumblebug |
| `TERRARIUM_API_PASSWORD` | ✅ | `default` | mc-terrarium, cb-tumblebug |
| `VAULT_ADDR` | ✅ | `http://openbao:8200` | cb-tumblebug, mc-terrarium |
| `VAULT_TOKEN` | ⚪ optional | (blank) | cb-tumblebug, mc-terrarium — see [§4](#4-runtime-generated-values-vault_token-airflow_jwt_secret) |
| `OPENBAO_UNSEAL_POLL_INTERVAL` | ⚪ optional | `30` | openbao-unseal — compose defaults it via `${...:-30}`, so a blank value is fine |

### cm-beetle
| Variable | Req | Default | Used by |
|----------|:---:|---------|---------|
| `BEETLE_API_USERNAME` | ✅ | `default` | cm-beetle, cm-cicada |
| `BEETLE_API_PASSWORD` | ✅ | (secret) | cm-beetle, cm-cicada |
| `BEETLE_LOGLEVEL` | ✅ | `debug` | cm-beetle |

### cm-damselfly
| Variable | Req | Default | Used by |
|----------|:---:|---------|---------|
| `DAMSELFLY_API_AUTH_ENABLED` | ✅ | `true` | cm-damselfly |
| `DAMSELFLY_API_USERNAME` | ✅ | `default` | cm-damselfly, cm-cicada |
| `DAMSELFLY_API_PASSWORD` | ✅ | `default` | cm-damselfly, cm-cicada |

### cm-ant (+ its DB)
| Variable | Req | Default | Used by |
|----------|:---:|---------|---------|
| `ANT_DB_USER` | ✅ | `cm-ant-user` | ant-postgres, cm-ant |
| `ANT_DB_PASSWORD` | ✅ | (secret) | ant-postgres, cm-ant |
| `ANT_DB_NAME` | ✅ | `cm-ant-db` | ant-postgres, cm-ant |

### cm-butterfly (+ its DB)
| Variable | Req | Default | Used by |
|----------|:---:|---------|---------|
| `BUTTERFLY_DB_USER` | ✅ | `butterflyadmin` | cm-butterfly-api, cm-butterfly-db |
| `BUTTERFLY_DB_PASSWORD` | ✅ | (secret) | cm-butterfly-api, cm-butterfly-db |
| `BUTTERFLY_DB_NAME` | ✅ | `butterfly-db` | cm-butterfly-api, cm-butterfly-db |

### Airflow (workflow engine backing cm-cicada)
| Variable | Req | Default | Used by |
|----------|:---:|---------|---------|
| `AIRFLOW_DB_USER` | ✅ | `airflow` | airflow-mysql, airflow-server |
| `AIRFLOW_DB_PASSWORD` | ✅ | (secret) | airflow-mysql, airflow-server |
| `AIRFLOW_DB_ROOT_PASSWORD` | ✅ | (secret) | airflow-mysql |
| `AIRFLOW_DB_NAME` | ✅ | `airflow` | airflow-mysql, airflow-server |
| `AIRFLOW_JWT_SECRET` | ✅ | (blank — auto-generated) | airflow-server — see [§4](#4-runtime-generated-values-vault_token-airflow_jwt_secret) |

### Email notifications (optional)
| Variable | Req | Default | Used by |
|----------|:---:|---------|---------|
| `SMTP_HOST` | ⚪ optional | `smtp.gmail.com` | airflow-server |
| `SMTP_PORT` | ⚪ optional | `587` | airflow-server |
| `SMTP_USER` | ⚪ optional | (blank) | airflow-server |
| `SMTP_PASSWORD` | ⚪ optional | (blank) | airflow-server |
| `SMTP_MAIL_FROM` | ⚪ optional | (blank) | airflow-server |

> Note the **shared credentials**: `SPIDER_*` (cb-spider + cm-ant), `TB_API_*` (cb-tumblebug + cm-cicada), `TERRARIUM_API_*` (mc-terrarium + cb-tumblebug), `BEETLE_API_*` (cm-beetle + cm-cicada), `DAMSELFLY_API_*` (cm-damselfly + cm-cicada), and `VAULT_*` (cb-tumblebug + mc-terrarium). A single `.env` value drives every consumer, so changing it in one place updates them all — which is exactly why leaving one blank breaks more than one service.

---

## 4. Runtime-generated values (VAULT_TOKEN, AIRFLOW_JWT_SECRET)

Two values ship blank in `.env.example` and are produced at runtime instead of by hand. They look similar but are handled differently, because one is **optional** and the other stays **required**.

### `VAULT_TOKEN` — optional, generated by OpenBao

`VAULT_TOKEN` is the OpenBao root token that cb-tumblebug and mc-terrarium use to reach the secret store. It is **not** something you fill in the template: on a fresh install `mayfly infra run` initializes OpenBao and writes the generated token back into `.env` automatically. That is why it is blank in `.env.example` and **excluded from the required check** (it is in `optionalEnvKeys`) — requiring it would block the very auto-init that produces it.

Everything about how the token is generated, injected, re-unsealed after restarts, and recovered is documented in [openbao-unseal.md](openbao-unseal.md).

### `AIRFLOW_JWT_SECRET` — required, generated locally on first run

`AIRFLOW_JWT_SECRET` is the secret Airflow (the workflow engine behind cm-cicada) uses to sign the JWTs its own processes exchange. Any unguessable random string works, as long as every Airflow process sees the same one. It is **required** — a blank value would break Airflow — but you never have to invent it: on the first `run`/`update`, `mayfly` fills a blank entry with a fresh 32-byte random value and writes it back into `.env` before the required-value check runs. So it stays required, but a blank line is treated as "generate me", not as an error.

The difference from `VAULT_TOKEN` is worth keeping straight: `AIRFLOW_JWT_SECRET` is generated **locally by mayfly** (`generatedEnvKeys` in [`cmd/docker/root.go`](../cmd/docker/root.go)) and remains a required key, whereas `VAULT_TOKEN` is generated **by OpenBao's init flow** and is optional. Once `AIRFLOW_JWT_SECRET` is set it is never rotated automatically — changing it invalidates already-issued tokens, so queued tasks would fail.

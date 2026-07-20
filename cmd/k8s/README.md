# `mayfly k8s` (not built)

This package holds a Helm-based Kubernetes deployment path for Cloud-Migrator —
`run`, `stop`, `info`, `update` and `remove` against a chart instead of Docker
Compose. It worked when it was written, but that was several years ago.

It is not part of the CLI today. `main.go` does not import it (the import line is
commented out), so `mayfly k8s` is an unknown command, and every file here now
carries a `//go:build k8s` tag so the package is excluded from a normal build as
well.

## Why it is kept

Kubernetes support is still planned. When it comes back it will most likely be
rebuilt around the Kubernetes API rather than shelling out to `helm`, but the
command layout, the flag set and the chart values in `conf/k8s/` are a useful
starting point, and throwing them away would mean rediscovering decisions that
were already made once.

## Why it is not simply re-enabled

Kubernetes has moved a long way since this code was last exercised: API versions
have been removed, the Helm interface has changed, and the assumptions the
commands make about cluster access have not been checked against a current
cluster. Turning it back on would need re-verification and a fair amount of
rework — it is not a matter of uncommenting the import.

## Building it

```sh
go build -tags k8s ./...
```

The tag exists so the intent is visible in the code itself, and so the package
is not compiled, linted, or security-scanned as if it were shipping. Restoring
the import in `main.go` alone will not enable it.

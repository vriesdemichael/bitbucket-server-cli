# Quickstart

## Install

Download a release artifact, extract `bb`, and run:

```bash
./bb --help
```

## Authenticate

```bash
go run ./cmd/bb auth login --host http://localhost:7990 --username admin --password admin
go run ./cmd/bb auth status
```

## Run a command

```bash
go run ./cmd/bb search repos --limit 20
```

## Machine mode

Use `--json` for machine-readable responses:

```bash
go run ./cmd/bb --json auth status
```

Envelope contract is `bb.machine` on version `v2`.

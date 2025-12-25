# Terraform Provider Purelymail

This provider manages Purelymail resources via the Purelymail API.

## Requirements

- Terraform >= 1.0
- Go >= 1.24

## Building the provider

```sh
go install
```

## Acceptance tests

```sh
TF_ACC=1 go test ./internal/provider -timeout 120s
```

## Documentation

Generate docs:

```sh
go generate ./...
```

Examples live under `examples/`; generated docs under `docs/`.

# Inference policy proxy

Independent Go service, deployed with `helm/infra/inference`. Worldbuilder and operator experiments share authenticated OpenAI-compatible chat routes. Local has no monetary/daily quota; explicitly selected Bedrock requests share an atomic $100 UTC-month PostgreSQL budget. No automatic paid fallback or billed-request replay.

Read [the operator guide](../../docs/Inference.md) for catalogue settings, API limits, private access, certificate renewal, pricing approval, model maintenance, backup and staged activation. The default chart disables Bedrock and uses the existing Helheim Qwen model. Embeddings are unchanged.

Runtime environment:

- `INFERENCE_CONFIG`: JSON catalogue/config file.
- `INFERENCE_CLIENTS`: JSON client credential file; distinct secrets of at least 32 characters.
- `INFERENCE_DATABASE_URL`: isolated `inference_runtime` connection, required for cloud; local remains available during database failure.

The HTTP listener defaults to port 8080. For local development set `INFERENCE_LISTEN=127.0.0.1:18081` to bind only loopback.

Run `go test -race ./...` here. From repository root run `bash scripts/inference/test-postgres.sh` for concurrent reservations, rollover, tariff/duplicate/role/recovery and populated restore checks. `scripts/inference/test-manifests.py` checks route/secret/overlay contracts; `test-gateway.py` exercises actual controllers with synthetic providers. All database/controller tests create owned disposable resources and never use live AWS access.

The ledger reserves worst-case billable input and allowed output before dispatch. Unknown outcomes keep their reservation indefinitely; the concurrency lease is separate. Missing usage, expired rates and failed accounting never trigger a retry or refund. Usage and metrics contain estimated costs, not invoices. Do not log request bodies, credentials or certificate keys.

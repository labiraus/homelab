# Worldbuilder AI on Labiraus

Worldbuilder is a collaborative worldbuilding application running on the Labiraus
homelab. PostgreSQL owns canon and immutable history; people review and publish
changes. AI generates provisional material. The Labiraus MCP server and existing
document tools remain supported integration surfaces.

Start with the [documentation index](docs/README.md).

- [Architecture](docs/architecture.md): services, data ownership and repository map.
- [Development](docs/development.md): devcontainer, local commands and CI/Flux delivery.
- [Worldbuilder API](docs/worldbuilder/api.md) and [experience](docs/worldbuilder/experience.md): implemented contracts and product requirements.
- [Operations](docs/operations.md), [recovery](docs/recovery.md) and [troubleshooting](runbooks/troubleshooting.md): running the estate.
- [Status](docs/status.md): source capabilities, dated evidence and outstanding work.
- [AGENTS.md](AGENTS.md): repository operating policy for agents.

SeaweedFS is the private external S3 target on `svartalfheim`. MinIO remains the
migration/rollback source until the [storage cutover gates](docs/storage.md) pass.
Source configuration and historical deployment evidence do not establish present
production health.

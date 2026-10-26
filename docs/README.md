# Documentation

These guides describe the checked-in system. [Status](status.md) distinguishes
source capabilities from dated publication and deployment evidence. Superseded
plans and incident narratives remain available in Git history.

## Start by task

| Task | Read |
| --- | --- |
| Understand the system | [Architecture](architecture.md), [design decisions](worldbuilder/decisions.md) |
| Develop and test | [Development](development.md), [testing](testing.md), the relevant component README |
| Change Worldbuilder | [API](worldbuilder/api.md), [experience and journeys](worldbuilder/experience.md) |
| Provision hosts | [Infrastructure](infrastructure.md), [Ansible](../ansible/README.md) |
| Configure access | [Authentication](authentication.md), [secrets](secrets.md) |
| Operate or recover | [Operations](operations.md), [recovery](recovery.md), [troubleshooting](../runbooks/troubleshooting.md) |
| Migrate storage | [Storage](storage.md) |
| Work with models | [Inference](inference.md) |
| Use document capabilities | [Documents](documents.md), [MCP](mcp.md) |
| Operate Minecraft | [Minecraft](minecraft.md) |
| Find remaining work/evidence | [Status](status.md) |

## Component references

Runtime configuration and local commands live beside
[external](../apps/external/README.md), [MCP](../apps/mcp/README.md),
[orchestrator](../apps/orchestrator/README.md), [processor](../apps/processor/README.md),
[inference](../apps/inference/README.md), and [UI](../apps/ui/README.md).
Migration inventories live in [Worldbuilder SQL](../sql/worldbuilder/README.md)
and [inference SQL](../sql/inference/README.md). Evaluation entry points are in
[Worldbuilder evaluations](../evals/worldbuilder/README.md) and
[document retrieval evaluations](../evals/ragas/README.md).

## Maintenance

Each topic has one canonical guide. Component READMEs own local commands and
configuration; guides own cross-component contracts and procedures. Update the
existing section when behavior changes. Do not append another status notice above
obsolete instructions or duplicate a procedure in agent memory.

Keep requirements explicitly separate from source implementation. Record evidence
once in status with its date, source/artifact identity, tested boundary and remaining
limits. Neither a prior success nor a prior outage proves current health. Keep
private reports private and mark unavailable evidence honestly.

Agents follow the root [policy](../AGENTS.md), then the most specific applicable
`AGENTS.md`. Topic guides are reference material, not additional grants of authority.

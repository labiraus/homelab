# Native MCP authorization boundary

- This is the MCP authorization server, not a replacement for Google website login. Browser consent requires the exact OAuth proxy mesh identity, a registered user, same-origin POST and an independent secure CSRF cookie.
- Keep S256 PKCE, exact registered redirects (only loopback port substitution), resource binding, single-use codes and explicit per-client consent. Never mint a Google token or accept caller identity headers at token validation.
- Store credential hashes only. Code exchange is transactional; refresh rotation and revocation acquire the same family lock before row locks. Retain spent refresh tokens until expiry so replay revokes descendants across replicas.
- Use a dedicated non-owner database role with no canon or auth-user write grants. Apply migrations explicitly and rehearse on an isolated restore. Startup cannot modify schemas.
- Keep native login disabled until protocol/database tests, composed chart refusal checks and live mesh isolation pass. A successful Google browser login or metadata response is not native client acceptance.
- Test with the existing owned PostgreSQL harness; fixture browser identity shortcuts belong only in `_test.go`. Never redirect tests to production or persist fixture credentials in the operator's Codex configuration.
- Database restoration may resurrect a previously revoked credential. Revoke all stored token families and clear pending grants on an isolated restore before exposing it; preserve client identities and require fresh browser consent. Do not reset credentials on ordinary process replacement.

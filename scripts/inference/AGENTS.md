# Inference acceptance

- Reuse the owned Worldbuilder PostgreSQL transport with generated names matching `worldbuilder-test-<digits>-<digits>` and its exact ownership label. Its exec bridge deliberately rejects other prefixes, including inference-specific names. Keep the database itself isolated as `inference_test`.
- Offline accounting, render and controller checks never use AWS credentials or production DSNs. Live Bedrock evaluation must explicitly select a paid profile through the policy proxy and uses the same monthly budget as other clients.
- Rendered credential fixtures contain public certificate/ARN placeholders only. Never retain client tokens, private keys or kubeconfigs in acceptance artifacts.
- For disposable kind diagnostics use `Fixture.kubectl` or the owned exec-ID shim with both `WORLDBUILDER_KIND_NAME` and the original `WORLDBUILDER_DOCKER_CLI`; its generated host kubeconfig may have an unreachable Docker-outside-Docker loopback address. Behind TLS inspection set `WORLDBUILDER_TEST_CA_BUNDLE` to the trusted public CA bundle before fixture creation; never bypass TLS.
- The official AWS helper Amzn2023 binary dynamically links glibc and libresolv. Use a compatible base runtime, not distroless/static, and execute the built image's version/help command before claiming image acceptance.
- Synthetic IMDSv2 token responses must return `X-Aws-Ec2-Metadata-Token-Ttl-Seconds`; the Go AWS SDK rejects a token without it. Inspect the actual consuming `ai-gateway-extproc` native init sidecar as well as ordinary containers when credential acceptance fails.

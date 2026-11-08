# Pub/Sub helper

This optional Google Pub/Sub integration uses the v2 Go client. `GetTopic` returns
a cached publisher; topic and subscription provisioning uses the client's admin
APIs. Missing resources are created only when the corresponding configuration flag
allows it. Permission and transport failures never trigger creation.

Run `go test -race ./...` in this directory. The tests use an in-memory gRPC server
and do not access a Google account. The primary document pipeline uses NATS; this
package does not change that deployment.

# MCP contract properties

Hegel is a test-only dependency in this separate module. It does not enter
Skillshare's production binary or root Go module. Ordinary MCP regressions
also run in the existing package/integration suites.

Inside the supported devcontainer or remote CI:

```sh
cd tests/property
go get hegel.dev/go/hegel@latest
go test -mod=mod -count=1 -timeout 5m ./...
```

The update deliberately uses the newest Hegel release. The initial module
records v0.9.30 (verified from the upstream Go release and Go checksum database).
Hegel's tagged Go module embeds its native property engine; no production
runtime is added. The command may refresh this module's dependency files.
All network endpoints are local inert `httptest` fixtures; no real MCP service,
user configuration or reference-project fixture is used.

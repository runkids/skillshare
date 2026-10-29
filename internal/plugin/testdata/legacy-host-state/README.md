# Legacy plugin fixture

Produced with `Service.Preview`, `Apply`, and `save` from upstream commit
`e64fac67c3a254900bc299f35309bfcf0e936233`, in an isolated baseline checkout,
using fake native clients and scratch HOME/config/state. The source Git clone
was redirected to a scratch repository, without contacting a service.

The producer added the nested Pi/OpenCode candidate at source ref `main`,
disabled OpenCode, imported a filtered Pi native entry, then attempted an
alternate Pi-account distribution with an injected native failure. Snapshot
owner/digest markers and content are copied from the producer's snapshot.

`__HOST__` and `__ALTERNATE__` replace scratch paths. The private Pi record is intentionally absent: these shared IDs are foreign
proof candidates, and the tests must not adopt them without local evidence. Native directory names and resolved Git
commit remain the actual producer output. All package code is an inert fixture.

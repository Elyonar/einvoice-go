# Publishing einvoice-go

A Go module is published by a git tag: there is no registry upload. The module path is the
lower-case `github.com/elyonar/einvoice-go` (GitHub serves the `Elyonar` organisation
case-insensitively; the module path must stay lower-case forever, it is what every `go.mod` records).
pkg.go.dev and the module proxy index the tag on first request; nothing to configure.

## Releasing

1. Merge the changes to `main`.
2. On `main`: set the new version in `version.go` (`const Version = "0.1.0"`), move the CHANGELOG's
   unreleased section under the version and date, run `make guides` (it records the version), commit
   `chore(release): <version>` and push.
3. The release workflow runs on a `version.go` change on `main`: lint → build → test → sync-check →
   guides-check → tag `v<version>` → GitHub Release → a `go list -m` against proxy.golang.org so
   pkg.go.dev picks the version up. It skips when the tag already exists.
4. Integrators then `go get github.com/elyonar/einvoice-go@v0.1.0`.

Stay on `v0.x` (no `/v2` module path suffix) until the API surface is declared stable; a `v1.0.0`
tag is the owner's call. The SDKs are versioned independently of einvoice-js: the einvoice-js commit
(or tag) pinned in `scripts/sync.sh` records which JS release this one tracks.

Local fallback: `git tag -a v<version> -m "Release v<version>" && git push origin v<version>`.

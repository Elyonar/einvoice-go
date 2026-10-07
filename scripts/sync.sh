#!/usr/bin/env sh
# Syncs this SDK with einvoice-js, the source of the API snapshot, the webhook test vectors and the
# generated models (see CLAUDE.md "Source of truth"). Development tooling; needs git and Node 22.
#
#   make sync                 copy the snapshot and vectors from the pinned einvoice-js commit and
#                             regenerate types_gen.go
#   make sync-check           exit 1 when any of the three is not what the pinned commit produces (CI)
#
#   EINVOICE_JS_REF=<sha|tag|branch>   the einvoice-js commit to pin (default below; bump it to take a
#                                      new API surface — a commit SHA needs no einvoice-js release)
#   EINVOICE_JS_DIR=<path>             use a local einvoice-js checkout (any folder name) instead of fetching
#   EINVOICE_JS_TOKEN=<token>          a GitHub token that may read einvoice-js while it is private (CI: a secret)
set -eu

# einvoice-js feat/codegen-emitters (PR #7): the generator with the Go emitter, snapshot of the
# 106 API-key operations at API 1.0, and the webhook vectors.
EINVOICE_JS_REF="${EINVOICE_JS_REF:-885f1cd8d0f26fac0bd3cbf98129e09ebefbab1c}"
EINVOICE_JS_REPO="${EINVOICE_JS_REPO:-https://github.com/Elyonar/einvoice-js.git}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CHECK=""
[ "${1:-}" = "--check" ] && CHECK=1

if [ -n "${EINVOICE_JS_DIR:-}" ]; then
    SRC="$(cd "$EINVOICE_JS_DIR" && pwd)"
    echo "sync: using local einvoice-js at $SRC"
else
    SRC="$(mktemp -d)"
    trap 'rm -rf "$SRC"' EXIT
    echo "sync: fetching einvoice-js $EINVOICE_JS_REF"
    # A shallow fetch of one ref: a SHA, a tag or a branch all work (GitHub serves reachable SHAs).
    git -C "$SRC" init --quiet
    if [ -n "${EINVOICE_JS_TOKEN:-}" ]; then
        # A private einvoice-js: authenticate with a token that may read it (never echoed; not in the URL).
        AUTH="AUTHORIZATION: basic $(printf 'x-access-token:%s' "$EINVOICE_JS_TOKEN" | base64 | tr -d '\n')"
        git -C "$SRC" -c "http.extraheader=$AUTH" fetch --quiet --depth 1 "$EINVOICE_JS_REPO" "$EINVOICE_JS_REF"
    else
        git -C "$SRC" fetch --quiet --depth 1 "$EINVOICE_JS_REPO" "$EINVOICE_JS_REF"
    fi
    git -C "$SRC" checkout --quiet FETCH_HEAD
fi

SNAPSHOT="$ROOT/scripts/openapi-api-key-ops.json"
VECTORS="$ROOT/tests/vectors/webhook-signature.json"
GENERATED="$ROOT/types_gen.go"
status=0

if [ -n "$CHECK" ]; then
    cmp -s "$SRC/scripts/openapi-api-key-ops.json" "$SNAPSHOT" || { echo "scripts/openapi-api-key-ops.json differs from einvoice-js $EINVOICE_JS_REF: run make sync"; status=1; }
    cmp -s "$SRC/scripts/vectors/webhook-signature.json" "$VECTORS" || { echo "tests/vectors/webhook-signature.json differs from einvoice-js $EINVOICE_JS_REF: run make sync"; status=1; }
    node "$SRC/scripts/gen-types.mjs" --lang go --snapshot "$SNAPSHOT" --out "$GENERATED" --check || status=1
    [ "$status" = 0 ] && echo "sync: snapshot, vectors and generated models are current"
    exit $status
fi

mkdir -p "$(dirname "$VECTORS")"
cp "$SRC/scripts/openapi-api-key-ops.json" "$SNAPSHOT"
cp "$SRC/scripts/vectors/webhook-signature.json" "$VECTORS"
node "$SRC/scripts/gen-types.mjs" --lang go --snapshot "$SNAPSHOT" --out "$GENERATED"
echo "sync: done (einvoice-js $EINVOICE_JS_REF)"

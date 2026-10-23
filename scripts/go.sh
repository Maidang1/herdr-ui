#!/bin/sh
# Every build tool (including the mygo packager's child go commands) uses the
# exact pinned module and the same minimal composition capability patch.
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
python3 "$ROOT/scripts/prepare-mygo.py"
export GOWORK="$ROOT/.build/go.work"
exec go "$@"

#!/usr/bin/env python3
"""Prepare a version-pinned mygo workspace with the public composition query.

Never edits the Go module cache. The only upstream change is an atomic terminal
composition flag, a getter, and updates from compose, commit, and focus loss.
"""
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parent.parent
MODULE = "github.com/egoist/mygo"


def replace_once(source, before, after, filename):
    count = source.count(before)
    if count != 1:
        raise RuntimeError(f"{filename}: expected exactly one patch anchor, found {count}: {before!r}")
    return source.replace(before, after, 1)


def prepare_locked():
    match = re.search(r"^\s*github\.com/egoist/mygo\s+(v[^\s]+)\s*$", (ROOT / "go.mod").read_text(), re.M)
    if not match:
        raise RuntimeError("go.mod must pin exactly one mygo version")
    version = match.group(1)
    build = ROOT / ".build"
    dest = build / "mygo"
    stamp = {"version": version, "patch_sha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest()}
    stamp_path = dest / ".herdr-ui-patch.json"
    work = build / "go.work"
    go_version = re.search(r"^go\s+(\S+)", (ROOT / "go.mod").read_text(), re.M).group(1)
    workspace = f"go {go_version}\n\nuse (\n\t..\n\t./mygo\n)\n"
    if stamp_path.exists() and json.loads(stamp_path.read_text()) == stamp and work.exists() and work.read_text() == workspace:
        return
    env = dict(os.environ, GOWORK="off")
    metadata = json.loads(subprocess.check_output(["go", "mod", "download", "-json", f"{MODULE}@{version}"], cwd=ROOT, env=env))
    if metadata.get("Error") or metadata.get("Version") != version or not metadata.get("Dir"):
        raise RuntimeError(f"unable to download pinned mygo: {metadata}")
    build.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="mygo-prepare-", dir=build) as staging:
        staged = Path(staging) / "mygo"
        shutil.copytree(metadata["Dir"], staged)
        # Module cache files are read-only. Only the private generated copy changes.
        for directory, dirs, files in os.walk(staged):
            Path(directory).chmod(0o755)
            for file in files:
                path = Path(directory) / file
                path.chmod(path.stat().st_mode | 0o200)
        terminal = staged / "plugins/terminal/terminal.go"
        source = terminal.read_text()
        source = replace_once(source, "\tcode     atomic.Int64", "\tcode     atomic.Int64\n\tcomposing atomic.Bool", terminal)
        source = replace_once(source, "// gridSize is the size of the screen in cells and of a cell in pixels.", "// Composing reports whether the view has active input-method preedit text.\n// It is safe to call concurrently with terminal input and rendering.\nfunc (t *Terminal) Composing() bool { return t.composing.Load() }\n\n// gridSize is the size of the screen in cells and of a cell in pixels.", terminal)
        terminal.write_text(source)
        view = staged / "plugins/terminal/view.go"
        source = view.read_text()
        source = replace_once(source, "case ui.InputCompose:\n\t\tv.preedit,", "case ui.InputCompose:\n\t\tv.t.composing.Store(ev.Text != \"\")\n\t\tv.preedit,", view)
        source = replace_once(source, "func (v *view) typed(text string) {\n\tv.preedit = \"\"", "func (v *view) typed(text string) {\n\tv.t.composing.Store(false)\n\tv.preedit = \"\"", view)
        source = replace_once(source, "\t\t\tv.preedit, v.pending = \"\", nil", "\t\t\tt.composing.Store(false)\n\t\t\tv.preedit, v.pending = \"\", nil", view)
        view.write_text(source)
        subprocess.run(["gofmt", "-w", str(terminal), str(view)], check=True, env=env)
        (staged / ".herdr-ui-patch.json").write_text(json.dumps(stamp, sort_keys=True) + "\n")
        if dest.exists():
            shutil.rmtree(dest)
        staged.rename(dest)
    work.write_text(workspace)


def prepare():
    build = ROOT / ".build"
    build.mkdir(exist_ok=True)
    # Serialize the stamp check and publication across build/package/test
    # invocations. The lock is local to this generated workspace.
    with (build / "prepare.lock").open("a") as lock:
        fcntl.flock(lock.fileno(), fcntl.LOCK_EX)
        prepare_locked()


if __name__ == "__main__":
    prepare()

#!/usr/bin/env python3
"""Prepare a version-pinned mygo workspace with small terminal patches.

Never edits the Go module cache. The upstream changes are an atomic terminal
composition flag, a getter, and updates from compose, commit, and focus loss;
and SetDragSelects, which lets a drag select text while the program takes the
mouse. `herdr terminal attach` always takes the mouse, even over a shell that
does not, so without it the pointer could never select. SetOptionAsAlt and
SetCursor let settings change those options on running terminals, and an
OptionAsAlt element property keeps Option keys from macOS input methods, which
otherwise typed the Option character after the Meta key.
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

DRAG_SELECTS_METHOD = """// SetDragSelects has a drag select while the program takes the mouse, as one
// with Shift held does, while a click still goes to the program whole.
func (t *Terminal) SetDragSelects(on bool) { t.dragSelects.Store(on) }
"""

SETTERS = """// SetOptionAsAlt changes Options.OptionAsAlt, as when the user changes a
// setting: the next key takes it.
func (t *Terminal) SetOptionAsAlt(on bool) {
	t.mu.Lock()
	t.opts.OptionAsAlt = on
	t.mu.Unlock()
}

// SetCursor changes Options.Cursor and Options.NoBlink, the cursor shown
// until a program asks for another, as when the user changes a setting.
func (t *Terminal) SetCursor(style CursorStyle, noBlink bool) {
	t.mu.Lock()
	t.opts.Cursor, t.opts.NoBlink = style, noBlink
	if t.term != nil {
		t.term.SetDefaultCursor(vtCursor(style), !noBlink)
	}
	t.mu.Unlock()
	t.redraw()
}

"""

POINTER_INPUT = """// pointerInput holds a primary press back from a program that takes the
// mouse while SetDragSelects is on: dragging out of its cell selects from
// it, as with Shift held, and releasing in it reports the whole click and
// clears the selection, as a click does without the program.
func (v *view) pointerInput(ev ui.InputEvent) bool {
	t := v.t
	if v.dragging {
		v.dragging = ev.Kind != ui.InputPointerUp
		ev.Mods |= ui.Shift
		return v.pointerEvent(ev)
	}
	t.mu.Lock()
	tracking := t.term != nil && t.term.MouseTracking()
	col, row := v.cellAt(ev.X, ev.Y)
	t.mu.Unlock()
	if v.held == nil {
		if !t.dragSelects.Load() || !tracking || v.selecting || v.reporting || v.linkPress || ev.Kind != ui.InputPointerDown || ev.Button != 0 || ev.Mods&(ui.Shift|ui.Cmd) != 0 {
			return v.pointerEvent(ev)
		}
		press := ev
		v.held, v.heldCell = &press, [2]int{col, row}
		return true
	}
	if ev.Kind == ui.InputPointerMove && v.heldCell == [2]int{col, row} {
		return true
	}
	press := *v.held
	v.held = nil
	if ev.Kind == ui.InputPointerMove {
		v.dragging = true
		press.Mods |= ui.Shift
		ev.Mods |= ui.Shift
		v.pointerEvent(press)
		return v.pointerEvent(ev)
	}
	v.clearSelection()
	v.pointerEvent(press)
	return v.pointerEvent(ev)
}
"""


def replace_once(source, before, after, filename):
    count = source.count(before)
    if count != 1:
        raise RuntimeError(f"{filename}: expected exactly one patch anchor, found {count}: {before!r}")
    return source.replace(before, after, 1)


def patch_option_as_alt(staged):
    """Let a terminal using Option as Alt keep Option keys from the input
    method on macOS: they typed the Option character after the Meta key, and
    dead keys such as Option-E started a composition. Returns the files."""
    edits = [
        ("ui/element.go", "\tnoIME      bool // InputMethod(false)\n", "\tnoIME      bool // InputMethod(false)\n\toptionAlt  bool // OptionAsAlt(true)\n"),
        ("ui/handler.go", "func (e *node) InputMethod(on bool) *node {", "// OptionAsAlt keeps keys typed with Option on macOS from the input\n// methods, while they compose nothing, so that the element alone makes them\n// Alt (Meta) keys, as a terminal's option-as-alt setting does.\nfunc (e *node) OptionAsAlt(on bool) *node {\n\te.optionAlt = on\n\treturn e\n}\n\nfunc (e *node) InputMethod(on bool) *node {"),
        ("ui/api_gen.go", "func (_handle Element) InputMethod(on bool) Element {", "// OptionAsAlt keeps Option keys from the input methods on macOS.\nfunc (_handle Element) OptionAsAlt(on bool) Element {\n\t_node := _handle.node()\n\tif _node == nil {\n\t\treturn Element{}\n\t}\n\treturn wrapElement(_node.OptionAsAlt(on))\n}\n\nfunc (_handle Element) InputMethod(on bool) Element {"),
        ("ui/layoutcache_test.go", "takesText noIME copyData", "takesText noIME optionAlt copyData"),
        ("ui/context.go", "\tnoIME       bool\n", "\tnoIME       bool\n\toptionAlt   bool\n"),
        ("ui/runtime.go", "s.input, s.caret, s.takesText, s.noIME = e.inputFn, e.caret, e.takesText, e.noIME", "s.input, s.caret, s.takesText, s.noIME, s.optionAlt = e.inputFn, e.caret, e.takesText, e.noIME, e.optionAlt"),
        ("ui/input.go", "\t\tt.Active, t.NoInputMethod = true, s.noIME\n", "\t\tt.Active, t.NoInputMethod, t.OptionAsAlt = true, s.noIME, s.optionAlt\n"),
        ("internal/platform/surface.go", "\tNoInputMethod bool\n}", "\tNoInputMethod bool\n\t// OptionAsAlt keeps keys typed with Option from input methods on\n\t// macOS while they compose nothing: the key alone tells them.\n\tOptionAsAlt bool\n}"),
        ("internal/darwin/surface.go", "\t\t\ttext := s.input.Active && mods&(platform.ModSuper|platform.ModCtrl) == 0\n", "\t\t\toptionAlt := s.input.OptionAsAlt && mods&platform.ModAlt != 0 && !s.hasMarkedText()\n\t\t\ttext := s.input.Active && mods&(platform.ModSuper|platform.ModCtrl) == 0 && !optionAlt\n"),
    ]
    files = []
    for name, before, after in edits:
        path = staged / name
        path.write_text(replace_once(path.read_text(), before, after, path))
        files.append(path)
    return files


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
        source = replace_once(source, "\tcode     atomic.Int64", "\tcode     atomic.Int64\n\tcomposing atomic.Bool\n\tdragSelects atomic.Bool", terminal)
        source = replace_once(source, "// gridSize is the size of the screen in cells and of a cell in pixels.", "// Composing reports whether the view has active input-method preedit text.\n// It is safe to call concurrently with terminal input and rendering.\nfunc (t *Terminal) Composing() bool { return t.composing.Load() }\n\n" + DRAG_SELECTS_METHOD + "\n// gridSize is the size of the screen in cells and of a cell in pixels.", terminal)
        source = replace_once(source, "// SetMinimumContrast changes the least contrast of text with its", SETTERS + "// SetMinimumContrast changes the least contrast of text with its", terminal)
        terminal.write_text(source)
        view = staged / "plugins/terminal/view.go"
        source = view.read_text()
        source = replace_once(source, "case ui.InputCompose:\n\t\tv.preedit,", "case ui.InputCompose:\n\t\tv.t.composing.Store(ev.Text != \"\")\n\t\tv.preedit,", view)
        source = replace_once(source, "func (v *view) typed(text string) {\n\tv.preedit = \"\"", "func (v *view) typed(text string) {\n\tv.t.composing.Store(false)\n\tv.preedit = \"\"", view)
        source = replace_once(source, "\t\t\tv.preedit, v.pending = \"\", nil", "\t\t\tt.composing.Store(false)\n\t\t\tv.preedit, v.pending = \"\", nil", view)
        source = replace_once(source, "\tselecting, reporting bool\n", "\tselecting, reporting bool\n\t// held is a primary press SetDragSelects keeps from the program until\n\t// it drags out of heldCell or is released; dragging that it dragged.\n\theld     *ui.InputEvent\n\theldCell [2]int\n\tdragging bool\n", view)
        source = replace_once(source, "\t\ttook := v.pointerEvent(ev)\n", "\t\ttook := v.pointerInput(ev)\n", view)
        source = replace_once(source, "\tif v.tracking && mods&ui.Shift != 0 {", "\tif v.tracking && (mods&ui.Shift != 0 || v.t.dragSelects.Load()) {", view)
        source = replace_once(source, "// pointerEvent reports the pointer to the program that takes the mouse, or\n", POINTER_INPUT + "\n// pointerEvent reports the pointer to the program that takes the mouse, or\n", view)
        source = replace_once(source, "\te.TextCaret(v.caret())\n", "\te.TextCaret(v.caret())\n\te.OptionAsAlt(t.opts.OptionAsAlt)\n", view)
        view.write_text(source)
        patched = [terminal, view] + patch_option_as_alt(staged)
        subprocess.run(["gofmt", "-w", *map(str, patched)], check=True, env=env)
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

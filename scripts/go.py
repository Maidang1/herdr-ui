#!/usr/bin/env python3
"""Run Go against the same patched workspace on Unix and native Windows."""
import os
from pathlib import Path
import runpy
import subprocess
import sys

root = Path(__file__).resolve().parent.parent
runpy.run_path(str(root / "scripts" / "prepare-mygo.py"), run_name="__main__")
env = dict(os.environ, GOWORK=str(root / ".build" / "go.work"))
raise SystemExit(subprocess.call(["go", *sys.argv[1:]], cwd=root, env=env))

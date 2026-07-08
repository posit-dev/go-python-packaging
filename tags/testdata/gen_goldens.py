# Regenerate goldens:
#   python3 -m pip install 'packaging==26.2' && python3 tags/testdata/gen_goldens.py
#
# Generated against packaging==26.2.
#
# Produces the reference ORDERED tag list for a handful of hand-picked
# targets, using packaging.tags directly. Each target here needs no
# host-detection monkeypatching: the platform list is passed explicitly to
# packaging.tags.cpython_tags/compatible_tags, so the result does not depend
# on the machine gen_goldens.py runs on. Linux/macOS targets (Tasks 3/4) DO
# require monkeypatching packaging's glibc/musl/arch detection and are added
# in their own sections of this file.
#
# Output: testdata/<name>.json = ["tag", ...] in the exact order Go's
# tags.Target.Compile().Tags() must produce. tags/golden_test.go reads this
# JSON only; it never imports or shells out to Python.
import json
import pathlib

import packaging.tags as T

HERE = pathlib.Path(__file__).parent


def write(name, tag_iter):
    tags = [str(tag) for tag in tag_iter]
    (HERE / name).write_text(json.dumps(tags, indent=2) + "\n")
    print(f"wrote {name} ({len(tags)} tags)")


# --- windows-amd64: cp311, Windows amd64 -----------------------------------
# The full sys_tags()-equivalent ordering for a declared (not host-detected)
# CPython 3.11 / win_amd64 target: cpython_tags(...) followed by
# compatible_tags(..., interpreter="cp311", ...) over the same platform.
write(
    "windows-amd64.json",
    list(T.cpython_tags(python_version=(3, 11), abis=["cp311"], platforms=["win_amd64"]))
    + list(T.compatible_tags(python_version=(3, 11), interpreter="cp311", platforms=["win_amd64"])),
)

# --- generic-py3: pure-Python (Implementation "py"), same platform ---------
# Exercises the "py" (no compiled ABI) branch of the ordering framework:
# just the compatible tier + universal tail, with no interpreter-specific
# entries (compatible_tags with interpreter=None never emits "<interp>-none-any").
write(
    "generic-py3.json",
    list(T.compatible_tags(python_version=(3, 11), interpreter=None, platforms=["win_amd64"])),
)

# Linux targets (manylinux/musllinux + arch-dependent floor) are added in
# #18632 Task 3.

# macOS targets (11+) are added in #18632 Task 4.

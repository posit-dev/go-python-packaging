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

# --- linux targets: manylinux/musllinux + arch-dependent glibc floor -------
#
# packaging computes manylinux/musllinux platform tags from *detected host
# state* (the running interpreter's glibc/musl version, by way of
# packaging._manylinux._get_glibc_version / packaging._musllinux._get_musl_version),
# not from an explicit parameter. To fake an arbitrary declared target (not
# this machine's), we monkeypatch those two host-detection functions to the
# target's version, and sysconfig.get_platform to the target's arch, then
# drive the real packaging.tags._linux_platforms() generator (the same one
# packaging.tags.platform_tags() uses on an actual Linux host). This is the
# same monkeypatch-the-detector technique packaging's own test suite uses
# (see upstream tests/test_manylinux.py / test_musllinux.py).
#
# Per our Libc-per-Target design, a target is glibc-only or musl-only
# (never both): for a glibc target we additionally force musl detection to
# "not musl" (None), and for a musl target we force glibc detection to
# "undetected" ((-1, -1), packaging's own sentinel for "no glibc here"), so
# _linux_platforms() naturally yields only the one family.
import packaging._manylinux as _manylinux
import packaging._musllinux as _musllinux
import sysconfig


def _linux_platform_list(arch, libc, libc_major, libc_minor):
    sysconfig.get_platform = lambda: f"linux-{arch}"
    if libc == "glibc":
        _manylinux._get_glibc_version = lambda: _manylinux._GLibCVersion(libc_major, libc_minor)
        _musllinux._get_musl_version = lambda executable: None
    elif libc == "musl":
        _musllinux._get_musl_version = lambda executable: _musllinux._MuslVersion(libc_major, libc_minor)
        _manylinux._get_glibc_version = lambda: _manylinux._GLibCVersion(-1, -1)
    else:
        raise ValueError(f"unknown libc {libc!r}")
    platforms = list(T._linux_platforms())
    # packaging.tags._linux_platforms() already yields the bare "linux_<arch>"
    # tag last (after manylinux/musllinux), matching our own "ranked last"
    # design decision. Reorder defensively anyway, so this generator stays
    # correct even if a future packaging release changes that ordering.
    bare = f"linux_{arch}"
    platforms = [p for p in platforms if p != bare] + [p for p in platforms if p == bare]
    return platforms


def write_linux(name, python_version, interpreter, arch, libc, libc_major, libc_minor):
    platforms = _linux_platform_list(arch, libc, libc_major, libc_minor)
    write(
        name,
        list(T.cpython_tags(python_version=python_version, abis=[interpreter], platforms=platforms))
        + list(T.compatible_tags(python_version=python_version, interpreter=interpreter, platforms=platforms)),
    )


# cp311, x86_64, glibc 2.28: walks the full manylinux1(2.5)..manylinux_2_28
# ladder with all three legacy aliases (manylinux1/2010/2014).
write_linux("cp311_glibc228_x86_64.json", (3, 11), "cp311", "x86_64", "glibc", 2, 28)

# cp39, aarch64, glibc 2.17: floors exactly at manylinux_2_17 (+ manylinux2014
# alias only) -- no manylinux_2_5_aarch64, since aarch64's floor is 2.17, not
# x86_64's 2.5.
write_linux("cp39_glibc217_aarch64.json", (3, 9), "cp39", "aarch64", "glibc", 2, 17)

# cp312, x86_64, musl 1.2: musllinux_1_2 down to musllinux_1_0, no manylinux
# tags at all (this target has no glibc).
write_linux("cp312_musl12_x86_64.json", (3, 12), "cp312", "x86_64", "musl", 1, 2)

# macOS targets (11+) are added in #18632 Task 4.

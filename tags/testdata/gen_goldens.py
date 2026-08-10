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


# The ABI and the compatible-tier interpreter are separate inputs: sys_tags
# passes the abiflag-carrying ABI ("cp313t") to cpython_tags but the bare
# "cp" + py_version_nodot ("cp313") to compatible_tags. They coincide for
# ordinary GIL-enabled builds and diverge for free-threaded ones.
def _cp_interp(python_version):
    return f"cp{python_version[0]}{python_version[1]}"


def write_linux(name, python_version, abi, arch, libc, libc_major, libc_minor):
    platforms = _linux_platform_list(arch, libc, libc_major, libc_minor)
    interpreter = _cp_interp(python_version)
    write(
        name,
        list(T.cpython_tags(python_version=python_version, abis=[abi], platforms=platforms))
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

# cp312, x86_64, musl 2.3: a HYPOTHETICAL musl major bump, recorded to pin what
# the reference does with a musl major other than 1. packaging's
# _musllinux.platform_tags uses the detected major verbatim
# ("musllinux_{sys_musl.major}_{minor}") and walks the minor to 0 -- it has no
# notion of a "musllinux_1 only" restriction. No musl 2 exists, so this fixture
# is not reachable from a real host; it exists to keep our generator's handling
# of the major honest rather than guessed.
write_linux("cp312_musl23_x86_64.json", (3, 12), "cp312", "x86_64", "musl", 2, 3)

# --- a non-3 Python major ---------------------------------------------------
#
# Also hypothetical, and also recorded rather than reasoned about. packaging
# gates the stable ABI on a LEXICOGRAPHIC version comparison,
# _abi3_applies -> tuple(python_version) >= (3, 2), so a 4.0 target DOES get
# cp40-abi3-<plat>. The descending abi3 walk is empty (range(-1, 1, -1)), and
# _py_interpreter_range((4, 0)) yields just py40 and py4.
write_linux("cp40_glibc239_x86_64.json", (4, 0), "cp40", "x86_64", "glibc", 2, 39)

# --- a glibc major bump -----------------------------------------------------
#
# Also hypothetical, and measured rather than reasoned about. packaging assumes
# compatibility ACROSS glibc major versions (_manylinux.platform_tags: "We can
# assume compatibility across glibc major versions", citing
# https://sourceware.org/bugzilla/show_bug.cgi?id=24636), so a glibc 3.x target
# claims the whole glibc 2 series below it as well.
#
# To enumerate an older major it needs that major's highest minor version, which
# it takes from _LAST_GLIBC_MINOR -- a defaultdict whose fallback is 50 and whose
# own comment calls it a guess (see tags/generate.go's lastGlibcMinor, which
# quotes it). So a glibc 3.5 target yields manylinux_3_5..3_0 and then
# manylinux_2_50..2_5, with the legacy aliases still interleaved in the major-2
# range only.
#
# Both archs are recorded because the floor differs: x86_64 bottoms out at 2.5
# with all three legacy aliases, every other arch at 2.17 with manylinux2014
# alone.
write_linux("cp312_glibc35_x86_64.json", (3, 12), "cp312", "x86_64", "glibc", 3, 5)
write_linux("cp312_glibc35_aarch64.json", (3, 12), "cp312", "aarch64", "glibc", 3, 5)

# --- macOS targets ----------------------------------------------------------
#
# packaging.tags.mac_platforms((major, minor), arch) is directly parameterized
# by declared (major, minor) and arch -- no host-detection monkeypatching
# needed, unlike linux. Its output is passed through UNFILTERED: as of #18766
# we generate the pre-11 range too, both the legacy "macosx_10_<n>" walk for a
# declared 10.x target and the pre-11 compatibility tail an 11+ target carries
# (each yearly macOS release prior to 11 bumped the minor version under major
# 10, so 10.x is a minor-version walk while 11+ is a major-version walk).
def _mac_platform_list(major, minor, arch):
    return list(T.mac_platforms((major, minor), arch))


def write_macos(name, python_version, abi, major, arch, minor=0):
    platforms = _mac_platform_list(major, minor, arch)
    interpreter = _cp_interp(python_version)
    write(
        name,
        list(T.cpython_tags(python_version=python_version, abis=[abi], platforms=platforms))
        + list(T.compatible_tags(python_version=python_version, interpreter=interpreter, platforms=platforms)),
    )


# cp310, macOS 12, arm64: major walk stops at macosx_12_0/macosx_11_0, each
# with just [arm64, universal2] -- arm64 has no intel/fat* legacy formats --
# then the pre-11 tail, which for a non-x86_64 arch is universal2 only
# (macosx_10_16_universal2 .. macosx_10_4_universal2). Arm64 support arrived in
# macOS 11, so there is no macosx_10_<n>_arm64.
write_macos("cp310_macos12_arm64.json", (3, 10), "cp310", 12, "arm64")

# cp312, macOS 14, x86_64: full x86_64 format list
# ([x86_64, intel, fat64, fat32, universal2, universal]) at each of
# macosx_14_0/13_0/12_0/11_0, then the same full format list across the pre-11
# tail macosx_10_16 .. macosx_10_4.
write_macos("cp312_macos14_x86_64.json", (3, 12), "cp312", 14, "x86_64")

# cp39, macOS 10.15 (Catalina), x86_64: a DECLARED pre-11 target. Walks the
# minor version down (macosx_10_15 .. macosx_10_4) with the full x86_64 format
# list; macosx_10_3 and below yield nothing at all, because
# _mac_binary_formats returns [] for x86_64 below (10, 4). No 11+ major walk
# and no compatibility tail -- both of those blocks are gated on
# version >= (11, 0).
write_macos("cp39_macos1015_x86_64.json", (3, 9), "cp39", 10, "x86_64", minor=15)

# --- free-threaded CPython (PEP 703 / PEP 803) ------------------------------
#
# The free-threaded build's ABI carries a "t" in its abiflags: cp313t. The abi
# is the only input that tells cpython_tags a target is free-threaded --
# _is_threaded_cpython() re-reads it out of abis[0] -- so passing abis=["cp313t"]
# explicitly is sufficient, with no monkeypatching of Py_GIL_DISABLED.
#
# The consequence measured here is NOT just an extra "t": free-threaded builds
# do not support abi3 (_abi3_applies returns False when threading), and instead
# get PEP 803's abi3t. So the stable-ABI slot and the whole descending
# stable-ABI walk switch from abi3 to abi3t.
write_linux("cp313t_glibc235_x86_64.json", (3, 13), "cp313t", "x86_64", "glibc", 2, 35)
write_macos("cp314t_macos15_arm64.json", (3, 14), "cp314t", 15, "arm64")

# --- PyPy / non-CPython implementation ABI ----------------------------------
#
# packaging routes non-CPython interpreters through generic_tags() rather than
# cpython_tags(), then a compatible tier whose "<interp>-none-any" entry is the
# MAJOR-only "pp3" (see sys_tags: interp = "pp3" for interp_name == "pp", not
# "pp310"). generic_tags appends "none" to the ABI list itself.
#
# The ABI spelling comes from _generic_abi()'s EXT_SUFFIX parse: PyPy 7.3 for
# Python 3.10 has EXT_SUFFIX ".pypy310-pp73-<plat>.so", giving abi
# "pypy310_pp73" ("pypy" + python version nodot + "_pp" + pypy version nodot).
def write_pypy(name, python_version, abis, platforms):
    interp = f"pp{python_version[0]}{python_version[1]}"
    write(
        name,
        list(T.generic_tags(interpreter=interp, abis=list(abis), platforms=platforms))
        + list(
            T.compatible_tags(
                python_version=python_version,
                interpreter=f"pp{python_version[0]}",
                platforms=platforms,
            )
        ),
    )


write_pypy(
    "pp310_pypy73_glibc228_x86_64.json",
    (3, 10),
    ["pypy310_pp73"],
    _linux_platform_list("x86_64", "glibc", 2, 28),
)
write_pypy(
    "pp311_pypy73_macos14_arm64.json",
    (3, 11),
    ["pypy311_pp73"],
    _mac_platform_list(14, 0, "arm64"),
)
# A PyPy target whose PyPy-side version is unknown: generic_tags with an empty
# ABI list still yields the "<interp>-none-<plat>" tier (it appends "none"
# itself), just no implementation-specific ABI tag.
write_pypy(
    "pp310_noabi_windows_amd64.json",
    (3, 10),
    [],
    ["win_amd64"],
)

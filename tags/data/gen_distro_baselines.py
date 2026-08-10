# Regenerate the distro baseline table:
#   python3 tags/data/gen_distro_baselines.py
#
# Writes tags/data/distro_baselines.json, which tags/distro.go embeds. Requires
# network access AT AUTHORING TIME ONLY -- the table is compiled into the
# library, and nothing in tags/ reaches the network at runtime.
#
# WHY A GENERATOR AND NOT A HAND-WRITTEN TABLE
#
# A hardcoded list of distro libc versions is exactly the kind of table a
# future maintainer cannot safely update: it goes stale every six months, and
# without a stated source there is no way to tell a typo from a deliberate
# entry. So the version numbers are derived mechanically, from one
# machine-readable source, by rules stated here.
#
# The division of labor is deliberate. Repology supplies the LIBC VERSION -- the
# part that is easy to get wrong and impossible to verify by eye. RELEASES below
# supplies the list of RELEASES -- the part a human can verify and that changes
# only a few times a year. The script refuses to emit anything for a release not
# on that list, and fails if a listed release has no row upstream, so a Repology
# repo rename or a typo is loud rather than silent.
#
# SOURCE
#
#   https://repology.org/api/v1/project/glibc
#   https://repology.org/api/v1/project/musl
#
# Repology's API terms of use (https://repology.org/api) cap request rate at one
# per second and require a User-Agent naming the client's source repository; this
# script makes exactly two requests and sets such a User-Agent. Do not turn it
# into a loop.
#
# Each row is (repo, srcname, version, status). Three filters turn that into one
# version per release:
#
#   1. srcname must be exactly "glibc" / "musl". Repology also reports
#      "compat-glibc" (CentOS 6's 2.5 compatibility library) and
#      "glibc-doc-reference" (a documentation package whose version lags the
#      library by several releases, e.g. 2.42 in a 2.43 Ubuntu). Both would
#      otherwise be indistinguishable from the real answer.
#   2. status must not be "legacy". A repo can carry an older version alongside
#      its current one; "legacy" is Repology's marker for the superseded row.
#   3. the (distro, release) pair must be listed in RELEASES.
#
# After those filters each release must have exactly one version; the script
# fails loudly otherwise rather than silently picking one.
#
# ⚠️ A DISTRO'S libc IS NOT "WHATEVER libc IT PACKAGES". Debian, Ubuntu and
# Fedora all ship a `musl` package, and an earlier version of this script
# happily recorded "ubuntu 22.04 musl 1.2" from it -- which would have claimed a
# glibc distro as a musllinux target. Each release is therefore listed under
# exactly one libc, the one its system C library actually is.
#
# WHAT THE TABLE DOES NOT COVER
#
# Red Hat Enterprise Linux and Amazon Linux 2023 have no public package
# repository for Repology to index. RHEL is represented by its 1:1 rebuilds,
# AlmaLinux and Rocky Linux, which ship the same glibc; see the notes recorded
# in the output file.
import json
import pathlib
import re
import urllib.request

HERE = pathlib.Path(__file__).parent
OUT = HERE / "distro_baselines.json"

API = "https://repology.org/api/v1/project/{project}"
UA = "go-python-packaging distro-baseline generator (+https://github.com/posit-dev/go-python-packaging)"

# Canonical distro name -> Repology repo-name template. "{release}" is the
# release with dots replaced by underscores.
REPO_TEMPLATE = {
    "ubuntu": "ubuntu_{release}",
    "debian": "debian_{release}",
    "centos": "centos_{release}",
    "centos-stream": "centos_stream_{release}",
    "almalinux": "almalinux_{release}",
    "rocky": "rocky_{release}",
    "fedora": "fedora_{release}",
    "amazonlinux": "amazon_{release}",
    "opensuse-leap": "opensuse_leap_{release}",
    "alpine": "alpine_{release}",
}

# The releases to record, per libc. RELEASED, non-rolling releases only:
# a devel series' libc still moves (Repology indexes Debian forky, Fedora
# rawhide and the in-progress Ubuntu interim alike), so a version read off one
# is not a baseline anyone can target.
#
# Ubuntu is LTS-only on purpose: interim releases are supported for nine months
# and nobody sets a compatibility floor at one.
#
# Reviewed as of 2026-08. Adding a release is a one-line change here; the
# version comes from upstream.
RELEASES = {
    "glibc": {
        # CentOS 6 and 7 are the baselines the legacy manylinux2010 and
        # manylinux2014 aliases were defined against, which is why two
        # long-EOL releases are worth keeping.
        "centos": ["6", "7"],
        "centos-stream": ["9", "10"],
        "almalinux": ["8", "9"],
        "rocky": ["8", "9"],
        "debian": ["11", "12", "13"],
        "ubuntu": ["16.04", "18.04", "20.04", "22.04", "24.04", "26.04"],
        "fedora": ["41", "42", "43"],
        "amazonlinux": ["2"],
        "opensuse-leap": ["15.5", "15.6"],
    },
    "musl": {
        # Every currently supported Alpine is musl 1.2, so these rows collapse
        # to one answer. They are still worth recording: the useful question is
        # "can Alpine 3.19 run a musllinux_1_2 wheel", and answering it should
        # not require knowing that.
        "alpine": ["3.17", "3.18", "3.19", "3.20", "3.21", "3.22", "3.23", "3.24"],
    },
}


def fetch(project):
    req = urllib.request.Request(API.format(project=project), headers={"User-Agent": UA})
    with urllib.request.urlopen(req, timeout=60) as resp:
        return json.load(resp)


def libc_version(version):
    """major.minor of a package version: '1.2.4_git20230717' -> (1, 2)."""
    m = re.match(r"(\d+)\.(\d+)", version)
    if not m:
        raise ValueError(f"unparseable libc version {version!r}")
    return int(m.group(1)), int(m.group(2))


def collect(libc):
    wanted = {}  # repo name -> (distro, release)
    for distro, releases in RELEASES[libc].items():
        for release in releases:
            repo = REPO_TEMPLATE[distro].format(release=release.replace(".", "_"))
            wanted[repo] = (distro, release)

    found = {}
    for row in fetch(libc):
        if row.get("srcname") != libc or row.get("status") == "legacy":
            continue
        if row["repo"] not in wanted:
            continue
        found.setdefault(wanted[row["repo"]], set()).add(libc_version(row["version"]))

    missing = sorted(set(wanted.values()) - set(found))
    if missing:
        raise SystemExit(
            f"no upstream {libc} row for these releases (Repology repo renamed, "
            "release dropped from the index, or a typo in RELEASES):\n"
            + "\n".join(f"  {d} {r} (expected repo "
                        f"{REPO_TEMPLATE[d].format(release=r.replace('.', '_'))})"
                        for d, r in missing)
        )

    ambiguous = [(d, r, sorted(v)) for (d, r), v in found.items() if len(v) != 1]
    if ambiguous:
        raise SystemExit(
            f"ambiguous {libc} version after filtering, refusing to guess:\n"
            + "\n".join(f"  {d} {r}: {v}" for d, r, v in ambiguous)
        )

    out = []
    for (distro, release), versions in found.items():
        major, minor = versions.pop()
        out.append(
            {
                "distro": distro,
                "release": release,
                "libc": libc,
                "libcMajor": major,
                "libcMinor": minor,
            }
        )
    return out


def sort_key(entry):
    return (entry["distro"], [int(part) for part in entry["release"].split(".")])


baselines = collect("glibc") + collect("musl")
baselines.sort(key=sort_key)

doc = {
    "source": "https://repology.org/api/v1/project/{glibc,musl}",
    "regenerate": "python3 tags/data/gen_distro_baselines.py",
    "notes": [
        "The libc version a stock release ships, as reported by the "
        "distribution's own package repository via Repology. Released, "
        "non-rolling releases only; see gen_distro_baselines.py for the exact "
        "filters and the reviewed release list.",
        "Red Hat Enterprise Linux has no public package repository to index. "
        "AlmaLinux N and Rocky Linux N are 1:1 rebuilds of RHEL N and ship the "
        "same glibc, so use those rows for RHEL. Amazon Linux 2023 is likewise "
        "unindexed and absent.",
        "A host may deviate from its release's stock libc (backports, HWE, a "
        "hand-built glibc, a container image with a different base). This table "
        "names a baseline; it is not a substitute for knowing the actual target.",
    ],
    "baselines": baselines,
}

OUT.write_text(json.dumps(doc, indent=2) + "\n")
print(f"wrote {OUT.name} ({len(baselines)} baselines)")
for entry in baselines:
    print(
        f"  {entry['distro']:14s} {entry['release']:6s} "
        f"{entry['libc']} {entry['libcMajor']}.{entry['libcMinor']}"
    )

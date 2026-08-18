# Regenerate the SPDX identifier set:
#   python3 license/data/gen_spdx_licenses.py
#
# Writes license/data/spdx_licenses.json, which license/spdx_ids.go embeds.
# Requires network access AT AUTHORING TIME ONLY -- the set is compiled into the
# library, and nothing in license/ reaches the network at runtime.
#
# WHAT THIS FILE IS FOR
#
# The free-form `License:` metadata field is unconstrained text. Deriving SPDX
# ids from it is only safe if every token is checked against a KNOWN identifier
# (see typesFromLicenseText); an unchecked derivation would happily "recognize"
# `3-Clause` or `License` as licenses. That check needs one thing: the set of
# identifiers SPDX actually publishes.
#
# Exception identifiers (the right-hand side of `GPL-3.0-only WITH
# Classpath-exception-2.0`) are collected for the same reason. An exception is
# not a license and never becomes a derived type, but it has to be recognizable:
# `MIT with restrictions` parses as MIT-plus-an-exception, and a derivation that
# dropped the unrecognized `restrictions` would report plain MIT for a package
# that explicitly said it was not granting plain MIT.
#
# SOURCE
#
#   https://raw.githubusercontent.com/spdx/license-list-data/<TAG>/json/licenses.json
#   https://raw.githubusercontent.com/spdx/license-list-data/<TAG>/json/exceptions.json
#
# ⚠️ PIN TO A RELEASED TAG, NEVER `main`. The file on `main` carries an
# unreleased `licenseListVersion` (a commit hash) and can contain identifiers no
# published release has. Consumers validate derived ids against their own copy
# of the SPDX list -- Posit Package Manager's ValidateLicenseType is one -- and
# an id this module emits that the consumer's list does not carry becomes a
# license type that exists in the data but cannot be named in a rule. Tracking
# releases keeps the two able to converge.
#
# FILTERS
#
#   1. isDeprecatedLicenseId must be false. A deprecated id (`GPL-3.0`,
#      `LGPL-2.1`, ...) is exactly the ambiguous spelling this module must NOT
#      resolve: SPDX deprecated `GPL-3.0` because it does not say whether
#      "or later" applies, which is the same reason bare `GPL` is rejected.
#      Consumers reject deprecated ids too, so emitting one would produce an
#      unusable type.
#   2. An id equal (case-insensitively) to the `Unknown` sentinel is dropped, so
#      that a future SPDX addition cannot launder the sentinel into a "detected"
#      type. SPDX has never published such an id; this is a standing guard, and
#      license/spdx_ids.go enforces the same rule when it loads this file.
#
# The ids are written sorted so that a regeneration produces a reviewable diff
# rather than a reshuffle.

import json
import os
import urllib.request

TAG = "v3.28.0"
BASE = f"https://raw.githubusercontent.com/spdx/license-list-data/{TAG}/json"
LICENSES_URL = f"{BASE}/licenses.json"
EXCEPTIONS_URL = f"{BASE}/exceptions.json"

UNKNOWN_SENTINEL = "unknown"

HERE = os.path.dirname(os.path.abspath(__file__))
OUT = os.path.join(HERE, "spdx_licenses.json")


def fetch(url: str) -> dict:
    with urllib.request.urlopen(url) as resp:
        return json.load(resp)


def check_release_version(version: str) -> None:
    if not version[0].isdigit():
        raise SystemExit(
            f"licenseListVersion {version!r} is not a release version -- "
            f"{TAG} does not look like a released tag"
        )


def check_fold(ids: list, what: str) -> None:
    # The Go side folds case to match `mit` as well as `MIT`, so two ids
    # differing only in case would make the fold ambiguous. Fail loudly rather
    # than let one silently win.
    lowered = {}
    for i in ids:
        lowered.setdefault(i.lower(), []).append(i)
    collisions = {k: v for k, v in lowered.items() if len(v) > 1}
    if collisions:
        raise SystemExit(f"case-insensitive {what} collisions: {collisions}")


def main() -> None:
    licenses = fetch(LICENSES_URL)
    exceptions = fetch(EXCEPTIONS_URL)

    version = licenses["licenseListVersion"]
    check_release_version(version)
    check_release_version(exceptions["licenseListVersion"])
    if exceptions["licenseListVersion"] != version:
        raise SystemExit(
            f"license list {version} and exception list "
            f"{exceptions['licenseListVersion']} disagree"
        )

    ids = sorted(
        lic["licenseId"]
        for lic in licenses["licenses"]
        if not lic["isDeprecatedLicenseId"]
        and lic["licenseId"].lower() != UNKNOWN_SENTINEL
    )
    if not ids:
        raise SystemExit("no non-deprecated identifiers found -- upstream shape changed?")
    check_fold(ids, "identifier")

    exception_ids = sorted(
        exc["licenseExceptionId"]
        for exc in exceptions["exceptions"]
        if not exc["isDeprecatedLicenseId"]
    )
    if not exception_ids:
        raise SystemExit("no non-deprecated exceptions found -- upstream shape changed?")
    check_fold(exception_ids, "exception identifier")

    doc = {
        "source": [LICENSES_URL, EXCEPTIONS_URL],
        "regenerate": "python3 license/data/gen_spdx_licenses.py",
        "notes": [
            "Non-deprecated SPDX license identifiers, from the SPDX License List "
            "at the pinned release tag. See gen_spdx_licenses.py for the filters "
            "and for why the tag must be a release, never main.",
            "This is the recognition set for the free-form `License:` field. It is "
            "deliberately identifiers only: this module validates spellings, it does "
            "not render license names or texts.",
            "licenseExceptionIds are the identifiers valid to the right of a WITH. "
            "An exception is never a derived license type; it is listed so that an "
            "UNRECOGNIZED exception can reject the field it appears in.",
        ],
        "licenseListVersion": version,
        "licenseIds": ids,
        "licenseExceptionIds": exception_ids,
    }

    with open(OUT, "w") as f:
        json.dump(doc, f, indent=2)
        f.write("\n")
    print(
        f"wrote {OUT}: {len(ids)} identifiers, {len(exception_ids)} exceptions, "
        f"SPDX license list {version}"
    )


if __name__ == "__main__":
    main()

# go-python-packaging

Go primitives for Python packaging metadata: PEP 440 versions, PEP 508
requirement specifiers and markers, PEP 685 extras, PEP 425/600/656
compatibility tags, wheel/METADATA parsing, and `requirements.txt`.

Part of [RFD 0001 — Native Go PyPI Dependency Resolution](https://github.com/rstudio/package-manager/blob/main/docs/rfds/0001-pypi-native-resolver/README.md).

> **Status: scaffolding.** Packages are populated per RFD 0001 Phase 0–1.

## Packages

| Package | Scope |
|---|---|
| [`version/`](https://github.com/rstudio/go-pep440-version) | PEP 440 versions and specifiers |
| `requirement/` | PEP 508 dependency specifiers |
| `marker/` | PEP 508 environment markers |
| `extras/` | PEP 685 extra-name normalization |
| `reqtxt/` | pip `requirements.txt` format |
| `distribution/` | METADATA / wheel parsing *(added by migration #18630)* |
| `wheelname/` | wheel filename → structured fields |
| `tags/` | PEP 425/600/656 compatibility tags |

## License

Dual-licensed under **Apache-2.0** OR **MIT** at your option. Every source
file carries `SPDX-License-Identifier: Apache-2.0 OR MIT`.

// SPDX-License-Identifier: Apache-2.0 OR MIT

// Package wheelname decomposes a wheel filename into its structured fields.
//
// Parse splits a PEP 427 wheel filename (name-version[-build]-tags.whl) into
// its name, version, optional build tag, and expanded compatibility tags.
// The version field is parsed as-is via the version package; PEP 427's
// filename-safe "_" is not un-escaped to "+", so filenames that encode a
// local version segment (e.g. "torch-2.4.0+cu121-...") must already carry
// the literal "+". CompareBuildTags totally orders PEP 427 build tags,
// treating an absent build tag as ordering before any present tag.
package wheelname

// SPDX-License-Identifier: Apache-2.0 OR MIT

// Package packagename normalizes Python project (distribution) names per
// PEP 503.
//
// Normalize lowercases a project name as Python's str.lower() does and
// collapses any run of "-", "_", or "." into a single "-".
// The result is deterministic and idempotent: normalizing an
// already-normalized name is a no-op.
//
// PEP 685 extra names use the same rule, so packagename.Normalize and
// extras.Normalize always agree; both delegate to one shared implementation.
// Call the one whose name matches what you are normalizing.
package packagename

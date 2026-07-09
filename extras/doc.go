// SPDX-License-Identifier: Apache-2.0 OR MIT

// Package extras normalizes Python extra names per PEP 685.
//
// Normalize lowercases an extra name and collapses any run of "-", "_", or
// "." into a single "-", matching the PEP 503 name-canonicalization rule
// that PEP 685 mandates for extras. The result is deterministic and
// idempotent: normalizing an already-normalized name is a no-op.
package extras

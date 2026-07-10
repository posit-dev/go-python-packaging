// SPDX-License-Identifier: Apache-2.0 OR MIT

// Package reqtxt parses pip's requirements.txt file format, a superset of
// PEP 508 requirement lines with pip-specific directives ("-r"/"-c"
// includes, "-e" editables, VCS/URL/local-path targets, global and
// per-requirement options, "${VAR}" environment expansion, and inline
// comments).
//
// Parse takes requirements.txt content as a string and returns an ordered
// *File of typed Entry values (RequirementEntry, IncludeEntry,
// UnnamedEntry, OptionEntry) reflecting pip's own line-classification and
// dispatch rules. Parse is pure: it does no I/O and does not follow "-r"/
// "-c" includes, so a File's IncludeEntry values name paths the caller
// still has to resolve. Flatten does that: given a root path and a
// caller-supplied opener, it recursively parses and inlines every included
// file's entries in file order, tracking constraint-ness and detecting
// include cycles. WithEnv, a ParseOption accepted by both Parse and
// Flatten, enables "${VAR}" expansion against a caller-supplied lookup.
// File.String() renders a File back to requirements.txt text; reparsing
// that text reproduces the same entries (comments aside).
//
// reqtxt builds on the requirement package for the PEP 508 requirement
// specifiers that make up RequirementEntry.Requirement.
package reqtxt

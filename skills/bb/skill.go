// Package skill embeds the bb agent skill template so it can be accessed
// from anywhere in the binary without requiring the source tree at runtime.
package skill

import _ "embed"

// Content holds the embedded SKILL.md template bytes.
// The template may contain {{BB_VERSION}} which callers replace with the
// running binary's version string.
//
//go:embed SKILL.md
var Content []byte

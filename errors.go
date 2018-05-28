// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package ebcdic_kana

// copied from golang.org/x/text/encoding/internal/internal.go

// A RepertoireError indicates a rune is not in the repertoire of a destination
// encoding. It is associated with an encoding-specific suggested replacement
// byte.
type RepertoireError byte

// Error implements the error interrface.
func (r RepertoireError) Error() string {
	return "encoding: rune not supported by encoding."
}

// Replacement returns the replacement string associated with this error.
func (r RepertoireError) Replacement() byte { return byte(r) }

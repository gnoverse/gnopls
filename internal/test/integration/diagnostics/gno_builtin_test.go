// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package diagnostics

import (
	"testing"

	"github.com/gnolang/gno/gnovm/pkg/gnoenv"
	. "github.com/gnoverse/gnopls/internal/test/integration"
)

// requireGnoRoot skips the test when no gno root can be located, which is what
// supplies the standard library the resolver loads.
func requireGnoRoot(t *testing.T) {
	t.Helper()
	root, err := gnoenv.GuessRootDir()
	if err != nil || root == "" {
		t.Skipf("no gno root available: %v", err)
	}
}

// gnoRealmProgram exercises the parts of the gno universe scope that the Go
// type checker cannot express on its own: the self-referential realm
// interface, its full method set, the cross function, and the crossing forms
// of init and main.
const gnoRealmProgram = `
-- gnomod.toml --
module = "gno.land/r/test/builtins"
gno = "0.9"

-- realm.gno --
package builtins

var owner address

func init(cur realm) {
	owner = cur.Address()
}

// caller returns the realm that crossed into this one.
func caller(rlm realm) address {
	return rlm.Previous().Address()
}

func predicates(rlm realm) bool {
	return rlm.IsCode() && rlm.IsUser() && rlm.IsUserCall() &&
		rlm.IsUserRun() && rlm.IsEphemeral() && rlm.IsCurrent()
}

func describe(rlm realm) string {
	return rlm.PkgPath() + rlm.String()
}

func callee(cur realm) {}

func crosser(cur realm) {
	callee(cross(cur))
}
`

// TestGnoBuiltins_NoDiagnostics is an end-to-end check that a realm package
// using the gno universe scope produces no diagnostics through the LSP.
//
// It also pins the driver-selection path: the test sandbox sets
// GOPACKAGESDRIVER=off in the process environment, so the in-process resolver
// can only be reached if the configured folder environment is what selects it.
func TestGnoBuiltins_NoDiagnostics(t *testing.T) {
	requireGnoRoot(t)
	WithOptions(
		EnvVars{"GOPACKAGESDRIVER": ":memory:"},
	).Run(t, gnoRealmProgram, func(t *testing.T, env *Env) {
		env.OpenFile("realm.gno")
		env.AfterChange(NoDiagnostics(ForFile("realm.gno")))
	})
}

// TestGnoInitDecl_RejectsNonCrossingForms checks that suppressing the
// InvalidInitDecl error for `init(cur realm)` does not also suppress it for
// init declarations that are wrong in gno as well as in Go.
func TestGnoInitDecl_RejectsNonCrossingForms(t *testing.T) {
	requireGnoRoot(t)
	const files = `
-- gnomod.toml --
module = "gno.land/r/test/badinit"
gno = "0.9"

-- realm.gno --
package badinit

var x int

func init(cur realm) { x = 1 }

func init(n int) { x = n }
`
	WithOptions(
		EnvVars{"GOPACKAGESDRIVER": ":memory:"},
	).Run(t, files, func(t *testing.T, env *Env) {
		env.OpenFile("realm.gno")
		env.AfterChange(
			Diagnostics(
				env.AtRegexp("realm.gno", `init\(n int\)`),
				WithMessage("init must have no arguments"),
			),
		)
	})
}

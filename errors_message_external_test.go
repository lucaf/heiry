package heiry_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/lucaf/heiry"
)

func r1Diagnostic(t *testing.T, err error) *heiry.Error {
	t.Helper()
	var diagnostic *heiry.Error
	if !errors.As(err, &diagnostic) {
		t.Fatalf("no *heiry.Error in %v", err)
	}
	return diagnostic
}

func r1RootDefinition() *heiry.Definition[r1DirectRootLeaf] {
	d := heiry.Define[r1DirectRootLeaf]()
	d.Level[r1DirectRoot, r1Read, r1Empty]("").BindNode[r1Read]((*r1DirectRoot)(nil)).Extend[r1DirectRootLeaf]()
	return d
}

func TestR1ErrorMessageContext(t *testing.T) {
	ownership := func() error {
		d := r1RootDefinition()
		x := &r1DirectRootLeaf{}
		return d.Init(x, d.NodeProvider(&r1DirectRoot{}), d.NodeProvider(x))
	}
	unknownOwner := func() error {
		d := r1RootDefinition()
		x := &r1DirectRootLeaf{}
		return d.Init(x, d.NodeProvider(&r1ShownRoot{}), d.NodeProvider(&x.r1DirectRoot), d.NodeProvider(x))
	}
	holder := func() error {
		d := heiry.Define[r1HiddenRootLeaf]()
		d.Level[r1HiddenRoot, r1Read, r1Empty]("").BindNode[r1Read]((*r1HiddenRoot)(nil)).Extend[r1HiddenRootLeaf]()
		return d.Validate()
	}
	missing := func() error {
		d := heiry.Define[r1DirectRootLeaf]()
		d.Level[r1DirectRoot, r1Read, r1Empty]("").Extend[r1DirectRootLeaf]()
		return d.Validate()
	}
	// A second Init of a single-level object names nothing but its target and
	// level 0, which levelKnown keeps for a code that always concerns a level.
	reinitialized := func() error {
		d := heiry.Define[r1DirectRoot]()
		d.Level[r1DirectRoot, r1Read, r1Empty]("").BindNode[r1Read]((*r1DirectRoot)(nil))
		x := &r1DirectRoot{}
		if err := d.Init(x, d.NodeProvider(x)); err != nil {
			return err
		}
		return d.Init(x, d.NodeProvider(x))
	}
	uninitialized := func() (err error) {
		defer func() { err, _ = recover().(error) }()
		(*heiry.Node)(nil).Virtual[r1Read]()
		return nil
	}
	for _, tc := range []struct {
		name string
		err  *heiry.Error
		want string
	}{
		{"Init ownership", r1Diagnostic(t, ownership()),
			`ProviderOwnership: target heiry_test.r1DirectRootLeaf, level 0, path "r1DirectRoot", expected *heiry_test.r1DirectRoot, actual *heiry_test.r1DirectRoot`},
		{"unknown owner", r1Diagnostic(t, unknownOwner()),
			`InvalidProvider: target heiry_test.r1DirectRootLeaf, level -1 (unknown owner), path ".", actual *heiry_test.r1ShownRoot`},
		{"validation with a path", r1Diagnostic(t, holder()),
			`MissingNode: target heiry_test.r1HiddenRootLeaf, level 0, path "r1HiddenRoot.r1hidden", expected heiry.Node`},
		{"target and level 0 only", r1Diagnostic(t, reinitialized()),
			`AlreadyInitialized: target heiry_test.r1DirectRoot, level 0`},
		{"missing method", r1Diagnostic(t, missing()),
			`MissingMethods: target heiry_test.r1DirectRootLeaf, level 1, public Read capability, expected func() int`},
		{"uninitialized scope", r1Diagnostic(t, uninitialized()),
			`Uninitialized: capability heiry_test.r1Read`},
		// An unset scope has no level, even in an error a caller builds.
		{"uninitialized with a level", &heiry.Error{Code: heiry.Uninitialized, Level: 2}, `Uninitialized`},
		{"whole target", r1Diagnostic(t, (*heiry.Plan[r1DirectRootLeaf])(nil).Init(&r1DirectRootLeaf{})),
			`InvalidPlan: target heiry_test.r1DirectRootLeaf`},
		{"joined cause", &heiry.Error{Code: heiry.InvalidPlan, Target: reflect.TypeFor[r1DirectRootLeaf](), Cause: errors.Join(errors.New("first"), errors.New("second\r\nthird"))},
			`InvalidPlan: target heiry_test.r1DirectRootLeaf, cause: first; second; third`},
		{"code only", &heiry.Error{Code: heiry.MissingNode}, `MissingNode`},
		{"nil", nil, `<nil>`},
	} {
		got := tc.err.Error()
		if got != tc.want {
			t.Errorf("%s:\n got %s\nwant %s", tc.name, got, tc.want)
		}
		if strings.ContainsAny(got, "\r\n") || got != tc.err.Error() {
			t.Errorf("%s: message is not one deterministic line: %q", tc.name, got)
		}
	}
}

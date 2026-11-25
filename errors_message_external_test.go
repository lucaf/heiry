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
	// A declaration call after preparation says why it was refused.
	late := func() error {
		d := r1RootDefinition()
		d.MustPrepare()
		d.Extend[r1DirectRootLeaf]()
		x := &r1DirectRootLeaf{}
		return d.Init(x, d.NodeProvider(&x.r1DirectRoot), d.NodeProvider(x))
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
			`MissingMethods: target heiry_test.r1DirectRootLeaf, level 1 (the target; no level has a valid registration), public Read capability, expected func() int`},
		// The form dispatch can raise names the caller's level, not the
		// target's, so it carries no note.
		{"missing method at dispatch", &heiry.Error{Code: heiry.MissingMethods, Capability: reflect.TypeFor[r1Read](), Level: 2},
			`MissingMethods: level 2, capability heiry_test.r1Read`},
		{"declaration after preparation", r1Diagnostic(t, late()),
			`InvalidDeclaration: target heiry_test.r1DirectRootLeaf, level 2, actual heiry_test.r1DirectRootLeaf, cause: the Definition was already prepared and no longer accepts changes`},
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

// Every declaration call refused because the Definition is prepared names
// that reason in its message. The other sticky InvalidDeclaration errors,
// an extension with no level before it and an empty group, name their own.
func TestStickyDeclarationErrorsNameTheirReason(t *testing.T) {
	const reason = "already prepared"
	d := heiry.Define[r1DirectRootLeaf]()
	root := d.Level[r1DirectRoot, r1Read, r1Empty]("").BindNode[r1Read]((*r1DirectRoot)(nil))
	root.Extend[r1DirectRootLeaf]()
	plan := d.MustPrepare()
	root.Bind[r1Read]((*r1DirectRoot)(nil), ".")
	root.BindNode[r1Read]((*r1DirectRoot)(nil))
	root.BindGroup((*r1DirectRoot)(nil), ".", heiry.Of[r1Read]())
	root.BindNodeGroup((*r1DirectRoot)(nil), heiry.Of[r1Read]())
	d.Level[r1DirectRootLeaf, r1Read, r1Empty]("r1DirectRoot")
	d.Extend[r1DirectRootLeaf]()
	d.ExtendWith[r1DirectRootLeaf, r1Read, r1Empty]()
	err := d.Validate()
	if !errors.Is(err, &heiry.Error{Code: heiry.InvalidDeclaration}) {
		t.Fatalf("Validate = %v, want InvalidDeclaration", err)
	}
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok || len(joined.Unwrap()) != 7 {
		t.Fatalf("Validate = %v, want the 7 refused calls", err)
	}
	for _, item := range joined.Unwrap() {
		diagnostic := r1Diagnostic(t, item)
		if diagnostic.Code != heiry.InvalidDeclaration || !strings.Contains(diagnostic.Error(), reason) {
			t.Errorf("refused call after preparation: %v, want InvalidDeclaration naming %q", diagnostic, reason)
		}
	}
	x := &r1DirectRootLeaf{}
	if err := plan.Init(x, plan.NodeProvider(&x.r1DirectRoot), plan.NodeProvider(x)); err != nil {
		t.Fatalf("the earlier Plan stopped working: %v", err)
	}

	// The other sticky InvalidDeclaration errors name their own reasons.
	first := heiry.Define[r1DirectRootLeaf]()
	first.Extend[r1DirectRootLeaf]()
	firstWith := heiry.Define[r1DirectRootLeaf]()
	firstWith.ExtendWith[r1DirectRootLeaf, r1Read, r1Empty]()
	empty := heiry.Define[r1DirectRootLeaf]()
	empty.Level[r1DirectRootLeaf, r1Read, r1Empty]("").BindNodeGroup((*r1DirectRootLeaf)(nil))
	emptyPath := heiry.Define[r1DirectRootLeaf]()
	emptyPath.Level[r1DirectRootLeaf, r1Read, r1Empty]("").BindGroup((*r1DirectRootLeaf)(nil), ".")
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"Extend first", first.Validate(), "cause: the Definition has no level to extend: declare the root with Level"},
		{"ExtendWith first", firstWith.Validate(), "cause: the Definition has no level to extend: declare the root with Level"},
		{"empty BindNodeGroup", empty.Validate(), "cause: the group names no operation: pass at least one Of token"},
		{"empty BindGroup", emptyPath.Validate(), "cause: the group names no operation: pass at least one Of token"},
	} {
		var diagnostic *heiry.Error
		if !errors.As(tc.err, &diagnostic) || diagnostic.Code != heiry.InvalidDeclaration || !strings.Contains(diagnostic.Error(), tc.want) || strings.Contains(tc.err.Error(), reason) {
			t.Errorf("%s: %v, want InvalidDeclaration naming %q and not %q", tc.name, tc.err, tc.want, reason)
		}
	}
}

package matrix

import (
	"errors"
	"fmt"
	"testing"

	"github.com/lucaf/heiry"
)

func codeOf(t *testing.T, err error) heiry.Code {
	t.Helper()
	var diagnostic *heiry.Error
	if !errors.As(err, &diagnostic) {
		t.Fatalf("want a *heiry.Error, got %v", err)
	}
	return diagnostic.Code
}

func newCrown() *crown {
	c := &crown{limit: 7}
	c.left.n, c.right.n, c.own.n = 1, 2, 3
	return c
}

// Two definitions over one leaf type keep their own registrations, slots,
// contracts, and lineage; each has its own generated names.
func TestTwoDefinitionsOverOneLeaf(t *testing.T) {
	a, b := newCrown(), newCrown()
	if err := wideInit(a); err != nil {
		t.Fatal(err)
	}
	if err := narrowInit(b); err != nil {
		t.Fatal(err)
	}

	if got := a.Node.Virtual[Namer]().Name(); got != "crown > branch > root" {
		t.Fatalf("wide Name = %q", got)
	}
	if got := b.Node.Virtual[Namer]().Name(); got != "root" {
		t.Fatalf("narrow Name = %q", got)
	}
	if got := a.Node.Virtual[fmt.Stringer]().String(); got != "root:crown > branch > root" {
		t.Fatalf("wide String = %q", got)
	}

	// Same-type slots are told apart by path, and a provider type reused at
	// two levels resolves to the right field of this object.
	if got := a.Node.Virtual[Counter](); got != Counter(&a.own) {
		t.Fatal("wide Counter is not the leaf's own tally")
	}
	if got := a.own.Node.Parent[Counter](); got != Counter(&a.left) {
		t.Fatal("the parent of the leaf's Counter is not the root's left tally")
	}
	if got := b.Node.Virtual[Counter](); got != Counter(&b.right) {
		t.Fatal("narrow Counter is not the root's right tally")
	}

	// Anonymous generic component and nested component.
	if got := a.Node.Virtual[Tracer](); got != Tracer(&a.journal) {
		t.Fatal("wide Tracer is not the anonymous journal")
	}
	if got := b.Node.Virtual[Tracer](); got != Tracer(&b.shell.inner) {
		t.Fatal("narrow Tracer is not the nested tally")
	}

	// The contract that grows at the leaf exists only in the wide definition.
	if got := a.Node.Virtual[Limiter]().Limit(); got != 7 {
		t.Fatalf("Limit = %d", got)
	}
	func() {
		defer func() {
			var diagnostic *heiry.Error
			if err, ok := recover().(error); !ok || !errors.As(err, &diagnostic) || diagnostic.Code != heiry.UnknownCapability {
				t.Fatalf("narrow Limiter: want UnknownCapability, got %v", err)
			}
		}()
		b.Node.Virtual[Limiter]()
	}()

	// Lineage: one definition's references do not initialize through the other.
	c := newCrown()
	if got := codeOf(t, narrow.Init(c, wideRefs(c)...)); got != heiry.BindingMismatch {
		t.Fatalf("wide references through narrow = %s, want BindingMismatch", got)
	}
	if got := codeOf(t, wideInitPlan(narrow.MustPrepare(), c)); got != heiry.BindingMismatch {
		t.Fatalf("narrow Plan through the wide helper = %s, want BindingMismatch", got)
	}
	if c.Node.Scope() != (heiry.Scope{}) || c.own.Node.Scope() != (heiry.Scope{}) || c.left.Node.Scope() != (heiry.Scope{}) {
		t.Fatal("a rejected initialization installed a scope")
	}
}

// The intermediate level is a target of its own definition.
func TestStandaloneAncestor(t *testing.T) {
	b := &branch{}
	if err := standaloneInitPlan(standalone.MustPrepare().MustPrecompute(), b); err != nil {
		t.Fatal(err)
	}
	if got := b.Node.Virtual[Namer]().Name(); got != "branch > root" {
		t.Fatalf("Name = %q", got)
	}
	if got := codeOf(t, standaloneInit(b)); got != heiry.AlreadyInitialized {
		t.Fatalf("second Init = %s", got)
	}
}

// Every helper passes a nil leaf to the library, for dynamic and precomputed
// Plans, with components present.
func TestNilLeafWithComponents(t *testing.T) {
	if got := codeOf(t, wideInit(nil)); got != heiry.InvalidProvider {
		t.Fatalf("wideInit(nil) = %s", got)
	}
	plan := narrow.MustPrepare()
	for _, p := range []*heiry.Plan[crown]{plan, plan.MustPrecompute()} {
		if got := codeOf(t, narrowInitPlan(p, nil)); got != heiry.InvalidProvider {
			t.Fatalf("narrowInitPlan(plan, nil) = %s", got)
		}
	}
	if got := codeOf(t, narrowInitPlan(nil, newCrown())); got != heiry.InvalidPlan {
		t.Fatalf("nil Plan = %s", got)
	}
}

// The generated reference list is exactly what a handwritten one would be:
// replacing one reference with a pointer into another object is rejected
// with the library's ownership error at the full leaf-relative path.
func TestForeignComponentThroughGeneratedReferences(t *testing.T) {
	c, other := newCrown(), newCrown()
	refs := wideRefs(c)
	for index := range refs {
		forged := append([]heiry.ProviderRef[crown](nil), refs...)
		forged[index] = wideRefs(other)[index]
		err := wide.Init(c, forged...)
		var diagnostic *heiry.Error
		if !errors.As(err, &diagnostic) || diagnostic.Code != heiry.ProviderOwnership {
			t.Fatalf("reference %d from another object: %v", index, err)
		}
	}
	if err := wide.Init(c, refs...); err != nil {
		t.Fatal(err)
	}
}

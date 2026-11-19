package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/lucaf/heiry"
)

func code(t *testing.T, err error) heiry.Code {
	t.Helper()
	var diagnostic *heiry.Error
	if !errors.As(err, &diagnostic) {
		t.Fatalf("want a *heiry.Error, got %v", err)
	}
	return diagnostic.Code
}

func uninitialized(o *tagged) bool {
	empty := heiry.Scope{}
	return o.Node.Scope() == empty && o.limited.Node.Scope() == empty && o.limited.base.Node.Scope() == empty
}

// direct is the handwritten idiom the helpers replace: Init with references
// created by the definition, addresses taken by hand.
func direct(o *tagged) error {
	return hierarchy.Init(o,
		hierarchy.NodeProvider(&o.limited.base),
		hierarchy.NodeProvider(&o.limited),
		hierarchy.NodeProvider(o))
}

func directPlan(plan *heiry.Plan[tagged], o *tagged) error {
	return plan.Init(o,
		hierarchy.NodeProvider(&o.limited.base),
		hierarchy.NodeProvider(&o.limited),
		hierarchy.NodeProvider(o))
}

// same requires two results to be the same diagnostic: code and every
// context field, not only the code.
func same(t *testing.T, name string, got, want error) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Fatalf("%s: helper returned %v, the direct call %v", name, got, want)
	}
	if got == nil {
		t.Logf("%-28s helper and direct call: no error", name)
		return
	}
	var g, w *heiry.Error
	if !errors.As(got, &g) || !errors.As(want, &w) {
		t.Fatalf("%s: not typed errors: %v, %v", name, got, want)
	}
	if *g != *w {
		t.Fatalf("%s:\nhelper %+v\ndirect %+v", name, *g, *w)
	}
	t.Logf("%-28s helper and direct call: %s", name, context(g))
}

// context renders the fields of a diagnostic that carry its meaning.
func context(e *heiry.Error) string {
	return fmt.Sprintf("Code=%s Target=%v Level=%d Path=%q Expected=%v Actual=%v", e.Code, e.Target, e.Level, e.Path, e.Expected, e.Actual)
}

// For every object the handwritten idiom can be called with, the helpers
// return exactly what it returns: the same code and the same context.
func TestHelpersMatchTheHandwrittenIdiom(t *testing.T) {
	plan := hierarchy.MustPrepare()
	fast := plan.MustPrecompute()
	var none *heiry.Plan[tagged]

	// Successful initialization.
	same(t, "valid, lazy", hierarchyInit(&tagged{}), direct(&tagged{}))
	same(t, "valid, dynamic Plan", hierarchyInitPlan(plan, &tagged{}), directPlan(plan, &tagged{}))
	same(t, "valid, precomputed Plan", hierarchyInitPlan(fast, &tagged{}), directPlan(fast, &tagged{}))

	// Repeated initialization, through either entry and either mode.
	a, b := &tagged{}, &tagged{}
	if err := hierarchyInit(a); err != nil {
		t.Fatal(err)
	}
	if err := direct(b); err != nil {
		t.Fatal(err)
	}
	same(t, "repeated, lazy", hierarchyInit(a), direct(b))
	same(t, "repeated, precomputed Plan", hierarchyInitPlan(fast, a), directPlan(fast, b))

	// An object of which only an inner level was initialized before.
	a, b = &tagged{}, &tagged{}
	inner := heiry.Define[limited]().
		Level[base, Public, Private]("").
		Bind[ProcessOp]((*base)(nil), ".").
		Bind[DescribeOp]((*base)(nil), ".").
		Bind[ValidateOp]((*base)(nil), ".").
		Bind[TransformOp]((*base)(nil), ".").
		Level[limited, Public, Private]("base")
	for _, o := range []*tagged{a, b} {
		if err := inner.Init(&o.limited, inner.NodeProvider(&o.limited.base), inner.NodeProvider(&o.limited)); err != nil {
			t.Fatal(err)
		}
	}
	same(t, "partly initialized", hierarchyInit(a), direct(b))

	// Unusable Plans, with a valid and with an initialized object.
	same(t, "nil Plan", hierarchyInitPlan(none, &tagged{}), directPlan(none, &tagged{}))
	same(t, "unprepared Plan", hierarchyInitPlan(&heiry.Plan[tagged]{}, &tagged{}), directPlan(&heiry.Plan[tagged]{}, &tagged{}))
	same(t, "nil Plan, initialized object", hierarchyInitPlan(none, a), directPlan(none, b))
}

// The handwritten idiom cannot be called with a nil object: it dereferences
// it while taking the addresses. Init itself defines what a nil object is
// when the references are well formed, and the helpers return exactly that.
func TestNilObjectIsReportedAsInitReportsIt(t *testing.T) {
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("the handwritten idiom did not panic on a nil object")
			}
		}()
		_ = direct(nil)
	}()

	// Well-formed references for a nil object can only point into another
	// object. This is what Init returns for them.
	other := &tagged{}
	defined := hierarchy.Init(nil,
		hierarchy.NodeProvider(&other.limited.base),
		hierarchy.NodeProvider(&other.limited),
		hierarchy.NodeProvider(other))
	var want *heiry.Error
	if !errors.As(defined, &want) || want.Code != heiry.InvalidProvider || want.Path != "" || want.Expected != nil || want.Actual != nil {
		t.Fatalf("Init with a nil object and well-formed references = %+v", defined)
	}

	// Two other ways a helper could hand a nil object to Init, recorded for
	// comparison. Neither gives the diagnostic above.
	var typedNil, noRefs *heiry.Error
	errors.As(hierarchy.Init(nil, hierarchy.NodeProvider((*base)(nil)), hierarchy.NodeProvider((*limited)(nil)), hierarchy.NodeProvider((*tagged)(nil))), &typedNil)
	errors.As(hierarchy.Init(nil), &noRefs)
	if typedNil == nil || noRefs == nil || *typedNil == *want || *noRefs == *want {
		t.Fatalf("alternatives: %+v, %+v", typedNil, noRefs)
	}
	t.Logf("%-28s %s", "Init, well-formed refs", context(want))
	t.Logf("%-28s %s", "Init, typed-nil refs", context(typedNil))
	t.Logf("%-28s %s", "Init, no refs", context(noRefs))

	plan := hierarchy.MustPrepare()
	same(t, "nil object, lazy", hierarchyInit(nil), defined)
	same(t, "nil object, dynamic Plan", hierarchyInitPlan(plan, nil), defined)
	same(t, "nil object, precomputed Plan", hierarchyInitPlan(plan.MustPrecompute(), nil), defined)
	if !uninitialized(other) {
		t.Fatal("a scope was installed")
	}
}

// An unusable Plan is reported before anything about the object, exactly as
// Plan.Init reports it, and the object stays untouched.
func TestInvalidPlanTakesPrecedence(t *testing.T) {
	var none *heiry.Plan[tagged]
	if got := code(t, hierarchyInitPlan(none, nil)); got != heiry.InvalidPlan {
		t.Fatalf("nil plan, nil leaf = %s, want InvalidPlan", got)
	}
	o := &tagged{}
	if got := code(t, hierarchyInitPlan(none, o)); got != heiry.InvalidPlan {
		t.Fatalf("nil plan = %s, want InvalidPlan", got)
	}
	if got := code(t, hierarchyInitPlan(&heiry.Plan[tagged]{}, o)); got != heiry.InvalidPlan {
		t.Fatalf("unprepared plan = %s, want InvalidPlan", got)
	}
	if !uninitialized(o) {
		t.Fatal("a rejected initialization installed a scope")
	}
	if err := hierarchyInit(o); err != nil {
		t.Fatalf("the object is still usable: %v", err)
	}
}

// One standalone Plan, dynamic or precomputed, initializes independent
// objects, and a second initialization is rejected without replacement.
func TestStandalonePlansAndIndependentObjects(t *testing.T) {
	plan := hierarchy.MustPrepare()
	fast := plan.MustPrecompute()
	objects := map[string]*tagged{"lazy": {tag: "a"}, "dynamic": {tag: "b"}, "precomputed": {tag: "c"}}
	if err := hierarchyInit(objects["lazy"]); err != nil {
		t.Fatal(err)
	}
	if err := hierarchyInitPlan(plan, objects["dynamic"]); err != nil {
		t.Fatal(err)
	}
	if err := hierarchyInitPlan(fast, objects["precomputed"]); err != nil {
		t.Fatal(err)
	}
	for name, o := range objects {
		o.maxLen = 12
		p := &pipeline{state: o}
		if got := p.Describe(); got != "tagged > base" {
			t.Fatalf("%s: Describe = %q", name, got)
		}
		want := "[" + o.tag + "] HELLO"
		if got, err := p.Process("  hello  "); err != nil || got != want {
			t.Fatalf("%s: Process = %q, %v, want %q", name, got, err, want)
		}
		if _, err := p.Process("this message is too long"); err == nil {
			t.Fatalf("%s: the limited level's Validate was not selected", name)
		}
		if p.Processed() != 1 {
			t.Fatalf("%s: processed = %d, want 1", name, p.Processed())
		}
		// The parent of the leaf's ValidateOp lookup is the embedded limited
		// value of this same object.
		if got := o.Node.Virtual[ValidateOp](); got != ValidateOp(&o.limited) {
			t.Fatalf("%s: virtual ValidateOp is not this object's limited level", name)
		}
		if got := code(t, hierarchyInit(o)); got != heiry.AlreadyInitialized {
			t.Fatalf("%s: second Init = %s, want AlreadyInitialized", name, got)
		}
		if got := code(t, hierarchyInitPlan(fast, o)); got != heiry.AlreadyInitialized {
			t.Fatalf("%s: second Init through a Plan = %s, want AlreadyInitialized", name, got)
		}
	}
}

// The helpers belong to one definition. A Plan prepared from another
// definition of the same leaf type is rejected through them even when both
// definitions have exactly the same slots: lineage is preserved.
func TestForeignPlanIsRejected(t *testing.T) {
	other := heiry.Define[tagged]().
		Level[base, Public, Private]("").
		Bind[ProcessOp]((*base)(nil), ".").
		Bind[DescribeOp]((*base)(nil), ".").
		Bind[ValidateOp]((*base)(nil), ".").
		Bind[TransformOp]((*base)(nil), ".").
		Level[limited, Public, Private]("base").
		Level[tagged, Public, Private]("limited").
		MustPrepare()
	o := &tagged{}
	if got := code(t, other.Init(o, hierarchyRefs(o)...)); got != heiry.BindingMismatch {
		t.Fatalf("foreign references = %s, want BindingMismatch", got)
	}
	for name, plan := range map[string]*heiry.Plan[tagged]{"another definition's dynamic Plan": other, "another definition's precomputed Plan": other.MustPrecompute()} {
		got := hierarchyInitPlan(plan, o)
		if code(t, got) != heiry.BindingMismatch {
			t.Fatalf("%s through the helper = %s, want BindingMismatch", name, code(t, got))
		}
		// The same code and context as the direct call with this
		// definition's references.
		same(t, name, got, plan.Init(o, hierarchyRefs(o)...))
	}
	if !uninitialized(o) {
		t.Fatal("a rejected initialization installed a scope")
	}
	// The other definition's own references are accepted.
	if err := other.Init(o, other.NodeProvider(&o.limited.base), other.NodeProvider(&o.limited), other.NodeProvider(o)); err != nil {
		t.Fatal(err)
	}
	if got := (&pipeline{state: o}).Describe(); got != "base" {
		t.Fatalf("the other definition's dispatch = %q, want base", got)
	}
}

// Concurrent first use through the generated helper relies on the library's
// own coordination: the helper adds no shared state. Run with -race.
func TestConcurrentFirstUseThroughHelpers(t *testing.T) {
	// A definition of its own, declared exactly as the generated one, so the
	// first preparation happens inside this test.
	fresh := heiry.Define[tagged]().
		Level[base, Public, Private]("").
		Bind[ProcessOp]((*base)(nil), ".").
		Bind[DescribeOp]((*base)(nil), ".").
		Bind[ValidateOp]((*base)(nil), ".").
		Bind[TransformOp]((*base)(nil), ".").
		Level[limited, Public, Private]("base").
		Bind[ValidateOp]((*limited)(nil), ".").
		Level[tagged, Public, Private]("limited").
		Bind[DescribeOp]((*tagged)(nil), ".")
	const workers = 16
	objects := make([]*tagged, workers)
	errs := make(chan error, 2*workers)
	start := make(chan struct{})
	for index := range objects {
		objects[index] = &tagged{maxLen: 12}
		mine := &tagged{maxLen: 12}
		go func() {
			<-start
			// Lazy first use of a separate definition.
			plan, err := fresh.Prepare()
			if err != nil {
				errs <- err
				return
			}
			errs <- plan.Init(mine, plan.NodeProvider(&mine.limited.base), plan.NodeProvider(&mine.limited), plan.NodeProvider(mine))
		}()
		go func() {
			<-start
			// The generated definition and helper.
			errs <- hierarchyInit(objects[index])
		}()
	}
	close(start)
	for range 2 * workers {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	for _, o := range objects {
		if got := (&pipeline{state: o}).Describe(); got != "tagged > base" {
			t.Fatalf("Describe = %q", got)
		}
	}
}

// hierarchyInitPlan creates its references with the definition, so that a
// Plan of another definition is rejected. Each reference creation takes the
// definition's lock, which references created by the Plan do not. This
// measures what that costs per initialized object, alone and with every
// processor initializing objects at once. The allocation of the object is
// included in both.
func BenchmarkInitPlanReferences(b *testing.B) {
	plan := hierarchy.MustPrepare().MustPrecompute()
	viaDefinition := func() {
		if err := hierarchyInitPlan(plan, &tagged{}); err != nil {
			b.Fatal(err)
		}
	}
	viaPlan := func() {
		o := &tagged{}
		if err := plan.Init(o, plan.NodeProvider(&o.limited.base), plan.NodeProvider(&o.limited), plan.NodeProvider(o)); err != nil {
			b.Fatal(err)
		}
	}
	for name, init := range map[string]func(){"definition references": viaDefinition, "plan references": viaPlan} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				init()
			}
		})
		b.Run(name+", parallel", func(b *testing.B) {
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					init()
				}
			})
		})
	}
}

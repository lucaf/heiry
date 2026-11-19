package main

import (
	"errors"
	"testing"

	"github.com/lucaf/heiry"
)

// Each private operation is checked on its own. A wrapper that failed to
// implement the whole private contract could still expose one of them.
func TestWrapperExposesNoPrivateOperation(t *testing.T) {
	p, err := NewPipeline("audit", 12)
	if err != nil {
		t.Fatal(err)
	}
	var consumer any = p
	if _, ok := consumer.(ValidateOp); ok {
		t.Error("the wrapper exposes Validate")
	}
	if _, ok := consumer.(TransformOp); ok {
		t.Error("the wrapper exposes Transform")
	}
	// The public operations are there, through their generated interfaces.
	if _, ok := consumer.(ProcessOp); !ok {
		t.Error("the wrapper lacks Process")
	}
	if _, ok := consumer.(DescribeOp); !ok {
		t.Error("the wrapper lacks Describe")
	}
}

// The generated operation interfaces are the registered capabilities: both
// resolution modes select the same providers of this object, and a parent
// call starts below the level that makes it.
func TestGeneratedOperationsInBothModes(t *testing.T) {
	plan := hierarchy.MustPrepare()
	for name, selected := range map[string]*heiry.Plan[tagged]{"dynamic": plan, "precomputed": plan.MustPrecompute()} {
		o := &tagged{tag: name}
		o.maxLen = 12
		if err := hierarchyInitPlan(selected, o); err != nil {
			t.Fatal(err)
		}
		if got := o.Node.Virtual[ValidateOp](); got != ValidateOp(&o.limited) {
			t.Fatalf("%s: virtual ValidateOp is not this object's limited level", name)
		}
		if got := o.limited.Node.Parent[ValidateOp](); got != ValidateOp(&o.limited.base) {
			t.Fatalf("%s: parent ValidateOp from limited is not this object's base level", name)
		}
		if got := o.Node.Parent[ProcessOp](); got != ProcessOp(&o.limited.base) {
			t.Fatalf("%s: parent ProcessOp from the leaf does not skip limited", name)
		}
		p := &pipeline{state: o}
		if got, err := p.Process("  hello  "); err != nil || got != "["+name+"] HELLO" {
			t.Fatalf("%s: Process = %q, %v", name, got, err)
		}
		if p.Processed() != 1 {
			t.Fatalf("%s: processed = %d", name, p.Processed())
		}
		var diagnostic *heiry.Error
		if err := hierarchyInit(o); !errors.As(err, &diagnostic) || diagnostic.Code != heiry.AlreadyInitialized {
			t.Fatalf("%s: second Init = %v", name, err)
		}
	}
}

// A nil object is reported as Init reports one, and a nil Plan first.
func TestNilInputs(t *testing.T) {
	other := &tagged{}
	var want, got *heiry.Error
	if err := hierarchy.Init(nil, hierarchyRefs(other)...); !errors.As(err, &want) {
		t.Fatal(err)
	}
	if err := hierarchyInit(nil); !errors.As(err, &got) || *got != *want {
		t.Fatalf("hierarchyInit(nil) = %+v, want %+v", got, want)
	}
	if err := hierarchyInitPlan(nil, nil); !errors.As(err, &got) || got.Code != heiry.InvalidPlan {
		t.Fatalf("nil Plan and nil object = %v", err)
	}
}

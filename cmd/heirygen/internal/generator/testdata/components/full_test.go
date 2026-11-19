package main

import (
	"errors"
	"testing"

	"github.com/lucaf/heiry"
)

// The private operation is checked individually, and the public ones are
// present through their generated interfaces.
func TestWrapperExposesNoPrivateOperation(t *testing.T) {
	repo, err := NewRepository()
	if err != nil {
		t.Fatal(err)
	}
	var consumer any = repo
	if _, ok := consumer.(RecordOp); ok {
		t.Error("the wrapper exposes Record")
	}
	if _, ok := consumer.(GetOp); !ok {
		t.Error("the wrapper lacks Get")
	}
	if _, ok := consumer.(PutOp); !ok {
		t.Error("the wrapper lacks Put")
	}
}

// One Plan and its precomputed conversion initialize independent objects;
// a cached value is replaced after an update in both modes, and component
// parent calls start below the owning level.
func TestGeneratedOperationsInBothModes(t *testing.T) {
	plan := hierarchy.MustPrepare()
	for name, selected := range map[string]*heiry.Plan[cachedRepository]{"dynamic": plan, "precomputed": plan.MustPrecompute()} {
		r := newObject()
		if err := hierarchyInitPlan(selected, r); err != nil {
			t.Fatal(err)
		}
		repo := &wrapper{state: r}
		repo.Put("color", name)
		if got, ok := repo.Get("color"); got != name || !ok {
			t.Fatalf("%s: first read = %q %t", name, got, ok)
		}
		repo.Put("color", "red")
		if got, ok := repo.Get("color"); got != "red" || !ok {
			t.Fatalf("%s: read after update = %q %t", name, got, ok)
		}
		if got := r.cache.Node.Virtual[RecordOp](); got != RecordOp(&r.stamper) {
			t.Fatalf("%s: virtual RecordOp is not this object's stamper", name)
		}
		if got := r.cache.Node.Parent[RecordOp](); got != RecordOp(&r.repository.log) {
			t.Fatalf("%s: parent RecordOp is not this object's log", name)
		}
		if got := r.cache.Node.Parent[GetOp](); got != GetOp(&r.repository.store) {
			t.Fatalf("%s: parent GetOp is not this object's store", name)
		}
		if len(repo.Events()) != 4 {
			t.Fatalf("%s: events = %q", name, repo.Events())
		}
		var diagnostic *heiry.Error
		if err := hierarchyInit(r); !errors.As(err, &diagnostic) || diagnostic.Code != heiry.AlreadyInitialized {
			t.Fatalf("%s: second Init = %v", name, err)
		}
	}
}

// A reference into another object is rejected with the library's ownership
// error, and a nil object as Init reports one.
func TestOwnershipAndNilObject(t *testing.T) {
	r, other := newObject(), newObject()
	for index := range hierarchyRefs(r) {
		refs := hierarchyRefs(r)
		refs[index] = hierarchyRefs(other)[index]
		var diagnostic *heiry.Error
		if err := hierarchy.Init(r, refs...); !errors.As(err, &diagnostic) || diagnostic.Code != heiry.ProviderOwnership {
			t.Fatalf("reference %d from another object: %v", index, err)
		}
	}
	if r.Node.Scope() != (heiry.Scope{}) || r.cache.Node.Scope() != (heiry.Scope{}) {
		t.Fatal("a rejected initialization installed a scope")
	}
	var want, got *heiry.Error
	if err := hierarchy.Init(nil, hierarchyRefs(other)...); !errors.As(err, &want) {
		t.Fatal(err)
	}
	if err := hierarchyInit(nil); !errors.As(err, &got) || *got != *want {
		t.Fatalf("hierarchyInit(nil) = %+v, want %+v", got, want)
	}
}

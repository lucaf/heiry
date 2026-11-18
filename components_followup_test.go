package heiry

import (
	"errors"
	"reflect"
	"testing"
)

func m8RequireDiagnostics(t *testing.T, err error, codes ...Code) []*Error {
	t.Helper()
	for _, code := range codes {
		if !errors.Is(err, &Error{Code: code}) {
			t.Fatalf("missing %s: %v", code, err)
		}
	}
	return m8Diagnostics(err)
}

func TestM8DuplicateInvalidThenValidCoverageIsOrderIndependent(t *testing.T) {
	for _, tc := range []struct {
		name    string
		invalid func(*LevelHandle[m8TwinLeaf])
		code    Code
	}{
		{"invalid path", func(l *LevelHandle[m8TwinLeaf]) { l.Bind[m8Read]((*m8Reader)(nil), "Absent") }, InvalidProvider},
		{"wrong provider type", func(l *LevelHandle[m8TwinLeaf]) { l.BindGroup((*m8Writer)(nil), "First", Of[m8Read]()) }, InvalidProvider},
	} {
		for _, invalidFirst := range []bool{true, false} {
			t.Run(tc.name+map[bool]string{true: "/invalid-first", false: "/valid-first"}[invalidFirst], func(t *testing.T) {
				d := Define[m8TwinLeaf]()
				l := d.Level[m8TwinLeaf, m8TwinPublic, m8Empty]("")
				if invalidFirst {
					tc.invalid(l)
				}
				l.Bind[m8Read]((*m8Reader)(nil), "First")
				if !invalidFirst {
					tc.invalid(l)
				}
				err := d.Validate()
				items := m8RequireDiagnostics(t, err, tc.code)
				m8RequireDiagnostics(t, err, DuplicateBinding, MissingMethods)
				for _, item := range items {
					if item.Code == MissingMethods && item.Name == "Read" {
						t.Fatalf("valid Read lost coverage: %v", err)
					}
				}
			})
		}
	}
}

func TestM8DuplicateWithInvalidInterfaceKeepsValidCoverage(t *testing.T) {
	for _, invalidFirst := range []bool{true, false} {
		d := Define[m8Leaf]()
		l := d.Level[m8Leaf, m8Public, m8Empty]("")
		invalid := func() { l.BindGroup((*m8Writer)(nil), "writer", Of[m8Read]()) }
		valid := func() { l.Bind[m8Read]((*m8Reader)(nil), "reader") }
		if invalidFirst {
			invalid()
		}
		valid()
		if !invalidFirst {
			invalid()
		}
		err := d.Validate()
		items := m8RequireDiagnostics(t, err, SignatureMismatch, DuplicateBinding, MissingMethods)
		for _, item := range items {
			if item.Code == MissingMethods && item.Name != "Write" {
				t.Fatalf("valid Read lost coverage, invalidFirst=%t: %v", invalidFirst, err)
			}
		}
	}
}

func TestM8DuplicateForbiddenOwnershipKeepsValidCoverage(t *testing.T) {
	for _, invalidFirst := range []bool{true, false} {
		d := Define[m8Child]()
		d.Level[m8Parent, m8Empty, m8Empty]("")
		l := d.Level[m8Child, m8Read, m8Empty]("m8Parent")
		invalid := func() { l.Bind[m8Read]((*m8Reader)(nil), "m8Parent.Part") }
		valid := func() { l.BindGroup((*m8Reader)(nil), "Own", Of[m8Read]()) }
		if invalidFirst {
			invalid()
		}
		valid()
		if !invalidFirst {
			invalid()
		}
		err := d.Validate()
		items := m8RequireDiagnostics(t, err, ProviderOwnership, DuplicateBinding)
		for _, item := range items {
			if item.Code == MissingMethods {
				t.Fatalf("valid child Read lost coverage, invalidFirst=%t: %v", invalidFirst, err)
			}
		}
	}
}

func TestM8UnresolvableSlotIsReportedOnceAndMissingMethodsRemain(t *testing.T) {
	d := Define[m8TwinLeaf]()
	d.Level[m8TwinLeaf, m8TwinPublic, m8Empty]("").
		BindGroup((*m8Reader)(nil), "Absent", Of[m8Read](), Of[m8Count]()).
		Bind[m8Audit]((*m8Reader)(nil), "Nested.Second")
	err := d.Validate()
	items := m8RequireDiagnostics(t, err, InvalidProvider, MissingMethods)
	var slotFailures int
	missing := map[string]bool{}
	for _, item := range items {
		if item.Code == InvalidProvider && item.Path == "Absent" {
			slotFailures++
		}
		if item.Code == MissingMethods {
			missing[item.Name] = true
		}
	}
	if slotFailures != 1 || !missing["Read"] || !missing["Count"] || missing["Audit"] {
		t.Fatalf("shared slot diagnostics = %v", err)
	}
}

func TestM8ComponentDeclarationDiagnosticsUseOwnedPath(t *testing.T) {
	d := Define[m8StepLeaf]()
	root := d.Level[m8StepRoot, m8StepPublic, m8StepPrivate]("")
	d.Level[m8StepLeaf, m8StepPublic, m8StepPrivate]("m8StepRoot")
	root.BindGroup((*m8RootStepper)(nil), "Stepper")
	root.Bind[m8Step](&m8RootStepper{}, "Stepper")
	root.BindGroup(&m8RootStepper{}, "Stepper", Of[m8Step]())
	items := m8RequireDiagnostics(t, d.Validate(), InvalidDeclaration, InvalidProvider)
	var invalidCount int
	for _, item := range items {
		if item.Code == InvalidDeclaration || item.Code == InvalidProvider {
			if item.Path != "m8StepRoot.Stepper" || item.Expected != nil {
				t.Fatalf("component diagnostic: %+v", item)
			}
			if item.Code == InvalidProvider {
				invalidCount++
				if item.Actual != reflect.TypeFor[*m8RootStepper]() {
					t.Fatalf("component actual type: %+v", item)
				}
			}
		}
	}
	if invalidCount != 2 {
		t.Fatalf("Bind and BindGroup invalid provider count = %d", invalidCount)
	}
	// A saved handle remains live after preparation, but its edit is rejected.
	frozen := Define[m8StepLeaf]()
	frozenRoot := frozen.Level[m8StepRoot, m8StepPublic, m8StepPrivate]("")
	frozenRoot.Bind[m8Step]((*m8RootStepper)(nil), "Stepper").Bind[m8Secret]((*m8RootSecret)(nil), "Hidden")
	frozen.Level[m8StepLeaf, m8StepPublic, m8StepPrivate]("m8StepRoot").Bind[m8Step]((*m8StepLeaf)(nil), ".")
	frozen.MustPrepare()
	frozenRoot.BindGroup((*m8RootStepper)(nil), "Stepper", Of[m8Step]())
	frozenRoot.Bind[m8Step]((*m8RootStepper)(nil), "Stepper")
	for _, item := range m8RequireDiagnostics(t, frozen.Validate(), InvalidDeclaration) {
		if item.Code == InvalidDeclaration && item.Path != "m8StepRoot.Stepper" {
			t.Fatalf("frozen component diagnostic: %+v", item)
		}
	}
}

func TestM8NonLeafContractShapePaths(t *testing.T) {
	tests := []struct {
		name   string
		code   Code
		middle func(*Definition[m8HierarchyLeaf], string)
	}{
		{"extension", ContractExtension, func(d *Definition[m8HierarchyLeaf], parent string) {
			d.Level[m8HierarchyMiddle, m8Empty, m8Empty](parent)
		}},
		{"collision", ContractCollision, func(d *Definition[m8HierarchyLeaf], parent string) {
			d.Level[m8HierarchyMiddle, m8Read, m8Read](parent)
		}},
		{"invalid contract", InvalidContract, func(d *Definition[m8HierarchyLeaf], parent string) {
			d.Level[m8HierarchyMiddle, m8Read, int](parent)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := Define[m8HierarchyLeaf]()
			d.Level[m8HierarchyRoot, m8Read, m8Empty]("")
			tc.middle(d, "m8HierarchyRoot")
			d.Level[m8HierarchyLeaf, m8Read, m8Empty]("m8HierarchyMiddle")
			err := d.Validate()
			if !errors.Is(err, &Error{Code: tc.code}) {
				t.Fatalf("missing %s: %v", tc.code, err)
			}
			var diagnostic *Error
			if !errors.As(err, &diagnostic) || diagnostic.Code != tc.code || diagnostic.Path != "m8HierarchyMiddle" || diagnostic.Level != 1 || diagnostic.Target != reflect.TypeFor[m8HierarchyLeaf]() {
				t.Fatalf("non-leaf %s diagnostic = %+v", tc.code, diagnostic)
			}
		})
	}
}

func TestM8InvalidParentEdgeDoesNotInventContractPath(t *testing.T) {
	d := Define[m8HierarchyLeaf]()
	d.Level[m8HierarchyRoot, m8Read, m8Empty]("")
	d.Level[m8HierarchyMiddle, m8Read, m8Read]("m8HierarchyRoot")
	d.Level[m8HierarchyLeaf, m8Read, m8Empty]("wrongMiddle")
	err := d.Validate()
	if !errors.Is(err, &Error{Code: InvalidHierarchy}) || !errors.Is(err, &Error{Code: ContractCollision}) {
		t.Fatalf("structure and contract errors = %v", err)
	}
	for _, item := range m8Diagnostics(err) {
		if item.Code == ContractCollision {
			if item.Path != "" || item.Level != 1 || item.Target != reflect.TypeFor[m8HierarchyLeaf]() {
				t.Fatalf("invented contract prefix: %+v", item)
			}
			return
		}
	}
	t.Fatal("missing individual collision diagnostic")
}

func TestM8RejectedPostFreezeLevelKeepsParentPathAndPlan(t *testing.T) {
	d := m8HierarchyDefinition()
	plan := d.MustPrepare()
	rejected := d.Level[m8HierarchyRoot, m8Read, m8Empty]("suppliedParent")
	before := len(d.recorded)
	if rejected.End() != d || !rejected.rejected {
		t.Fatal("rejected handle did not remain inert and attached")
	}
	rejected.Bind[m8Read]((*m8Reader)(nil), "First").BindGroup((*m8Reader)(nil), "First", Of[m8Read]())
	if len(d.recorded) != before || len(d.levels) != 3 || d.plan != plan {
		t.Fatal("rejected handle modified the prepared definition")
	}
	err := d.Validate()
	if !errors.Is(err, &Error{Code: InvalidDeclaration}) {
		t.Fatalf("missing frozen-edit error: %v", err)
	}
	var diagnostic *Error
	if !errors.As(err, &diagnostic) || diagnostic.Path != "suppliedParent" || diagnostic.Level != 3 || diagnostic.Target != reflect.TypeFor[m8HierarchyLeaf]() {
		t.Fatalf("post-freeze parent-field diagnostic = %+v", diagnostic)
	}
	leaf := &m8HierarchyLeaf{}
	if err := plan.Init(leaf, m8HierarchyRefs(plan, leaf)...); err != nil || Method[m8Read](leaf.Node.Scope()) != &leaf.m8HierarchyMiddle.Second {
		t.Fatalf("retained Plan unusable: %v", err)
	}
}

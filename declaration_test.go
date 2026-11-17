package heiry

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestM4EmptyDefinitionAndExactLeaf(t *testing.T) {
	definition := Define[stage3C]()
	var diagnostic *Error
	if err := definition.Validate(); !errors.As(err, &diagnostic) || diagnostic.Code != InvalidHierarchy || diagnostic.Target != reflect.TypeFor[stage3C]() {
		t.Fatalf("empty definition error = %+v", diagnostic)
	}
	definition.Level[stage3A, contractEmpty, contractEmpty]("").End()
	if err := definition.Validate(); !errors.As(err, &diagnostic) || diagnostic.Code != InvalidHierarchy || diagnostic.Expected != reflect.TypeFor[stage3C]() || diagnostic.Actual != reflect.TypeFor[stage3A]() {
		t.Fatalf("leaf mismatch = %+v", diagnostic)
	}
	root := Define[stage3A]().Level[stage3A, contractEmpty, contractEmpty]("").End()
	if err := root.Validate(); err != nil {
		t.Fatalf("valid root definition: %v", err)
	}
}

func TestM4TypedBindingsStableHandleAndNonfreezingValidation(t *testing.T) {
	definition := Define[stage3C]()
	root := definition.Level[stage3A, stage5Public, contractEmpty]("")
	root.Bind[stage3Read]((*stage3A)(nil), ".").
		Bind[stage3Add]((*stage3A)(nil), ".")
	definition = root.End().
		Level[stage3B, stage5Public, contractEmpty]("stage3A").End().
		Level[stage3C, stage5Public, contractEmpty]("stage3B").End()
	if err := definition.Validate(); !errors.Is(err, &Error{Code: MissingMethods}) {
		t.Fatalf("missing Walk = %v", err)
	}
	root.Bind[stage3Walk]((*stage3A)(nil), ".")
	if err := definition.Validate(); err != nil {
		t.Fatalf("saved root handle did not complete definition: %v", err)
	}
}

func TestM4InvalidWitnessIsStickyAndAddsNoBinding(t *testing.T) {
	definition := Define[stage3A]()
	level := definition.Level[stage3A, stage5Public, contractEmpty]("")
	live := &stage3A{}
	level.Bind[stage3Read](live, ".")
	level.Bind[stage3Add]((*stage3A)(nil), ".")
	level.Bind[stage3Walk]((*stage3A)(nil), ".")
	if len(level.level.bindings) != 2 {
		t.Fatalf("invalid witness added binding: %d", len(level.level.bindings))
	}
	err := definition.Validate()
	if !errors.Is(err, &Error{Code: InvalidProvider}) || !errors.Is(err, &Error{Code: MissingMethods}) {
		t.Fatalf("recorded and safe contract errors = %v", err)
	}
	level.Bind[stage3Read]((*stage3A)(nil), ".")
	err = definition.Validate()
	if !errors.Is(err, &Error{Code: InvalidProvider}) || errors.Is(err, &Error{Code: MissingMethods}) {
		t.Fatalf("sticky error after valid edit = %v", err)
	}
}

func TestM4InvalidCapabilityCallAddsNoBinding(t *testing.T) {
	definition := Define[stage3A]()
	level := definition.Level[stage3A, contractEmpty, contractEmpty]("")
	level.Bind[*stage3A]((*stage3A)(nil), ".")
	if len(level.level.bindings) != 0 {
		t.Fatal("invalid capability was registered")
	}
	if err := definition.Validate(); !errors.Is(err, &Error{Code: InvalidCapability}) {
		t.Fatalf("invalid capability error = %v", err)
	}
}

func TestM4MalformedProviderPathAddsNoBinding(t *testing.T) {
	definition := Define[stage3A]()
	level := definition.Level[stage3A, contractEmpty, contractEmpty]("")
	level.Bind[stage3Read]((*stage3A)(nil), "part..child")
	if len(level.level.bindings) != 0 {
		t.Fatal("malformed path added a binding")
	}
	var diagnostic *Error
	if err := definition.Validate(); !errors.As(err, &diagnostic) || diagnostic.Code != InvalidProvider || diagnostic.Path != "part..child" {
		t.Fatalf("path diagnostic = %+v", diagnostic)
	}
}

func TestM4NilInterfaceWitnessAndCompleteMissingDiagnostics(t *testing.T) {
	definition := Define[stage3A]()
	level := definition.Level[stage3A, stage5Public, contractEmpty]("")
	var nilRead stage3Read
	level.Bind[stage3Read](nilRead, ".")
	if len(level.level.bindings) != 0 {
		t.Fatal("nil interface witness added a binding")
	}
	err := definition.Validate()
	if !errors.Is(err, &Error{Code: InvalidProvider}) || !errors.Is(err, &Error{Code: MissingMethods}) {
		t.Fatalf("nil witness and missing-method diagnostics = %v", err)
	}
	for _, name := range []string{"Add", "Read", "Walk"} {
		if !strings.Contains(err.Error(), " "+name+" capability") {
			t.Fatalf("missing %s absent from %v", name, err)
		}
	}
}

func TestM4LaterEditRevalidatesCurrentDefinition(t *testing.T) {
	definition := Define[stage3A]()
	level := definition.Level[stage3A, contractPublicA, contractEmpty]("")
	level.Bind[stage3Read]((*stage3A)(nil), ".")
	if err := definition.Validate(); err != nil {
		t.Fatalf("first validation: %v", err)
	}
	level.Bind[stage3Read]((*stage3A)(nil), ".")
	if err := definition.Validate(); !errors.Is(err, &Error{Code: DuplicateBinding}) {
		t.Fatalf("later duplicate error = %v", err)
	}
}

func TestM4InvalidProviderAlsoReportsAllMissingMethods(t *testing.T) {
	definition := Define[stage3A]().
		Level[stage3A, contractPublicB, contractEmpty]("").
		Bind[contractRead]((*contractProvider)(nil), ".").End()
	err := definition.Validate()
	if !errors.Is(err, &Error{Code: InvalidProvider}) || !errors.Is(err, &Error{Code: MissingMethods}) {
		t.Fatalf("invalid provider and missing methods = %v", err)
	}
	var missing []string
	var collect func(error)
	collect = func(item error) {
		if joined, ok := item.(interface{ Unwrap() []error }); ok {
			for _, child := range joined.Unwrap() {
				collect(child)
			}
			return
		}
		if diagnostic, ok := item.(*Error); ok && diagnostic.Code == MissingMethods {
			if diagnostic.Expected == nil || diagnostic.Target != reflect.TypeFor[stage3A]() {
				t.Errorf("missing-method context = %+v", diagnostic)
			}
			missing = append(missing, diagnostic.Name)
		}
	}
	collect(err)
	if !slices.Equal(missing, []string{"Read", "Write"}) {
		t.Fatalf("missing methods = %v, want Read and Write", missing)
	}
}

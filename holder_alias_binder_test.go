package heiry

import (
	"errors"
	"reflect"
	"testing"
)

// r1binderHidden embeds the holder under an unexported alias name.
type r1binderHidden = Node

type r1BinderRoot struct{ r1binderHidden }

type r1BinderRootLeaf struct {
	r1BinderRoot
	Node
}

type r1BinderStore struct{ r1binderHidden }

type r1BinderOwner struct {
	Node
	store r1BinderStore
}

type r1BinderOwnerLeaf struct {
	r1BinderOwner
	Node
}

// r1BinderMetadata builds the two-level metadata that validation accepted
// before it rejected unexported holders, so that the binder's own holder
// check is reached. A nonempty component adds that level-0 slot.
func r1BinderMetadata(root, leaf reflect.Type, component string) *validatedNodes {
	holder := func(typ reflect.Type) int {
		for index := 0; index < typ.NumField(); index++ {
			if field := typ.Field(index); field.Anonymous && field.Type == reflect.TypeFor[Node]() {
				return index
			}
		}
		panic("no holder in " + typ.String())
	}
	parent, _ := leaf.FieldByName(root.Name())
	v := &validatedNodes{
		levels:       []nodeLevel{{typ: root}, {typ: leaf, parentField: root.Name()}},
		parentFields: []int{0, parent.Index[0]},
		parentValid:  []bool{false, true},
		nodeFields:   []int{holder(root), holder(leaf)},
		slots:        [][]providerSlot{{{path: ".", typ: root, nodeField: holder(root)}}, {{path: ".", typ: leaf, nodeField: holder(leaf)}}},
		slotIndex:    []map[string]int{{".": 0}, {".": 0}},
	}
	if component != "" {
		field, _ := root.FieldByName(component)
		v.slots[0] = append(v.slots[0], providerSlot{path: component, typ: field.Type, fields: field.Index, nodeField: holder(field.Type)})
		v.slotIndex[0][component] = 1
	}
	return v
}

func TestR1BinderNamesTheUnexportedHolderField(t *testing.T) {
	for _, tc := range []struct {
		name string
		bind func() error
		path string
	}{
		{"level node", func() error {
			x := &r1BinderRootLeaf{}
			v := r1BinderMetadata(reflect.TypeFor[r1BinderRoot](), reflect.TypeFor[r1BinderRootLeaf](), "")
			return bindValidatedNodes(v, x, []any{&x.r1BinderRoot, x})
		}, "r1BinderRoot.r1binderHidden"},
		{"component", func() error {
			x := &r1BinderOwnerLeaf{}
			v := r1BinderMetadata(reflect.TypeFor[r1BinderOwner](), reflect.TypeFor[r1BinderOwnerLeaf](), "store")
			return bindValidatedProviders(v, x, []map[string]any{{".": &x.r1BinderOwner, "store": &x.store}, {".": x}}, nil)
		}, "r1BinderOwner.store.r1binderHidden"},
	} {
		var diagnostic *Error
		if err := tc.bind(); !errors.As(err, &diagnostic) || diagnostic.Code != MissingNode || diagnostic.Level != 0 || diagnostic.Path != tc.path {
			t.Errorf("%s: %v, want MissingNode at level 0, path %q", tc.name, err, tc.path)
		}
	}
}

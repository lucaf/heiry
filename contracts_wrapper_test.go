package heiry

import (
	"reflect"
	"testing"

	"github.com/lucaf/heiry/testdata/stage4boundary"
)

type stage4Public interface{ Read() string }
type stage4Private interface{ Secret() string }

type stage4A struct{ Node }

func (a *stage4A) Read() string {
	return "A/" + Method[stage4Private](a.Node.Scope()).Secret()
}
func (*stage4A) Secret() string { return "A.secret" }

type stage4B struct {
	stage4A
	Node
}

func (b *stage4B) Read() string {
	return "B/" + ParentMethod[stage4Public](b.Node.Scope()).Read()
}
func (b *stage4B) Secret() string {
	return "B/" + ParentMethod[stage4Private](b.Node.Scope()).Secret()
}

type stage4Object struct{ leaf *stage4B }

func (o *stage4Object) Read() string {
	return Method[stage4Public](o.leaf.Node.Scope()).Read()
}

func TestStage4PublicWrapperAndPrivateDelegation(t *testing.T) {
	leaf := &stage4B{}
	bindNodeLevels([]nodeBinding{
		{node: &leaf.stage4A.Node, provider: &leaf.stage4A, capabilities: []reflect.Type{reflect.TypeFor[stage4Public](), reflect.TypeFor[stage4Private]()}},
		{node: &leaf.Node, provider: leaf, capabilities: []reflect.Type{reflect.TypeFor[stage4Public](), reflect.TypeFor[stage4Private]()}},
	})
	obj := &stage4Object{leaf: leaf}
	if got, private := stage4boundary.Consume(obj); got != "B/A/B/A.secret" || private {
		t.Fatalf("public/private trace = %q", got)
	}
	if _, ok := any(obj).(stage4Private); ok {
		t.Fatal("public wrapper exposes private capability")
	}
	methodSet := reflect.TypeOf(obj)
	if methodSet.NumMethod() != 1 || methodSet.Method(0).Name != "Read" {
		t.Fatalf("wrapper method set = %v", methodSet)
	}
}

type stage4Promoted struct {
	stage4A
	Node
}

func TestStage4PromotedMethodRegistrationTrustBoundary(t *testing.T) {
	provider := reflect.TypeFor[*stage4Promoted]()
	if _, err := validateContracts([]contractLevel{{public: reflect.TypeFor[stage4Public](), private: reflect.TypeFor[contractEmpty](), bindings: []contractBinding{{capability: reflect.TypeFor[stage4Public](), provider: provider}}}}, 0); err != nil {
		t.Fatalf("promoted method satisfies signature check: %v", err)
	}
	leaf := &stage4Promoted{}
	d := bindNodeLevels([]nodeBinding{{node: &leaf.Node, provider: leaf, capabilities: []reflect.Type{reflect.TypeFor[stage4Public]()}}})
	if got := d.levels[0][reflect.TypeFor[stage4Public]()]; got != leaf {
		t.Fatalf("registered provider = %p, want %p", got, leaf)
	}
}

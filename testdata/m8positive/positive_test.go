package m8positive

import (
	"reflect"
	"testing"

	"github.com/lucaf/heiry"
)

type read interface{ Read() int }
type count interface{ Count() int }
type secret interface{ Secret() int }
type public interface {
	read
	count
}
type private interface{ secret }

type reader struct {
	heiry.Node
	value int
}

func (r *reader) Read() int { return r.value }

type counter struct {
	heiry.Node
	value int
}

func (c *counter) Count() int { return c.value }

type hiddenSecret struct {
	heiry.Node
	value int
}

func (s *hiddenSecret) Secret() int { return s.value }

type root struct {
	heiry.Node
	Exported reader
	nested   struct{ named hiddenSecret }
	counter  // anonymous unexported component
}

type leaf struct {
	root
	heiry.Node
}

func (l *leaf) Read() int {
	return heiry.ParentMethod[read](l.Node.Scope()).Read() +
		heiry.Method[secret](l.Node.Scope()).Secret() +
		heiry.Method[count](l.Node.Scope()).Count()
}

type publicView struct{ state *leaf }

func (v *publicView) Read() int  { return heiry.Method[read](v.state.Node.Scope()).Read() }
func (v *publicView) Count() int { return heiry.Method[count](v.state.Node.Scope()).Count() }

func definition() *heiry.Definition[leaf] {
	d := heiry.Define[leaf]()
	d.Level[root, public, private]("").
		Bind[read]((*reader)(nil), "Exported").
		Bind[count]((*counter)(nil), "counter").
		BindGroup((*hiddenSecret)(nil), "nested.named", heiry.Of[secret]()).End().
		Level[leaf, public, private]("root").
		Bind[read]((*leaf)(nil), ".").End()
	return d
}

func TestExternalUnexportedComponentsAndPlanModes(t *testing.T) {
	d := definition()
	x := &leaf{}
	x.root.Exported.value, x.root.counter.value, x.root.nested.named.value = 2, 3, 4
	// Definition references are made before preparation and used by the converted Plan.
	refs := []heiry.ProviderRef[leaf]{
		d.ComponentProvider[root]("nested.named", &x.root.nested.named),
		d.NodeProvider(x),
		d.ComponentProvider[root]("Exported", &x.root.Exported),
		d.NodeProvider(&x.root),
		d.ComponentProvider[root]("counter", &x.root.counter),
	}
	dynamic, err := d.Prepare()
	if err != nil {
		t.Fatal(err)
	}
	fast := dynamic.MustPrecompute()
	if err := fast.Init(x, refs...); err != nil {
		t.Fatalf("restricted components: %v", err)
	}
	y := &leaf{}
	y.root.Exported.value, y.root.counter.value, y.root.nested.named.value = 5, 6, 7
	if err := dynamic.Init(y,
		dynamic.NodeProvider(&y.root),
		dynamic.ComponentProvider[root]("Exported", &y.root.Exported),
		dynamic.ComponentProvider[root]("nested.named", &y.root.nested.named),
		dynamic.ComponentProvider[root]("counter", &y.root.counter),
		dynamic.NodeProvider(y)); err != nil {
		t.Fatalf("dynamic Init: %v", err)
	}
	for _, tc := range []struct {
		object *leaf
		want   int
	}{{x, 9}, {y, 18}} {
		var view public = &publicView{state: tc.object}
		if got := view.Read(); got != tc.want {
			t.Fatalf("public Read = %d, want %d", got, tc.want)
		}
		if _, exposed := any(view).(private); exposed {
			t.Fatal("public wrapper exposes private capability")
		}
		if heiry.ParentMethod[read](tc.object.Node.Scope()) != &tc.object.root.Exported ||
			heiry.Method[count](tc.object.root.Node.Scope()) != &tc.object.root.counter ||
			heiry.Method[secret](tc.object.root.Node.Scope()) != &tc.object.root.nested.named {
			t.Fatal("wrong actual component selected")
		}
		if heiry.Method[count](tc.object.root.counter.Node.Scope()) != &tc.object.root.counter ||
			heiry.Method[secret](tc.object.root.nested.named.Node.Scope()) != &tc.object.root.nested.named {
			t.Fatal("restricted component holder did not receive its scope")
		}
	}
}

func TestExternalRestrictedReflectionCapabilities(t *testing.T) {
	x := &leaf{}
	rootValue := reflect.ValueOf(x).Elem().FieldByName("root")
	nested := rootValue.FieldByName("nested")
	component := nested.FieldByName("named")
	holder := component.FieldByName("Node")
	for _, tc := range []struct {
		name  string
		value reflect.Value
	}{{"component", component}, {"exported holder below restricted field", holder}} {
		if !tc.value.IsValid() {
			t.Fatalf("%s is invalid", tc.name)
		}
		if !tc.value.CanAddr() {
			t.Fatalf("%s is not addressable", tc.name)
		}
		if tc.value.CanInterface() {
			t.Fatalf("%s unexpectedly permits Interface", tc.name)
		}
		if tc.value.CanSet() {
			t.Fatalf("%s unexpectedly permits Set", tc.name)
		}
		for name, call := range map[string]func(){
			"Interface":      func() { tc.value.Interface() },
			"Addr.Interface": func() { tc.value.Addr().Interface() },
			"Set":            func() { tc.value.Set(tc.value) },
		} {
			t.Run(tc.name+"/"+name, func(t *testing.T) {
				defer func() {
					if recover() == nil {
						t.Fatal("restricted reflection operation did not panic")
					}
				}()
				call()
			})
		}
	}
}

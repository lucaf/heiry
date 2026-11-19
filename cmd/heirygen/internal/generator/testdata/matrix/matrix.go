// Package matrix is a heirygen test fixture for the parts of heiry's
// supported model that a generator could narrow by accident: several
// definitions over one leaf type, a standalone ancestor, same-type component
// slots, a provider type reused at two levels, nested, anonymous, unexported
// and generic components, an alias-named parent, a contract that grows at a
// later level, and an operation interface from another package. Its
// specifications use handwritten operation interfaces and no wrapper.
package matrix

//go:generate go run github.com/lucaf/heiry/cmd/heirygen

import (
	"fmt"

	"github.com/lucaf/heiry"
)

type (
	Namer   interface{ Name() string }
	Counter interface{ Count() int }
	Tracer  interface{ Trace(event string) }
	Limiter interface{ Limit() int }
)

type (
	Public interface {
		Namer
		fmt.Stringer
	}
	Private interface {
		Counter
		Tracer
	}
	// LeafPrivate adds an operation at the leaf level only.
	LeafPrivate interface {
		Private
		Limiter
	}
)

// tally is used for two slots of the root and again for one slot of the leaf.
type tally struct {
	heiry.Node
	n int
}

func (c *tally) Count() int { return c.n }

// journal is generic; its instantiation is the provider type.
type journal[T any] struct {
	heiry.Node
	events []T
}

func (j *journal[T]) Trace(event string) {
	var zero T
	j.events = append(j.events, zero)
	_ = event
}

// shell holds a nested component: the path to it has two steps.
type shell struct {
	label string
	inner tally
}

type root struct {
	heiry.Node
	left, right  tally
	shell        shell
	journal[int] // anonymous component, named after its type
}

func (r *root) Name() string   { return "root" }
func (r *root) String() string { return "root:" + r.Node.Virtual[Namer]().Name() }

// Parent is the name under which branch embeds root.
type Parent = root

type branch struct {
	Parent
	heiry.Node
}

func (b *branch) Name() string { return "branch > " + b.Node.Parent[Namer]().Name() }

type crown struct {
	branch
	heiry.Node
	own   tally
	limit int
}

func (c *crown) Name() string { return "crown > " + c.Node.Parent[Namer]().Name() }
func (c *crown) Limit() int   { return c.limit }

// wideSpec registers the crown's own Name and counts with the leaf's tally.
//
//heiry:spec
const wideSpec = `
hierarchy wide crown
import fmt "fmt"
level root public=Public private=Private
	node Namer fmt.Stringer
	component left tally Counter
	component journal journal[int] Tracer
level branch parent=Parent
	node Namer
level crown public=Public private=LeafPrivate
	node Namer Limiter
	component own tally Counter
`

// narrowSpec is a second definition over the same leaf type. It leaves the
// crown's Name unregistered, counts with the root's other same-type slot,
// and reaches a nested component through a two-step path.
//
//heiry:spec
const narrowSpec = `
hierarchy narrow crown
import fmt "fmt"
level root public=Public private=Private
	node Namer fmt.Stringer
	component right tally Counter
	component shell.inner tally Tracer
level branch parent=Parent
level crown
`

// branchSpec makes the intermediate level a standalone target.
//
//heiry:spec
const branchSpec = `
hierarchy standalone branch
import fmt "fmt"
level root public=Public private=Private
	node Namer fmt.Stringer
	component left tally Counter
	component journal journal[int] Tracer
level branch parent=Parent
	node Namer
`

// Trace lets a tally stand in as the tracer of the narrow definition.
func (c *tally) Trace(string) { c.n++ }

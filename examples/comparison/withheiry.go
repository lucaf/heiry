package main

import "github.com/lucaf/heiry"

// The heiry version of the same application.

type (
	Rendering interface {
		Render(item string) (string, error)
	}
	Checking  interface{ Check(item string) error }
	Prefixing interface{ Prefix() string }

	hPublic  interface{ Rendering }
	hPrivate interface {
		Checking
		Prefixing
	}
)

type hBase struct{ heiry.Node }

func (b *hBase) Render(item string) (string, error) {
	if err := b.Node.Virtual[Checking]().Check(item); err != nil { // virtual call
		return "", err
	}
	return b.Node.Virtual[Prefixing]().Prefix() + item, nil // virtual call
}

func (b *hBase) Check(item string) error {
	if item == "" {
		return errEmpty
	}
	return nil
}

func (b *hBase) Prefix() string { return "- " }

// hLimiter is a component of the strict level. Its scope records that level,
// so the parent call needs no hand-wired link.
type hLimiter struct {
	heiry.Node
	max      int
	rejected int
}

func (l *hLimiter) Check(item string) error {
	if len(item) > l.max {
		l.rejected++
		return errTooLong
	}
	return l.Node.Parent[Checking]().Check(item)
}

type hStrict struct {
	hBase
	heiry.Node
	limits hLimiter
}

type hFancy struct {
	hStrict
	heiry.Node
}

func (f *hFancy) Render(item string) (string, error) {
	// hStrict registers no Rendering, so the parent call reaches hBase.
	out, err := f.Node.Parent[Rendering]().Render(item)
	if err != nil {
		return "", err
	}
	return "<" + out + ">", nil
}

func (f *hFancy) Prefix() string { return "* " }

// The declaration states which provider implements which operation at each
// level. The component is registered directly; no forwarder is needed.
var fancy = heiry.Define[hFancy]().
	Level[hBase, hPublic, hPrivate]("").
	BindGroup((*hBase)(nil), ".", heiry.Of[Rendering](), heiry.Of[Checking](), heiry.Of[Prefixing]()).End().
	Extend[hStrict]().
	Bind[Checking]((*hLimiter)(nil), "limits").End().
	Extend[hFancy]().
	Bind[Rendering]((*hFancy)(nil), ".").
	Bind[Prefixing]((*hFancy)(nil), ".").End()

// heiryRenderer is the public wrapper.
type heiryRenderer struct{ state *hFancy }

func (r *heiryRenderer) Render(item string) (string, error) {
	return r.state.Node.Virtual[Rendering]().Render(item)
}

func (r *heiryRenderer) Rejected() int { return r.state.limits.rejected }

// newHeiryRenderer binds a new object to plan. Init checks that every
// pointer is the expected part of this object before installing any scope.
func newHeiryRenderer(plan *heiry.Plan[hFancy], max int) (Renderer, error) {
	f := &hFancy{}
	f.limits.max = max
	err := plan.Init(f,
		plan.NodeProvider(&f.hStrict.hBase),
		plan.NodeProvider(&f.hStrict),
		plan.ComponentProvider[hStrict]("limits", &f.hStrict.limits),
		plan.NodeProvider(f))
	if err != nil {
		return nil, err
	}
	return &heiryRenderer{state: f}, nil
}

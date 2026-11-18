package main

// The self-interface version. Each object keeps an interface value that
// refers to the most-derived object, and virtual calls go through it.

// selfOps lists every operation a body may call virtually. Assigning the
// leaf to it makes the compiler check that the leaf covers all of them.
type selfOps interface {
	Render(item string) (string, error)
	check(item string) error
	prefix() string
}

type selfBase struct{ self selfOps }

func (b *selfBase) Render(item string) (string, error) {
	if err := b.self.check(item); err != nil { // virtual call
		return "", err
	}
	return b.self.prefix() + item, nil // virtual call
}

func (b *selfBase) check(item string) error {
	if item == "" {
		return errEmpty
	}
	return nil
}

func (b *selfBase) prefix() string { return "- " }

// selfLimiter is a component of the strict level. A component is not part of
// the embedding chain, so no selector leads from it to "the check below my
// level". The constructor has to hand it that implementation.
type selfLimiter struct {
	parent   interface{ check(item string) error }
	max      int
	rejected int
}

func (l *selfLimiter) check(item string) error {
	if len(item) > l.max {
		l.rejected++
		return errTooLong
	}
	return l.parent.check(item)
}

type selfStrict struct {
	selfBase
	limits selfLimiter
}

// check forwards to the component, which makes it the strict level's
// implementation in the method set.
func (s *selfStrict) check(item string) error { return s.limits.check(item) }

type selfFancy struct{ selfStrict }

func (f *selfFancy) Render(item string) (string, error) {
	// Parent call through a selector. selfStrict declares no Render, so Go
	// selects the one promoted from selfBase.
	out, err := f.selfStrict.Render(item)
	if err != nil {
		return "", err
	}
	return "<" + out + ">", nil
}

func (f *selfFancy) prefix() string { return "* " }

func (f *selfFancy) Rejected() int { return f.limits.rejected }

func newSelfRenderer(max int) Renderer {
	f := &selfFancy{}
	f.limits.max = max
	f.limits.parent = &f.selfBase // parent link for the component
	f.self = f                    // does not compile unless *selfFancy covers selfOps
	return f
}

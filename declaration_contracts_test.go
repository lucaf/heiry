package heiry

import (
	"errors"
	"reflect"
	"testing"
)

type m4Read interface{ Read() int }
type m4Secret interface{ Secret() string }
type m4Extra interface{ Extra(int) int }
type m4Transform interface{ Transform(string) string }
type m4Finish interface{ Finish() }

type m4APublic interface{ Read() int }
type m4APrivate interface{ Secret() string }
type m4BPublic interface {
	m4APublic
	Extra(int) int
}
type m4BPrivate interface {
	m4APrivate
	Transform(string) string
}
type m4CPublic interface {
	m4BPublic
	Finish()
}
type m4CPrivate interface{ m4BPrivate }

type m4A struct{ Node }

func (*m4A) Read() int      { return 1 }
func (*m4A) Secret() string { return "A" }

type m4B struct {
	m4A
	Node
}

func (*m4B) Extra(n int) int           { return n + 1 }
func (*m4B) Transform(s string) string { return "B/" + s }

type m4C struct {
	m4B
	Node
}

func (*m4C) Finish() {}

type m4CAlias = m4C

func TestM4ChainedAndSeparateDeclarations(t *testing.T) {
	chained := Define[m4C]().
		Level[m4A, m4APublic, m4APrivate]("").
		Bind[m4Read]((*m4A)(nil), ".").
		Bind[m4Secret]((*m4A)(nil), ".").End().
		Level[m4B, m4BPublic, m4BPrivate]("m4A").
		Bind[m4Extra]((*m4B)(nil), ".").
		Bind[m4Transform]((*m4B)(nil), ".").End().
		Level[m4C, m4CPublic, m4CPrivate]("m4B").
		Bind[m4Finish]((*m4C)(nil), ".").End()
	if err := chained.Validate(); err != nil {
		t.Fatalf("chained definition: %v", err)
	}
	separate := Define[m4CAlias]()
	a := separate.Level[m4A, m4APublic, m4APrivate]("")
	b := separate.Level[m4B, m4BPublic, m4BPrivate]("m4A")
	c := separate.Level[m4C, m4CPublic, m4CPrivate]("m4B")
	a.Bind[m4Read]((*m4A)(nil), ".")
	a.Bind[m4Secret]((*m4A)(nil), ".")
	b.Bind[m4Extra]((*m4B)(nil), ".")
	b.Bind[m4Transform]((*m4B)(nil), ".")
	c.Bind[m4Finish]((*m4C)(nil), ".")
	if err := separate.Validate(); err != nil {
		t.Fatalf("separate definition: %v", err)
	}
	if !reflect.DeepEqual(chained, separate) {
		t.Fatalf("chained and separate metadata differ")
	}
}

func TestM4CurrentSnapshotRejectsInvalidParentAndProviderPath(t *testing.T) {
	definition := Define[m4C]().
		Level[m4A, m4APublic, m4APrivate]("").
		Bind[m4Read]((*m4A)(nil), ".").
		Bind[m4Secret]((*m4A)(nil), ".").End().
		Level[m4B, m4BPublic, m4BPrivate]("wrong").
		Bind[m4Extra]((*m4B)(nil), "component").End().
		Level[m4C, m4CPublic, m4CPrivate]("m4B").End()
	err := definition.Validate()
	if !errors.Is(err, &Error{Code: InvalidHierarchy}) || !errors.Is(err, &Error{Code: InvalidProvider}) {
		t.Fatalf("safe structural and provider errors = %v", err)
	}
}

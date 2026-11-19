// Command components is a heirygen test fixture: the shipped components
// example with its operation interfaces, declaration, references, and
// wrapper generated. The generator's tests require it to print exactly what
// examples/components prints. It is a test fixture, not a usage example.
package main

//go:generate go run github.com/lucaf/heiry/cmd/heirygen

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/lucaf/heiry"
)

// The contracts list the operations with their signatures. The generator
// derives one single-method interface per operation from them: GetOp, PutOp
// and RecordOp.
type (
	Public interface {
		Get(key string) (string, bool)
		Put(key, value string)
	}
	Private interface{ Record(event string) }
)

// store supplies two operations for the repository level.
type store struct {
	heiry.Node
	data map[string]string
}

func (s *store) Get(key string) (string, bool) {
	value, ok := s.data[key]
	return value, ok
}

func (s *store) Put(key, value string) {
	s.data[key] = value
	// A virtual call from a component selects the most-derived RecordOp.
	s.Node.Virtual[RecordOp]().Record("put " + key)
}

// auditLog supplies the third operation of the same level.
type auditLog struct {
	heiry.Node
	events []string
}

func (l *auditLog) Record(event string) { l.events = append(l.events, event) }

// repository is the root level. Its node implements nothing itself; its two
// components do.
type repository struct {
	heiry.Node
	store store
	log   auditLog
}

// cache overrides Get and Put at the cachedRepository level.
type cache struct {
	heiry.Node
	entries map[string]string
}

func (c *cache) Get(key string) (string, bool) {
	if value, ok := c.entries[key]; ok {
		return value, true
	}
	// A parent call from a component starts below the level that owns it.
	value, ok := c.Node.Parent[GetOp]().Get(key)
	if ok {
		c.entries[key] = value
	}
	c.Node.Virtual[RecordOp]().Record("cache miss " + key)
	return value, ok
}

// Put drops the cached copy before the write continues below this level, so
// a later Get cannot return a stale value.
func (c *cache) Put(key, value string) {
	delete(c.entries, key)
	c.Node.Parent[PutOp]().Put(key, value)
}

// stamper overrides Record at the same level as cache.
type stamper struct {
	heiry.Node
	count int
}

func (s *stamper) Record(event string) {
	s.count++
	s.Node.Parent[RecordOp]().Record(fmt.Sprintf("#%d %s", s.count, event))
}

type cachedRepository struct {
	repository
	heiry.Node
	cache   cache
	stamper stamper
}

// hierarchySpec is the generator's input. Each component line names the
// field that holds the provider, the provider's type, and the operations it
// registers. The wrapper line asks for the public wrapper type wrapper.
//
//heiry:spec
const hierarchySpec = `
operations Public Private suffix=Op
hierarchy hierarchy cachedRepository
level repository public=Public private=Private
	component store store GetOp PutOp
	component log auditLog RecordOp
level cachedRepository
	component cache cache GetOp PutOp
	component stamper stamper RecordOp
wrapper wrapper
`

func newObject() *cachedRepository {
	r := &cachedRepository{}
	r.store.data = map[string]string{}
	r.cache.entries = map[string]string{}
	return r
}

// Repository is the consumer-facing view.
type Repository interface {
	Public
	Events() []string
}

// Events is added by hand to the generated wrapper.
func (w *wrapper) Events() []string { return w.state.log.events }

// NewRepository allocates one object, binds it and returns the public view.
func NewRepository() (Repository, error) {
	r := newObject()
	if err := hierarchyInit(r); err != nil {
		return nil, err
	}
	return &wrapper{state: r}, nil
}

func run(out io.Writer) error {
	r := newObject()

	// A reference into a different object is rejected, and nothing in r is
	// initialized by the failed attempt.
	other := newObject()
	refs := hierarchyRefs(r)
	refs[1] = hierarchy.ComponentProvider[repository]("store", &other.repository.store)
	err := hierarchy.Init(r, refs...)
	var diagnostic *heiry.Error
	if !errors.As(err, &diagnostic) {
		return fmt.Errorf("foreign component was accepted: %v", err)
	}
	fmt.Fprintf(out, "foreign component rejected: %s at %s\n", diagnostic.Code, diagnostic.Path)
	fmt.Fprintln(out, "object untouched by the failed Init:", r.Node.Scope() == heiry.Scope{})

	if err := hierarchyInit(r); err != nil {
		return err
	}
	var repo Repository = &wrapper{state: r}

	repo.Put("color", "blue")
	for _, key := range []string{"color", "color", "shape"} {
		value, ok := repo.Get(key)
		fmt.Fprintf(out, "get %s: %q %t\n", key, value, ok)
	}
	repo.Put("color", "red")
	value, ok := repo.Get("color")
	fmt.Fprintf(out, "get color after update: %q %t\n", value, ok)
	for _, event := range repo.Events() {
		fmt.Fprintln(out, "event:", event)
	}

	// Seen from the cache component, the virtual RecordOp is its sibling at the
	// same level, while the parent RecordOp is the root log: a parent call
	// never selects a sibling.
	fmt.Fprintln(out, "virtual Recorder from cache is the sibling stamper:",
		r.cache.Node.Virtual[RecordOp]() == RecordOp(&r.stamper))
	fmt.Fprintln(out, "parent Recorder from cache is the root log:",
		r.cache.Node.Parent[RecordOp]() == RecordOp(&r.repository.log))
	return nil
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "example failed:", err)
		os.Exit(1)
	}
}

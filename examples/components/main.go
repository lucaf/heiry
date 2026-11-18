// Command components shows several implementation providers at one
// inheritance level. Each level's operations are supplied by components:
// struct values stored inside that level's node.
//
// Run it with: go run ./examples/components
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/lucaf/heiry"
)

type (
	Getter interface {
		Get(key string) (string, bool)
	}
	Putter   interface{ Put(key, value string) }
	Recorder interface{ Record(event string) }
)

type (
	Public interface {
		Getter
		Putter
	}
	Private interface{ Recorder }
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
	// A virtual call from a component selects the most-derived Recorder.
	heiry.Method[Recorder](s.Node.Scope()).Record("put " + key)
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
	scope := c.Node.Scope()
	if value, ok := c.entries[key]; ok {
		return value, true
	}
	// A parent call from a component starts below the level that owns it.
	value, ok := heiry.ParentMethod[Getter](scope).Get(key)
	if ok {
		c.entries[key] = value
	}
	heiry.Method[Recorder](scope).Record("cache miss " + key)
	return value, ok
}

// Put drops the cached copy before the write continues below this level, so
// a later Get cannot return a stale value.
func (c *cache) Put(key, value string) {
	delete(c.entries, key)
	heiry.ParentMethod[Putter](c.Node.Scope()).Put(key, value)
}

// stamper overrides Record at the same level as cache.
type stamper struct {
	heiry.Node
	count int
}

func (s *stamper) Record(event string) {
	s.count++
	heiry.ParentMethod[Recorder](s.Node.Scope()).Record(fmt.Sprintf("#%d %s", s.count, event))
}

type cachedRepository struct {
	repository
	heiry.Node
	cache   cache
	stamper stamper
}

// Each registration names the field that holds its provider. BindGroup
// registers several operations for one component; Bind registers one and lets
// the compiler check that the provider implements it.
var hierarchy = heiry.Define[cachedRepository]().
	Level[repository, Public, Private]("").
	BindGroup((*store)(nil), "store", heiry.Of[Getter](), heiry.Of[Putter]()).
	Bind[Recorder]((*auditLog)(nil), "log").End().
	Level[cachedRepository, Public, Private]("repository").
	BindGroup((*cache)(nil), "cache", heiry.Of[Getter](), heiry.Of[Putter]()).
	Bind[Recorder]((*stamper)(nil), "stamper").End()

func newObject() *cachedRepository {
	r := &cachedRepository{}
	r.store.data = map[string]string{}
	r.cache.entries = map[string]string{}
	return r
}

// references lists one pointer per level node and one per registered
// component. Init compares each pointer with the address it expects inside r.
func references(r *cachedRepository) []heiry.ProviderRef[cachedRepository] {
	return []heiry.ProviderRef[cachedRepository]{
		hierarchy.NodeProvider(&r.repository),
		hierarchy.ComponentProvider[repository]("store", &r.repository.store),
		hierarchy.ComponentProvider[repository]("log", &r.repository.log),
		hierarchy.NodeProvider(r),
		hierarchy.ComponentProvider[cachedRepository]("cache", &r.cache),
		hierarchy.ComponentProvider[cachedRepository]("stamper", &r.stamper),
	}
}

// Repository is the consumer-facing view.
type Repository interface {
	Public
	Events() []string
}

type wrapper struct{ state *cachedRepository }

func (w *wrapper) Get(key string) (string, bool) {
	return heiry.Method[Getter](w.state.Node.Scope()).Get(key)
}

func (w *wrapper) Put(key, value string) {
	heiry.Method[Putter](w.state.Node.Scope()).Put(key, value)
}

func (w *wrapper) Events() []string { return w.state.log.events }

// NewRepository allocates one object, binds it and returns the public view.
func NewRepository() (Repository, error) {
	r := newObject()
	if err := hierarchy.Init(r, references(r)...); err != nil {
		return nil, err
	}
	return &wrapper{state: r}, nil
}

func run(out io.Writer) error {
	r := newObject()

	// A reference into a different object is rejected, and nothing in r is
	// initialized by the failed attempt.
	other := newObject()
	refs := references(r)
	refs[1] = hierarchy.ComponentProvider[repository]("store", &other.repository.store)
	err := hierarchy.Init(r, refs...)
	var diagnostic *heiry.Error
	if !errors.As(err, &diagnostic) {
		return fmt.Errorf("foreign component was accepted: %v", err)
	}
	fmt.Fprintf(out, "foreign component rejected: %s at %s\n", diagnostic.Code, diagnostic.Path)
	fmt.Fprintln(out, "object untouched by the failed Init:", r.Node.Scope() == heiry.Scope{})

	if err := hierarchy.Init(r, references(r)...); err != nil {
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

	// Seen from the cache component, the virtual Recorder is its sibling at the
	// same level, while the parent Recorder is the root log: a parent call
	// never selects a sibling.
	scope := r.cache.Node.Scope()
	fmt.Fprintln(out, "virtual Recorder from cache is the sibling stamper:",
		heiry.Method[Recorder](scope) == Recorder(&r.stamper))
	fmt.Fprintln(out, "parent Recorder from cache is the root log:",
		heiry.ParentMethod[Recorder](scope) == Recorder(&r.repository.log))
	return nil
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "example failed:", err)
		os.Exit(1)
	}
}

# Alternatives and future possibilities

heiry is not the only way to get overridable behavior in Go, and often not the simplest. This page compares it with a hand-written self interface, using the two implementations in the [comparison example](../../examples/comparison/main.go), and says when the simpler approaches are the better choice. The last section lists possible future work.

## The application

Both versions format list items through three levels:

- **base** renders an item after checking it, using a prefix.
- **strict** adds a length limit. The limit lives in a component, a struct stored in the strict level, with its own counter of rejected items.
- **fancy** changes the prefix and wraps the base rendering.

[selfiface.go](../../examples/comparison/selfiface.go) is the self-interface version and [withheiry.go](../../examples/comparison/withheiry.go) the heiry version. The program runs both on the same inputs and fails if any result differs.

## The self-interface pattern

The root struct keeps an interface value that refers to the most-derived object. Bodies make virtual calls through it, and reach a parent implementation with an ordinary selector:

```go
type selfBase struct{ self selfOps }

func (b *selfBase) Render(item string) (string, error) {
	if err := b.self.check(item); err != nil { // virtual call
		return "", err
	}
	return b.self.prefix() + item, nil // virtual call
}

func (f *selfFancy) Render(item string) (string, error) {
	out, err := f.selfStrict.Render(item) // parent call through a selector
	...
}
```

The constructor sets `f.self = f`. That assignment compiles only if the leaf implements every operation in `selfOps`.

## Side by side

| Concern | Self interface | heiry |
| --- | --- | --- |
| Virtual call | `b.self.check(item)` | `b.Node.Virtual[Checking]().Check(item)` |
| Parent call from a level | A selector naming the parent, `f.selfStrict.Render(item)` | `f.Node.Parent[Rendering]().Render(item)` |
| Parent call from a component | A link the constructor wires by hand, `l.parent.check(item)` | `l.Node.Parent[Checking]().Check(item)` |
| Stating an override | Declaring a method with that name | An explicit registration with one of the `Bind` forms |
| Making a component the level's implementation | A forwarding method on the level | Registering the component with its field path |
| Completeness of the hierarchy | Checked by the compiler at `f.self = f` | Checked when the Definition is prepared, at run time |
| Hiding internal operations | Unexported methods, within one package | A private contract and a public wrapper |
| Setting up an object | Assign `self`, plus one link per component | `Init` with one reference per level and per component |
| Dependencies | None | The heiry package |

## What the compiler checks

**Self interface.** The compiler checks that the leaf has every operation with the right signature, at the single assignment to `self`. A missing or mistyped method does not build. This is the strongest check available and heiry does not match it.

**heiry.** The compiler checks every call made through `Virtual` and `Parent`, and that each `Bind` or `BindNode` witness implements its operation. It does not check that the hierarchy is complete, that a grouped registration's provider implements each listed operation, or that contracts extend correctly. Those are checked when the Definition is prepared, and reported as typed errors before any object is initialized.

## What initialization checks

**Self interface.** Nothing is checked. A constructor that forgets `f.self = f` builds, and the first virtual call fails with a nil-pointer panic. A link wired to the wrong object, such as `f.limits.parent = &other.selfBase`, also builds; calls then silently cross into another object.

**heiry.** `Init` compares every supplied pointer with the address of the expected part of the object being initialized, refuses pointers into other objects, refuses a second initialization, and writes nothing unless every check passes. A call through an uninitialized object panics with a typed `Uninitialized` error.

Application code can add equivalent checks to a self-interface design. They are not difficult, but each hierarchy has to write and maintain its own.

## Virtual and parent calls

In both approaches an ordinary Go call is not virtual. `b.check(item)` inside `selfBase.Render` would run the base check and never the strict level's limit, exactly as `b.Check(item)` would in the heiry version. A body has to go through `b.self` or through `b.Node.Virtual` for an override to be reached.

A selector such as `f.selfStrict.Render(item)` is a good parent call for a level in the embedding chain: Go resolves it to the nearest ancestor that declares the method, passing over levels that do not. It is not a general "implementation below the provider that is executing", and the limit component shows the difference:

- `selfLimiter` is a field of the strict level. It does not embed the base, so inside `selfLimiter.check` there is no selector that means "the check below my level". The constructor has to supply one: `f.limits.parent = &f.selfBase`.
- That link is correct only for this hierarchy. If the same limiter type were stored at another level, or in another hierarchy, each constructor would have to wire a different link.
- In the heiry version the limiter calls `l.Node.Parent[Checking]()`. Its holder records the level that owns this particular limiter, so the same body resolves correctly wherever the component is stored.

A self-interface design can express all of this; it does so with links and forwarders that the application writes for each case.

## Multiple providers and evolving hierarchies

- **Several providers at one level.** With a self interface, a component needs a `self` link if it makes virtual calls and a parent link if it calls the implementation below its level; the example's limiter needs only the parent link. The level also needs a forwarding method for each operation a component supplies. Embedding two components anonymously instead does not help when both, or a component and the parent, declare the same method: Go treats that method as ambiguous and the build fails. With heiry each component is registered with its path, and two registrations of one operation at one level are rejected during validation.
- **Adding a level or an override.** With a self interface a new override is picked up automatically by selectors and by `self`. With heiry it also needs a registration, and a new level needs one more reference in every `Init` call.
- **Forgetting the wiring.** What happens depends on whether another implementation still supplies the operation. If nothing does, both approaches report it: the self-interface version does not compile, and heiry's preparation returns `MissingMethods`. If an ancestor still supplies it, as the base check does in this example, both are silent: a missing forwarder or a missing registration simply leaves the component unused.

## Effort, dependencies and run-time cost

For this application, counting lines that are neither blank nor comments:

| | Self interface | heiry |
| --- | --- | --- |
| Lines of code | 54 | 83 |
| Named types | 5 | 10 |
| Extra Go requirement | None | The Go version in `go.mod` |

The heiry version needs one interface per operation, the declaration, a `heiry.Node` in every provider, and a wrapper. The self-interface version needs one forwarder and one hand-wired link.

`BenchmarkRender` in the example measures the same successful `Render("milk")` call through each version. Both do the same checks and build the same string; the heiry version resolves five times per call where the self-interface version makes three interface calls. Measured on 2026-11-18 (Go 1.27.1, darwin/arm64, Apple M2, medians of six runs), after the precomputed lookup was reduced to one table probe:

| Variant | Time per call | Allocation per call |
| --- | --- | --- |
| Self interface | 45.1 ns | 16 B, 2 allocations |
| heiry, dynamic Plan | 177 ns | 16 B, 2 allocations |
| heiry, precomputed Plan | 140 ns | 16 B, 2 allocations |

These numbers describe this call shape on this machine. Single runs ranged over 44.5 to 52.7 ns, 173 to 225 ns and 134 to 146 ns, and the compiler may optimize interface calls differently in other code. Run `go test -run '^$' -bench . -benchmem ./examples/comparison` to measure your own environment.

## What heiry standardizes

heiry puts four responsibilities in one tested place, where a self-interface design leaves them to each application:

- **Parent resolution relative to the executing provider's level**, for nodes and components alike.
- **Explicit operation ownership across several providers**, with duplicates rejected.
- **Validation of contracts and registrations for the whole hierarchy**, reported as structured errors.
- **Reusable immutable Plans and checked binding of each object.**

It does not make inheritance safer than the compiler can. In particular, neither approach can prove that a method is a real override. Satisfying an interface, or registering a provider, does not show that the provider declares the method itself rather than inheriting it. See "Trust in registrations" in the [guide](guide.md#rules-and-limitations).

## When the simpler approach is better

- **Plain embedding or composition** is enough when no ancestor needs to call a descendant's override. Most Go code is in this group.
- **A self interface** is the better choice for a hierarchy that lives in one package, where every provider is a level in the embedding chain and there are few virtual operations. It is checked at compile time, has no dependency, runs faster, and needs less code. For this page's application without the limit component, heiry offers no meaningful advantage.
- **heiry** becomes worth its cost when levels are assembled from several components that need parent calls, when the same provider types are reused at different levels or in different hierarchies, or when a project wants one uniform set of validation, diagnostics and object checks across many hierarchies.

## Possible future work

Nothing in this section is implemented, approved or scheduled. These are possibilities only.

### Further lookup optimization

A precomputed lookup now costs one table probe keyed by the operation type, an interface type assertion, and the call itself. A dynamic lookup still makes one probe to check that the operation is known and one more for each level it searches. The dynamic path could be given a similar layout. Removing the last probe would need another way to identify an operation at the call site. Neither has been designed.

### Generated dispatch accessors

The optional [heirygen](generation.md) command already writes operation interfaces, registrations, references, and public wrappers from a specification string. Generating typed virtual and parent accessors on provider types, in place of `Node.Virtual[T]()` and `Node.Parent[T]()` calls, has not been designed: a provider type can serve several slots, levels, and definitions with different operations, so one set of methods per type cannot describe it.

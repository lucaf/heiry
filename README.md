# heiry

heiry is a Go library for single inheritance with overridable methods. Ancestors are embedded by value in one object, implementations are registered explicitly, and virtual and parent calls are ordinary typed method calls.

heiry is an exploratory project. It explores whether Go 1.27's generic methods can give typed, checked inheritance with virtual and parent calls, and at what cost. It promises no support, and no stability beyond the `v0.x` note below. Calling it exploratory describes its purpose, not the quality of its code.

It is released as `v0.x`: until `v1.0.0`, the API may still change between versions. The [changelog](CHANGELOG.md) lists each release. It needs the Go version declared in `go.mod`.

## Installation

```sh
go get github.com/lucaf/heiry@latest
```

The optional generator, heirygen, is part of the same module; the [generation guide](docs/user/generation.md#installation-and-version) explains how to install and run it.

## Why

A method of an embedded Go type cannot reach an override declared by the type that embeds it. heiry provides that:

- **Virtual calls** select the most-derived implementation of an operation.
- **Parent calls** select the closest implementation below the level that is executing.
- **Several providers per level:** a level's operations can come from components stored inside it.
- **Checked setup:** the whole hierarchy is validated once, and every object is checked when it is initialized.

Virtual and parent calls end in ordinary interface method calls. heiry uses no reflective invocation and no `unsafe`.

## Who is this for

heiry may be worth its cost in one case: a component that makes parent calls, stored at a level that later levels extend, and reused at several levels or in several hierarchies. Each stored component's parent calls then resolve from the level that stores it, where a hand-written design wires a link for each place. That case has not been measured. In the cases that have been, plain Go needed less code and ran faster: plain embedding or a hand-written self interface is usually the simpler choice. [Alternatives](docs/user/alternatives.md#when-the-simpler-approach-is-better) gives the measurements and says when each approach fits.

The [comparison example](examples/comparison/withheiry.go) shows this case in its simplest form, without reuse: one component that makes a parent call, at a level that another level extends. Its strict level stores a limit component, which rejects long items and passes the others to the check below its level:

```go
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
```

The [self-interface version](examples/comparison/selfiface.go) of the same program needs a link wired by hand in its constructor, `f.limits.parent = &f.selfBase`, which is right only for this hierarchy. [Alternatives](docs/user/alternatives.md#virtual-and-parent-calls) explains the difference.

## Example

This is the package example in [example_test.go](example_test.go):

```go
// Each operation has one single-method interface.
type (
	Speaker interface{ Speak() string }
	Namer   interface{ Name() string }
)

// animal is the root level.
type animal struct{ heiry.Node }

// Speak calls Name virtually: the most-derived implementation answers.
func (a *animal) Speak() string {
	return a.Node.Virtual[Namer]().Name() + " makes a sound"
}

func (a *animal) Name() string { return "an animal" }

// dog embeds animal by value and overrides Name only.
type dog struct {
	animal
	heiry.Node
}

// Name extends the implementation below dog's level.
func (d *dog) Name() string {
	return "a dog, which is " + d.Node.Parent[Namer]().Name() + ","
}

func Example() {
	// Declare the hierarchy once. Speaker is the public contract and Namer the
	// private one; no object is involved yet.
	hierarchy := heiry.Define[dog]().
		Level[animal, Speaker, Namer]("").
		Bind[Speaker]((*animal)(nil), ".").
		Bind[Namer]((*animal)(nil), ".").End().
		Extend[dog]().
		Bind[Namer]((*dog)(nil), ".").End()

	// Allocate an object as usual, then bind it. The first Init also
	// validates the declaration.
	d := &dog{}
	err := hierarchy.Init(d, hierarchy.NodeProvider(&d.animal), hierarchy.NodeProvider(d))
	if err != nil {
		fmt.Println(err)
		return
	}

	// Speak is inherited from animal, yet it reaches dog's Name.
	fmt.Println(d.Node.Virtual[Speaker]().Speak())
	// Output: a dog, which is an animal, makes a sound
}
```

The [examples index](examples/README.md) lists five runnable programs, with the command for each and what its output shows.

## Limitations

- **Authoring cost:** one interface per operation, explicit registrations, a hand-written forwarder per public operation, and explicit references for every object. The optional [heirygen](docs/user/generation.md) command can write these from a specification string, through the same API. Specifications of one package can share levels and components, with `chain`, `from=`, `part`, and `use`.
- **Run-time cost:** each virtual or parent call does a lookup that a plain interface call does not.
- **Checks at preparation, not compilation:** hierarchy completeness is validated when a declaration is prepared. heirygen's outputs also check at compile time that each level's parent field, and the first field of each component's path, is declared directly in its own struct, so a specification that leaves out a level between two others fails the build.
- **Trust in registrations:** heiry cannot tell a real override from a method merely inherited by promotion.
- **Scope:** single inheritance through value embedding only. Deriving levels across packages is not supported.
- **Generator platforms:** heirygen is supported on Linux, including WSL2 on the Linux file system, and on macOS. Native Windows is not supported or tested. The library and generated code are plain Go.

The [user guide](docs/user/guide.md) covers these in detail, and the [generation guide](docs/user/generation.md#installation-and-version) lists the generator's platforms.

## Documentation

- [User guide](docs/user/guide.md): concepts, declaration, lifecycle, dispatch, components, errors and limitations, with diagrams.
- [Generating declarations with heirygen](docs/user/generation.md): the optional generator, its specification string, sharing levels and components between specifications, and its guards.
- [Alternatives and future possibilities](docs/user/alternatives.md): comparison with a self interface, and possible future work.
- [Examples](examples/README.md): runnable programs.
- [Changelog](CHANGELOG.md): what each release contains.

## Development

This project was conducted through Claude (Anthropic) and Codex (OpenAI). The way it was built is part of the exploration: see [How heiry was built](docs/development.md).

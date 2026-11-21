# heiry

heiry is a Go library for single inheritance with overridable methods. Ancestors are embedded by value in one object, implementations are registered explicitly, and virtual and parent calls are ordinary typed method calls.

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

For a small hierarchy in one package, plain embedding or a hand-written self interface is usually simpler and faster. [Alternatives](docs/user/alternatives.md) compares the approaches and says when each fits.

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
		BindNode[Speaker]((*animal)(nil)).
		BindNode[Namer]((*animal)(nil)).
		Extend[dog]().
		BindNode[Namer]((*dog)(nil))

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

## Runnable examples

```sh
go run ./examples/basic
go run ./examples/components
go run ./examples/lifecycle
go run ./examples/comparison
go run ./examples/generated
```

The [examples index](examples/README.md) describes each program and what its output shows.

## Limitations

- **Authoring cost:** one interface per operation, explicit registrations, a hand-written forwarder per public operation, and explicit references for every object. The optional [heirygen](docs/user/generation.md) command can write these from a specification string, through the same API.
- **Run-time cost:** each virtual or parent call does a lookup that a plain interface call does not.
- **Checks at preparation, not compilation:** hierarchy completeness is validated when a declaration is prepared.
- **Trust in registrations:** heiry cannot tell a real override from a method merely inherited by promotion.
- **Scope:** single inheritance through value embedding only. Deriving levels across packages is not supported.
- **Generator platforms:** heirygen is supported on Linux (including WSL2 on the Linux file system) and macOS; native Windows is not supported or tested. The library and generated code are plain Go.

The [user guide](docs/user/guide.md) covers these in detail, and the [generation guide](docs/user/generation.md#installation-and-version) lists the generator's platforms.

## Documentation

For users:

- [User guide](docs/user/guide.md): concepts, declaration, lifecycle, dispatch, components, errors and limitations, with diagrams.
- [Generating declarations with heirygen](docs/user/generation.md): the optional generator, its specification string, and its guards.
- [Alternatives and future possibilities](docs/user/alternatives.md): comparison with a self interface, and possible future work.
- [Examples](examples/README.md): runnable programs.
- [Changelog](CHANGELOG.md): what each release contains.

For contributors:

- [Specification](docs/specification.md): what the library must do.
- [Architecture](docs/architecture.md): how it is designed.
- [Testing](docs/testing.md): implementation milestones, tests and acceptance criteria.
- [Roadmap](docs/roadmap.md): implementation status and verification evidence.
- [Decisions](docs/decisions/): approved choices and design history.
- [AGENTS.md](AGENTS.md): mandatory working rules for Codex.

## Development

This project was conducted through Claude (Anthropic) and Codex (OpenAI).

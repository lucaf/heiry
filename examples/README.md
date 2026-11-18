# Examples

Each directory is a complete Go program that uses the heiry package through its exported API. They belong to the main module, so run them from the repository root. The [user guide](../docs/user/guide.md) explains the concepts they use.

| Example | What it teaches | Run |
| --- | --- | --- |
| [basic](basic/main.go) | A three-level hierarchy: selective overriding, inherited implementations, parent calls, shared embedded state, a public wrapper and a private operation | `go run ./examples/basic` |
| [components](components/main.go) | Several providers at one level, grouped and individual registration, component references, ownership checks, and parent calls that skip sibling components | `go run ./examples/components` |
| [lifecycle](lifecycle/main.go) | Declaration, eager and lazy preparation, Plan reuse across objects, dynamic and precomputed resolution, and setup errors | `go run ./examples/lifecycle` |
| [comparison](comparison/main.go) | One application written with a hand-written self interface and with heiry, checked to behave the same | `go run ./examples/comparison` |

Every program prints deterministic output. The exact expected output is the `// Output:` block of the `Example` function in each directory's `main_test.go`, and `go test` compares the real output with it:

```sh
go test ./examples/basic ./examples/components ./examples/lifecycle ./examples/comparison
```

## What to look for

### basic

- `describe: tagged > base`: the leaf's `Describe` extends the root's through a parent call that passes over the middle level, which does not implement it.
- The three `process` lines: `Process` is entered at the leaf, continues in the root through a parent call, and the root's body reaches the middle level's `Validate` and the leaf's `Transform` through virtual calls.
- `processed: 1`: the counter lives in the embedded root value of the same object; only the successful call incremented it.
- `wrapper exposes Validate: false`: the private operation is absent from the public wrapper.
- The `direct call` and `virtual call` lines: an ordinary Go call runs the body its selector names, while a virtual call from the same receiver reaches the leaf.

### components

- `foreign component rejected: ProviderOwnership at repository.store`: a pointer into another object is refused, and the next line shows the failed attempt initialized nothing.
- The `get` and `event` lines: one component supplies `Get` and `Put`, another supplies `Record`, and the derived level overrides all three with two more components.
- `get color after update: "red" true`: the derived level's `Put` drops its cached copy before the write continues to the root store, so the update is visible.
- The last two lines: from the `cache` component, a virtual call selects its sibling `stamper`, while a parent call selects the root `log`.

### lifecycle

- `repeated Prepare returns the same Plan: true`: preparation validates once and is then reused.
- The `dynamic`, `precomputed` and `lazy` lines are identical: the preparation schedule and the resolution mode do not change behavior. The program fails if they differ.
- `greetings counted per object: first=2 second=1`: objects sharing one Plan keep separate state.
- `second Init of one object: AlreadyInitialized` and `incomplete declaration: MissingMethods Sign`: setup problems are reported as typed errors.

### comparison

- The table shows the same result from both implementations for each input; the program fails if any differs.
- [selfiface.go](comparison/selfiface.go) and [withheiry.go](comparison/withheiry.go) hold the two versions. [Alternatives](../docs/user/alternatives.md) discusses what each one checks and costs.
- `go test -run '^$' -bench . -benchmem ./examples/comparison` measures one call through each version.

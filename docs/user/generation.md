# Generating declarations with heirygen

heirygen is an optional command that writes the repetitive parts of a heiry program for you: the single-method operation interfaces, the declaration with one registration per operation, the references every object needs, and the public wrapper with its forwarders. You keep writing the contracts, the level and component types, their method bodies, and your constructors, plus one specification string per hierarchy.

The generated code is ordinary Go that calls the same public API described in the [user guide](guide.md). Nothing about the library changes, every check it makes still runs, and code you write by hand keeps working alongside generated code. The runnable [generated example](../../examples/generated/main.go) shows the whole workflow on one page.

## Installation and version

The command lives in the heiry module, so the version of heiry your `go.mod` requires is also the version of the generator. There is no separate release to pick.

Record it as a tool of your module, which also adds the requirement:

```sh
go get -tool github.com/lucaf/heiry/cmd/heirygen
```

and run it with `go tool heirygen`. Alternatively, run it without the tool directive through `go run github.com/lucaf/heiry/cmd/heirygen`, which uses the same required version. Changing the heiry requirement changes both the library and the generator; regenerate afterwards. `heirygen -version` prints the module and version it was built from.

## A first specification

Write the contracts with their methods, the types, and the bodies as usual. Then add a string constant marked with `//heiry:spec` and a `go:generate` line:

```go
//go:generate go tool heirygen

type (
	Public  interface{ Send(to, text string) string }
	Private interface {
		Format(text string) string
		Record(line string)
	}
)

//heiry:spec
const notifierSpec = `
operations Public Private suffix=Op
hierarchy notifier signed
level sender public=Public private=Private
	node SendOp FormatOp
	component log journal RecordOp
level signed
	node FormatOp
wrapper mailer
`
```

Run `go generate ./...`. heirygen writes `main.heirygen.go` next to the file that holds the specification. It contains `SendOp`, `FormatOp`, and `RecordOp`; the declaration `notifier`; the helpers `notifierRefs`, `notifierInit`, and `notifierInitPlan`; and the wrapper type `mailer`. Method bodies use the generated operation interfaces in their dispatch calls, for example `s.Node.Parent[FormatOp]().Format(text)`.

A constructor allocates the object as usual and calls the helper:

```go
func newMailer(signature string) (*mailer, error) {
	s := &signed{signature: signature}
	if err := notifierInit(s); err != nil {
		return nil, err
	}
	return &mailer{state: s}, nil
}
```

## The specification

A specification is the value of one string constant whose doc comment contains the line `//heiry:spec`, with no space after `//`. The declaration names one constant with one string literal; a raw string literal is easiest to read and gives exact line numbers in diagnostics. A specification in a comment, a variable, or another file is not supported: the generated file can only check a constant.

Each line is one of eight forms. Blank lines and lines starting with `//` are ignored. Types are Go type expressions written without spaces.

| Line | Meaning |
| --- | --- |
| `import <name> "<path>"` | A package that type expressions in this specification use, as `<name>.Type`. Imports come first. |
| `operations <Contract>... [prefix=<p>] [suffix=<s>]` | Generate one single-method interface for each method of the listed contracts, named prefix + method + suffix. Give `prefix=` or `suffix=` whenever a method has no `existing` line; without either, the line is refused. A method several listed contracts share gets one interface. A contract is listed by at most one line in the package. |
| `existing <Method> <Type>` | Follows an `operations` line. The method is identified by an interface that already exists, handwritten or imported, and none is generated for it. |
| `hierarchy <Name> <LeafType>` | The generated declaration's name and the leaf type. At most one per specification. |
| `level <NodeType> [parent=<Field>] [public=<Type> private=<Type>]` | The next level, from the root. The root names both contracts. A later level names both to change them, or neither to keep the preceding pair. Without `parent=`, the parent field is the name an anonymous field of the preceding node type has, which is that type's name; write `parent=` for an alias name. |
| `node <Operation>...` | Operations the level's own node registers, by operation interface. |
| `component <path> <ProviderType> <Operation>...` | Operations a component of the level registers. The path is the field path from the level's node, as in `ComponentProvider`. |
| `wrapper <type> [field=<name>] [holder=<Field>]` | Generate the public wrapper: an unexported struct holding the leaf in one named unexported field, `state` by default, with one forwarder per public operation that dispatches through the leaf's directly embedded `heiry.Node`, `Node` by default. Write `holder=` when the leaf embeds it under an alias name. Use an exported alias: with an unexported one, generation succeeds but `Init` reports `MissingNode`, because the library installs only an exported holder. heirygen reads the leaf's struct, through local aliases and type definitions, to establish that field, and refuses the wrapper when it cannot. Both of the leaf's contracts must be listed by an `operations` line. |

A forwarder resolves the interface the `operations` line gives its method, so that must be the interface the hierarchy registers for it. When the hierarchy registers a handwritten or imported interface, or the interface another `operations` line generated, name it on the line that lists the wrapper's public contract: `existing Name Namer`. heirygen refuses a wrapper whose forwarder would resolve an interface that is not registered, and says which `existing` line to add. A registration that spells the same type through a local alias is accepted; the generated file asks the compiler to confirm that the two are identical.

Only what you list is registered. heirygen never looks at which methods a type has, so a method you do not list stays inherited, exactly as when you write the declaration by hand. A specification may hold only `operations` lines, for example to share operation interfaces between several hierarchies or definitions of one leaf type.

## Generated names

For a hierarchy `H` with leaf type `L`:

| Name | What it is |
| --- | --- |
| `H` | The `*heiry.Definition[L]`, declared with `Define`, an explicit `Level` per level, one `Bind` per registration with a typed-nil witness and its path, and `End`. Use it for `Prepare`, `MustPrepare`, `Validate`, and anything else a Definition offers. |
| `HRefs(*L)` | One reference per level node and registered component of the object, created by `H`. |
| `HInit(*L) error` | `H.Init` with those references. The first call validates and prepares the declaration. |
| `HInitPlan(*heiry.Plan[L], *L) error` | `Plan.Init` with the same references, for a Plan prepared from `H` or converted from one, dynamic or precomputed. |
| prefix + method + suffix | One operation interface per contract method, unless an `existing` line names it. |
| the `wrapper` type | The public wrapper struct and its forwarders. You can add your own methods to it in your own files. |

The generator refuses a generated name that your package already declares, and refuses helper names that `go test` would read as tests when the specification is in a test file. Names it introduces inside functions, such as parameters, are chosen so they never hide a name you use.

## Initialization helpers

The helpers call `Init` and nothing else, so every check of handwritten initialization applies: lineage, exact slots, physical ownership, atomic installation, and rejection of a second initialization. Diagnostics are the library's, with the same codes and context.

- An unusable Plan reports `InvalidPlan` and an invalid declaration reports its errors, before anything about the object.
- A nil object reports `InvalidProvider` with the target type and no level or path, which is what `Init` reports for a nil object. The references then point into a scratch value, so the helper never dereferences your nil pointer.
- A Plan prepared from another definition, even one with the same slots, is rejected with `BindingMismatch`, because the references are created by `H`.

Creating the references through the Definition costs a little. In one measurement on an Apple M2 laptop, `HInitPlan` took a median 922 ns per object against 804 ns for Plan-created references written out by hand, and 624 against 518 ns with eight goroutines initializing at once. It allocated one more object per call. The raw figures are in the [G1 evidence](../evidence/g1-generator/README.md). If that matters for your program, prepare and convert the Plan once and write the `Plan.Init` call by hand where it is hot.

## Where the files live

The output for `x.go` is `x.heirygen.go` in the same directory and package; for `x_test.go` it is `x.heirygen_test.go`. Its name keeps the source's platform suffixes, and its `//go:build` line expresses the source's constraint: the source's own `//go:build` line, or else its legacy `// +build` lines combined as the go command combines them, and `cgo` when the source imports `"C"`. So both files are built together in every configuration.

Its first line is `// Code generated by heirygen from x.go; DO NOT EDIT.` That marker tells heirygen the file is its own. It replaces or removes only files that carry the marker for the source their name belongs to, and stops with an error, changing nothing, if a file with an output name lacks the marker or a file with another name carries one. It also refuses an output name that differs only in case from another file of the directory: the go command rejects such pairs, and a case-insensitive file system treats them as one file. The exception is renaming a source in case only, `Main.go` to `main.go`, which renames its output with it. When a source no longer holds a specification, its output is removed.

Each run writes every new or changed output to a temporary file before putting any in place, and an output never appears half-written. Outputs are then put in place one file at a time, not as a set, and obsolete outputs are removed only after that: if a step fails partway, each output is whole, either new or as it was, the message lists which are new and which obsolete outputs were removed, and running the command again finishes the work. A new output is put in place with a hard link, so it never replaces a file that appeared while the command ran; when a run has a new output to put in place, including a case-only rename, and the file system has no hard links, the command stops before changing any output. Replacing or removing existing outputs needs no link. A case-only rename moves the old output aside under a hidden name and links the new one into place. If that fails, the old output is linked back under its old name, which also never replaces a file; if a file has taken that name meanwhile, that file is kept, the old output stays under the hidden name, and the error gives its exact path and how to recover it. In the rare case that heirygen cannot remove a hidden file it made, a temporary file or that hidden copy, the file is left and the error names it; the run still finishes the other outputs. A run that is interrupted, for example with Ctrl-C, can also leave hidden files named `.heirygen-…`, without a message. The go command and heirygen ignore them; delete them. If the interruption came during a case-only rename, the old output may be left only as such a file: run heirygen again to create the new one.

Avoid changing the same generated files while heirygen runs, for example by running two generations of one package at once or by switching branches during generation. Before replacing or removing an existing output, heirygen checks that it is still the file it read, but a change in the brief moment between that check and the replacement or removal is not detected, and that change is lost. heirygen never replaces or removes your other files; if you edit a source while it runs, run it again afterwards.

heirygen writes LF line endings. When it compares an existing output, it reads CRLF line endings as LF, so a checkout that converts line endings neither makes an output look out of date nor hides its marker; any other difference is a difference.

## Generate, build, and check

`go build` and `go test` never run the generator. Run `go generate ./...` after you change a specification, a listed contract, or the heiry version, and commit the generated files with the change. Committed output lets everyone build, test, and review without running a tool, and lets reviewers see what each specification change does.

`heirygen -check [dir]` writes nothing. It reports each output that is missing, differs from what this version would write, or belongs to a source that no longer has a specification. Run it in CI next to your tests:

```sh
go tool heirygen -check ./internal/shapes
```

| Exit status | Meaning |
| --- | --- |
| 0 | Success; with `-check`, everything is up to date |
| 1 | `-check` found outputs that are not up to date |
| 2 | Usage error |
| 3 | Generation or checking failed; the message says why and where |

`-v` lists the files a run wrote or removed.

## What the compiler catches, and what it cannot

The generated file carries compile-time assertions. Without regenerating, ordinary `go build`, `go vet`, and `go test` fail when:

- The specification text changed, or the constant became a variable.
- A listed contract gained, lost, or moved a method, or changed a signature, including through an embedded contract or an alias, in every build that compiles the output of the `operations` line that lists it.
- A wrapper's public or private contract changed, in every build that compiles the wrapper, wherever the `operations` lines are.
- The leaf field a wrapper's forwarders dispatch through is no longer a `heiry.Node` declared in the leaf, for example because an edit gave its name to a `*heiry.Node`, which could point into another object, or left only a field of that name promoted from an embedded struct, which may belong to another object.
- An `existing` interface no longer has exactly the method it was named for, in every build that compiles the registrations or forwarders that use it; this includes a registration through a local alias that now names another interface.
- A handwritten method on a generated wrapper has the name of a private operation.
- A registration that spells a forwarder's interface through an alias no longer denotes the same type.
- A field on the path to a level or component was renamed or removed, a provider or level type changed, a parent became embedded by pointer, or a provider no longer implements an operation it registers.

The error is reported in the generated file, next to a comment that says what to do.

The declaration also names each parent field as a string. That the field is an anonymous value embedding of the preceding level is checked by the library, not the compiler: a parent field that still exists but is now a named field compiles, and the first preparation reports `InvalidHierarchy`. The library's other validation, of contracts, coverage, and duplicates, likewise runs at first preparation as it does for handwritten declarations.

A leaf edit that embeds the holder under another name and gives the holder's old name to a named `heiry.Node` value also compiles. The wrapper then dispatches through a Node the library never installs, and its first call panics with `Uninitialized`. After changing the leaf's struct, run `heirygen -check` or regenerate: both establish the holder again and report the field to name.

These guards check particular properties; they do not prove that the generated file is what its specification asks for. A generated file that someone edited, or that a different generator version wrote, still compiles if it is well typed. Check mode finds such differences, and only where you run it. A defect of the generator itself would be repeated by check mode, which uses the same generator, so tests, review, and the library's own checks remain your protection there.

Two things are equally true of handwritten code and are not caught:

- A wrapper method with another name can still call a private operation. The guard sees names, not behavior, and `var _ Public = (*W)(nil)` only shows that the public methods exist.
- A registration is your statement that the provider owns that implementation. A specification that omits an override, or registers a provider that only inherits the method, is accepted as written.

## Supported and unsupported inputs

Supported: several hierarchies in one package, several definitions over one leaf type, shared and existing operation interfaces, equal method names in unrelated hierarchies through separate `operations` lines, same-type, nested, anonymous, generic, and unexported components, alias-named parents and holders, with an exported alias for a holder, contracts that grow at a later level, embedded local contracts and local aliases, variadic methods, named and unnamed parameters and results, multi-line signatures, specifications in test files, files with build constraints, legacy `// +build` lines, cgo, and platform suffixes, outputs checked out with CRLF line endings, and handwritten methods on generated wrappers.

Refused with a diagnostic:

- A contract that embeds an interface of another package, or is an alias of one. Write its methods in your contract, and add an `existing` line if that interface identifies the operation.
- Generic contracts, methods with type parameters, and type-set elements.
- A contract declared more than once in the directory, for example once per platform. heirygen reads every file of the directory regardless of build constraints.
- A contract that embeds an operation interface generated from it.
- Two lines that give one interface different methods, in any files, including test files and files that a build constraint leaves out.
- A registration whose guard heirygen cannot write for every build of its file: each `existing` line that names its interface, outside the registration's own file, has a signature that uses a name heirygen cannot show every one of those builds declares. It accepts a name declared in the registration's file, in a non-test file without build constraints or a platform suffix, or in a file with exactly the same conditions, compared as written, so equivalent conditions spelled differently, such as `//go:build linux` and a `_linux` suffix, count as different.
- A wrapper whose leaf contracts are not both listed, a wrapper or storage field that is exported, a wrapper whose leaf's directly embedded `heiry.Node` heirygen cannot establish, a holder that is another field, and a forwarder that would resolve an interface its hierarchy does not register.
- A signature in a file with a dot import that uses a name the package does not declare. Import that package with a name instead.
- An unqualified type in a specification that the package does not declare.
- An import whose package name is not the last element of its path and that has no explicit name, when a contract signature uses it.

Because the command reads every file of the directory, a generated name is the same in all build configurations of that directory. A wrapper is generated only when heirygen can read the leaf's struct, in the package, and find exactly one directly embedded `heiry.Node`. For a leaf declared in another package, or a holder embedded through an alias declared in another package, remove the `wrapper` line and write the wrapper by hand; the declaration and helpers are still generated. None of these limits applies to declarations you write by hand.

heirygen checks only the unqualified names of a signature it copies into a guard. A qualified type such as `syscall.Handle` is copied with its import, and may not exist on a platform where its package does. When a registration's guard would copy such a signature into a file that builds on such a platform, give that file's own `operations` line the `existing` line for the interface, with a contract whose signature every build of the file can compile: heirygen copies the hierarchy's own declaration first. Otherwise keep the hierarchy in a file that builds only where the type exists, such as a `_windows.go` file. A wrapper copies the leaf's own contracts, which its file already needs.

## Troubleshooting

| Message | Meaning |
| --- | --- |
| `undefined: notifierInit` from `go build` | The output does not exist yet. Run `go generate`. |
| `duplicate key false in map literal` in a generated file | The specification changed since generation. Run `go generate`. |
| `... is not constant` in a generated file | The specification is no longer a constant. Declare it with `const`. |
| `does not implement Public (missing method ...)` or `wrong type for method ...` in a generated file | A listed contract changed since generation. Run `go generate`; if the error stays, two listed contracts give one method different signatures, or an `existing` interface does not match the contract. |
| `ambiguous selector probe.X` in a generated file | If `X` is the wrapper's holder, the leaf now has that field only through an embedded struct: declare `heiry.Node` in the leaf again, or run `go generate`. Otherwise a handwritten method of the wrapper is named after private operation `X`: rename or remove it. |
| `cannot use &probe.X (value of type *struct{})` in a generated file | The wrapper's holder `X` is now promoted from deeper in the leaf, or gone. Declare `heiry.Node` in the leaf again, or run `go generate`. |
| `probe.X undefined` in a generated file | The private contract no longer has operation `X`. Run `go generate`. |
| `wrapper w forwards Name through NameOp, but hierarchy h registers Name as Namer` | Add `existing Name Namer` to the `operations` line that lists the public contract, so both name the same interface. |
| `existing M I names I for method M, but ... names it for method N` | Two lines, possibly in a test file or a file another build uses, contradict each other. Correct the wrong one: an interface identifies one operation in the whole package. |
| `registration R needs a guard in the output of F` | The guard heirygen writes into F's output copies the signature of an `existing` line for R's interface, and every such line outside F uses a name that some builds of F lack. Declare that name in a non-test file without build constraints or a platform suffix, or add an `existing` line for the interface in F. |
| `the generator cannot tell which operation X identifies` | The hierarchy registers an interface heirygen cannot read, typically an imported one. Add an `existing` line that names it for its method. |
| `holder=X names a field that L does not embed` | The field is not the leaf's directly embedded Node: a named Node, which the library never installs, or a pointer, perhaps to another object's. Write the `holder=` the message names. |
| `the generator cannot establish the directly embedded heiry.Node of the leaf` | heirygen cannot read the leaf's struct or find exactly one embedded Node in it. Remove the `wrapper` line and write the wrapper by hand. |
| `putting a new output in place needs a hard link` | The run had a new output to put in place, and the file system has no hard links. No output was changed. Generate on a file system that has them. |
| `the old output was kept as .heirygen-….tmp.old` | A case-only rename was interrupted because a file appeared at the output's name; that file was kept. Outputs the message lists as already put in place are new, and the others are as they were. Move that file away, then rename the kept file as the message says, or delete it, and run heirygen again. |
| `cannot use &leaf.X (value of type **heiry.Node)` in a generated file | The leaf's field `X`, which the wrapper dispatches through, became a pointer. Embed `heiry.Node` again, or run `go generate`, which names the holder to write. |
| `heirygen could not remove the hidden files` | heirygen made the named files, which the go command ignores, and could not remove them. The run exits with status 3 even when the rest of the message says `every output is up to date`, which means only those files remain. Delete them. |
| `wrapper W must be unexported` or `field=F must be unexported` | Public wrappers and their storage are unexported. Rename them. |
| `differs only in case from the existing X` | Rename `X`, or the source, so no two file names differ only in case. |
| `was created after heirygen read the directory` or `changed after heirygen read it` | Another program changed that file while heirygen ran, and heirygen left it as it is. Outputs the message lists as already put in place are new, those it lists as already removed are gone, and the others are as they were. Run heirygen again when nothing else is changing the package's generated files. |
| `undefined: XOp` in a generated file of an untagged or non-test file | The operations line that generates `XOp` is in a test file or a tagged file that this build leaves out. Move it to a file every such build includes. |
| `has the name of a heirygen output but does not start with its marker` | A file with an output name was not written by heirygen, or its marker was changed. Rename it, or remove it if it is a stale copy. |
| `starts with a heirygen marker but is not named as the output of ...` | A generated file was renamed or copied. Remove it and regenerate. |
| `... would generate X, which ... already declares` | A generated name collides with one of yours. Rename the hierarchy, the wrapper, or the operation prefix or suffix. |
| `"// heiry:spec" is not a marker` | Write `//heiry:spec` without a space. |

Positions in heirygen's messages point at the file line of the offending specification line when the constant is a raw string literal.

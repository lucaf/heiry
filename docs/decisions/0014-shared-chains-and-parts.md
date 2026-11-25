# ADR 0014 — Shared chains and parts in heirygen specifications

Status: accepted by the project owner on 2026-11-24. Proposed on 2026-11-22 for the work recorded in the [roadmap](../roadmap.md#findings-of-the-widget-experiment), and revised five times on 2026-11-23, after five independent reviews. The owner adopted `chain`, `from=`, `part` and `use` on 2026-11-22, subject to this record settling the open questions, and approved the revised record on 2026-11-24. Its implementation is recorded in the roadmap.

## Context

A heirygen specification lists every level of its hierarchy, from the root, with its registrations. In the widget experiment, five hierarchies shared one root level, three shared a button level, two shared a logged-button level, and two stored the same confirmation component. The specifications repeated those levels and that component line: 39 specification lines in 6 constants, and adding one operation to the root meant editing 5 of them. Handwritten declarations can share registrations through small functions; specifications cannot.

## Decision

Specifications in one package can share a run of levels and a group of component lines. Three line forms are added, `chain`, `part` and `use`, and the option `from=`, on the `chain` and `hierarchy` lines:

```text
chain <Name> [from=<Name>]
hierarchy <Name> <LeafType> [from=<Name>] [omit=InitPlan]
part <Name>
use <Part>
```

- `chain <Name>` starts a named run of levels. The `level` lines after it, with their `node`, `component` and `use` lines, belong to it, as they belong to a hierarchy. A chain has no leaf, no wrapper and no helpers, and generates no declaration.
- `from=<Name>` on a `hierarchy` or `chain` line names a hierarchy or a chain. The levels of the one that has it start with every level of the named one, with their parent fields, contracts and registrations; its own `level` lines follow.
- `part <Name>` starts a named group of `component` lines.
- `use <Part>` appears among the lines of a level, in a hierarchy or a chain. It adds the part's component lines to that level, at its own position. Their paths are relative to that level's node, as for a component line written there.

The widget program would read as follows. The operations line and the chain are in two constants of one file, so that an edit to one does not touch the outputs that use the other:

```text
operations widget kindBehavior prefix=op
```

```text
chain widgets
level base public=widget private=kindBehavior
	node opEnter opCancel opLine opLabel opSetEnabled opLoseFocus
```

```text
hierarchy buttons button from=widgets
level button
	node opBody opActivate
wrapper buttonWidget
```

```text
part confirming
	component confirm confirmation opBody opActivate opCancel opLoseFocus
```

```text
hierarchy confirmButtons confirmButton from=loggedButtons
level confirmButton
	use confirming
wrapper confirmButtonWidget
```

The menu item, logged button and confirming menu item hierarchies follow the same pattern. The program then has 26 specification lines in 8 constants, and adding one operation to the root means editing 1 line.

### Output

The generated declarations are written out in full, as today. A hierarchy's output is, byte for byte, the output of the same hierarchy written out in one constant of the same name, apart from the freshness assertions and the probes' comments. Written out means every `from=` replaced by the levels it takes, every `use` replaced by the part's component lines, and the import lines that those lines use added. The freshness assertions differ because the hierarchy's own assertion holds its own text, and one more assertion follows for each constant it takes lines from. The probes' comments differ because they name the constants where a fix can go, as described below.

A specification that uses none of the new lines keeps today's output exactly.

So every compile-time check of the written-out hierarchy is kept: one compiler-checked `Bind[Op]` per registration, the typed references, the probes for parent fields and component paths, the holder, contract, private-name and identity guards, and the repeated guards of existing interfaces. Every generation-time check runs on the hierarchy with its taken lines in place. The Definition is the same, so the library's validation and diagnostics are unchanged.

**Added assertions.** Each has the layout of the hierarchy's own assertion. Only its first comment line differs, so for a constant `widgetsChain` its comment reads:

```go
// The declarations below also take lines from the constant widgetsChain.
// If the map literal fails to compile with a duplicate key, widgetsChain has
// changed since then: run go generate. If the line above it fails, widgetsChain
// is no longer a constant.
```

**Probes' comments.** A probe for a missing level must send the reader to the constants where that level can be added. Between two levels of one constant, it can be added only in that constant. Where one constant's levels follow another's, it can be added at the end of the first or at the start of the second. In an output whose hierarchy uses `from=` or `use`, the comment above the probes therefore reads:

```go
// Each probe below compiles only while a struct declares a field itself, as
// the library requires: a level's parent field, and the first field of each
// component's path. If a selector below is ambiguous or has another type,
// that field is gone, has another type, or is reached only through another
// embedded struct; for a parent field, a level between the two is then
// missing from one of the constants that the probe's comment names.
// Regenerating writes the same probes, so first correct a constant that the
// probe's comment names, or the struct; then run go generate.
```

In such an output, and only there, each probe's own comment also ends with a second sentence naming where the fix can go. Like the header, this sentence leaves the probes' comments of today's outputs unchanged:
- **A parent field:** the constant that holds the level's `level` line, for example, for a second level in widgetsChain, `… Its level is in widgetsChain.` The probe of the first level of each constant whose levels follow another constant's levels names both constants. This holds wherever that boundary is, among taken levels as well as between taken and own levels. So button's probe reads, in the output of buttons and in the output of loggedButtons alike: `// button declares its parent field base, of type base. Its level is in buttonSpec, after the levels of widgetsChain.`
- **A component:** the generator writes one probe per slot, and component lines with the same path at one level share a slot (see [Component lines at one level](#component-lines-at-one-level)). The probe's comment names, in written-out order, the constant of each of the slot's lines, and for a line that came through `use`, also the constant of the `use` line. The fix can be in either: the part's line may be wrong, or the part may be used at a level whose node lacks the field. Each constant that holds some of the slot's lines is named once, even when several of those lines are in it. The constant of a `use` line is named separately, after "used in", even when it also holds some of the slot's lines. For one line written in the level: `… Its component line is in confirmButtonSpec.` For one line that came through `use`: `… Its component line is in confirmationPart, used in confirmButtonSpec.` For a slot with both: `… Its component lines are in confirmButtonSpec and in confirmationPart, used in confirmButtonSpec.`

In a scratch copy of the widget program, a level inserted between base and button in the structs alone failed three outputs, each at button's probe. By the rule above, that probe's comment now names both constants, `Its level is in buttonSpec, after the levels of widgetsChain.`, because the missing level can go at the end of widgetsChain or at the start of buttonSpec.

**Imports.** Each generated file imports exactly the packages that its own code uses, under the names that code uses. For a specification without the new lines this is what its output imports today, because every type expression of such a specification appears in its output. A chain's or part's own output holds no declaration of its lines, so it imports nothing for them, even when its constant shares a file with operations lines.

### Settled questions

**1. Guards hold whole texts.** Each assertion compares a whole constant with the text recorded at generation, in the form the hierarchy's own assertion has today. This is chosen because it is simple and cannot miss any change. Its cost is that an edit to any line of a used constant fails the build of every output that takes lines from it, until they are regenerated, including an edit to a line that no hierarchy takes. In the scratch copy of the widget program, a comment added to the chain's constant failed 6 outputs: the chain's own, and the 5 that take its lines. For this exploratory, demonstrative project, those extra failing builds are accepted. Keeping a chain or a part in a constant of its own, as the example does, limits them. Checking only the taken lines is possible; it is a rejected alternative, below.

**2. Guards are transitive.** S10 allows a run to fail partway, leaving some outputs new and the others as they were. Take `A from=B` and `B from=C`. If C is edited, and a run fails after writing B's output but before A's, A's old output still holds C's old lines, and a guard on B's text alone would pass. A hierarchy's output therefore asserts the text of every constant it takes lines from, directly or through another constant, whether through `from=` or `use`. Each constant has one assertion. They follow the hierarchy's own assertion in the order in which the written-out hierarchy first uses a line of each. No chain or hierarchy that `from=` names lacks levels of its own (question 5), so every constant on a `from=` path gives lines, and this rule asserts each of them. After any partial run, every output that still holds an old line of an edited constant fails to compile, and a rerun completes the work. The review showed this on the widget program, and showed that with assertions on direct uses only the same stale tree built.

**3. Imports go with the lines that use them.**
- An `import` line belongs to its constant, and each constant must still use each of its own import lines, as today.
- A taken line keeps the meaning its own constant gives its package names. Its types are resolved, and written into the output, with that constant's imports.
- A hierarchy never declares an import for a taken line: an import line that only a taken line would use is refused as unused, as today.
- A name may be bound to only one path within one generated file. The check covers every specification whose code ends up in that file, own and taken lines alike, not only the lines of one hierarchy. The diagnostic names both bindings: a specification's import line, or, for a copied signature, the import declaration of the Go file it comes from.
- One path under two names is accepted, as today, and the output imports it under both.

**4. A part, like a chain, is the only hierarchy-like content of its constant.** A constant holds at most one hierarchy, one chain or one part, together with any `import` and `operations` lines. After a `part` line come one or more `component` lines and nothing else. A part holds no `level`, `node`, `use` or `wrapper` line. A `use` line follows a `level` line, as a `component` line does, and not the `wrapper` line.

**5. Names and outputs.**
- Hierarchy, chain and part names share one namespace in each package clause: each name is unique among all three. `from=` names a hierarchy or a chain, and `use` names a part.
- A hierarchy name stays a Go identifier that its output declares, with today's collision checks. A chain or part name declares nothing in Go. It must be an identifier other than `_`, and it may equal a Go name of the package.
- A chain with no level of its own is refused while its own constant is read, with or without `from=`. Without `from=`, it has its own message, for example: `chain buttons2 has no level`.
- With `from=`, such a chain would only give another name to the chain or hierarchy it names, and it saves no line. The message keeps the target's name as written, without claiming that it exists, since links are not yet resolved, for example: `chain buttons2 has no level of its own; write from=buttons in the hierarchy instead`.
- A hierarchy with no level of its own stays allowed, because it generates a second definition, for example with another wrapper. But a `from=` that names such a hierarchy is refused, for the same reason: it takes nothing that the hierarchy's own target does not give. The refusal is one of the link checks, reported at that `from=` line, and names that target, for example: `buttons2 has no level of its own, so from= cannot name it; name its target directly, with from=buttons`. Added on 2026-11-24: when that target would lead back to the hierarchy or chain whose `from=` names it, the refusal leaves out the advice to name the target directly and keeps the rest of the message.
- Both refusals keep question 2's rule complete. A constant that gives no line would not be among a dependent's assertions, so a change to its `from=` would not fail the dependent's old output: the review built such an old output, and it compiled.
- A file whose constants hold only chains or parts still gets an output, as every file that holds a specification does. That output holds one freshness assertion per constant and nothing else. S10's rules for output files, ownership and obsolete outputs are therefore unchanged.

**6. `from=` takes levels and nothing else.**
- It takes each level's line, parent field, contracts, and `node`, `component` and `use` lines.
- It does not take the named hierarchy's name, leaf, `wrapper` line or `omit=` option, nor the `operations`, `existing` and `import` lines of its constant. Imports serve the taken lines, as question 3 says.
- The hierarchy's own `hierarchy` and `wrapper` lines alone decide its helpers and wrapper. A `chain` line takes no `omit=`, and a chain has no `wrapper` line: both are refused.
- The first own level after `from=` follows the last taken level, as consecutive levels do today. Without `parent=`, its parent field is the name that an anonymous field of the last taken node type has. Without contracts, it keeps the last taken level's.
- A hierarchy with no level of its own is allowed. Its leaf must then be the last taken level, and no `from=` may name it (question 5).

### The leaf and its last level

A hierarchy's leaf must be its last level. In every specification, with or without the new lines, the two are compared as follows:
- **Identical spellings:** a leaf spelled exactly as its last level is accepted, as today, before any alias is followed. Only otherwise are the two compared as types, by the rules below.
- **Parsed expressions:** both are parsed and compared as Go type expressions, not as printed text, so spacing does not matter. So a leaf `IB`, with `type IB = gen2[int, string]`, matches a last level written `gen2[int,string]` in the specification. A parenthesized type such as `(T)` is compared as `T`.
- **Package names:** each package name is replaced by the import path it stands for, in the file or constant where it is written, so one package under two import names is one package.
- **Local aliases:** they are followed, as interface identity already follows them. Only an alias declared once in the package and without type parameters is followed; its right side is read with the imports of its own file. If following them meets a cycle of aliases, or an alias whose right side uses an import whose package name cannot be read from its path, before the two match, the comparison is refused with that reason, naming the alias. Interface identity's alias lookup gives up silently after 32 steps today; the new comparison reports the cycle instead.
- **Type arguments:** package names and local aliases inside them are resolved in the same way, so `box[MyInt]`, with `type MyInt = int`, matches `box[int]`.

This comparison has a function of its own, separate from interface identity, so a change to one does not change the other.

A spelling that is still refused gets the reason, after the message that names both spellings. For example: `Btn is not declared`, `Btn is declared more than once`, `Btn is declared as a type of its own, not as an alias`, `Btn has type parameters`, `lib.Btn is declared in another package, whose aliases the generator does not read`, `Btn's right side uses Base from a dot import in alias.go; import that package with a name`, `the alias Btn is in a cycle of aliases: Btn, Base, Btn`, or `Btn's right side uses ui, but the generator cannot tell which import of alias.go provides ui; give that import the explicit name ui`, worded as the generator already words it for signatures. The dot-import reason matches S10's refusal of dot-imported names in copied signatures.

When the last level is a taken line, a mismatch is reported at the leaf's line, the `hierarchy` line. The message names the taken line with its constant, and both spellings, for example: `plainButtonSpec, line 2: the last level is button, at buttonSpec, line 3, but the leaf is loggedButton`.

An alias declared only in a file that some builds leave out, such as a test file, is followed like any other. This keeps today's behavior for any type declared that way: generation succeeds, and a build that leaves the file out fails in the generated file, as a specification type declared only in a test file does today.

A wrong acceptance would still not compile. The output passes the leaf to the last level's reference factory, so a leaf that is a different type fails compilation at that reference line, in the generated file.

With `from=`, the leaf is written with the hierarchy's import names and a taken last level with its own constant's. The written-out form then has both import lines and accepts exactly what the shared form accepts, so the byte-for-byte comparison needs no exception. Inputs refused today only because of how the leaf or the last level is spelled become accepted: through two import names of one package, through local aliases, with other spacing in an alias, or with parentheses, such as `(T)` for `T`. Nothing accepted today changes.

### Component lines at one level

- Component lines with the same path at one level merge into one slot, whether they come from the level's own lines or from parts, exactly as in the written-out form. Today's parser merges them within one constant.
- Two provider types for one slot, from lines in different constants, are reported at the later line in written-out order, naming both constants, for example: `component confirm is declared with provider types confirmation, in confirmationPart, and plainConfirmation, in confirmButtonSpec`. Provider types are compared as written.
- A conflict between two lines of the same constant stays a reading problem, with today's message.
- The probe of a merged slot names each constant that holds its lines once, in written-out order, as the probes' comments say.

### Rules for uses

- `from=` and `use` resolve within one package clause. A constant of the package's external test package, or the other way round, is refused, because neither can see the other's constants.
- A cycle of `from=`, of a chain with itself or between several, is refused.
- An unknown name, a `use` of a hierarchy or chain, a `from=` of a part, and a duplicate name are refused. An unknown name is refused also in a chain or part that nothing uses.
- Each constant that a hierarchy takes lines from, directly or not, must be compiled in every build that compiles the hierarchy's output, because the output's assertions name it. It must be declared in the hierarchy's own file, in a non-test file without build constraints or a platform suffix, or in a file with exactly the conditions of the hierarchy's file, compared as written. That is the rule S10 already applies to the repeated guards of existing interfaces. A test file may therefore take lines from a non-test file, but not the other way round.
- The same rule applies to a chain's own uses, through `from=` as well as `use`: a chain may name only a constant compiled in every build that compiles the chain's own file. So an ordinary chain that uses a part, or takes levels with `from=`, from a test file or from a file some builds leave out is refused at that line. The reasons:
  - the error comes as soon as the chain is written;
  - it does not depend on which hierarchies exist;
  - it lets every build check stop at direct uses;
  - a chain's own file shows who can take its lines: any hierarchy whose every build also compiles that file.
- Direct uses are enough because the comparison is transitive. If a file is compiled in every build of a second file, and the second in every build of a third, the first is compiled in every build of the third. With every chain's own uses checked, a hierarchy's own `from=` and `use` are the only ones its check needs.
- Every build-rule error is reported at the line that names the dependency: the `from=` or `use` line of the hierarchy or chain whose file some builds would compile without it.
- A chain or part that nothing uses is allowed. Its lines are checked as any constant's lines are, including the names it uses, but its registrations are compiled only in the outputs that take them.
- Problems found by the checks that run per constant are reported even in a chain or part that nothing uses, including a provider conflict in a merged slot of an unused chain. The checks that run per hierarchy never see an unused chain.

### Diagnostics

A diagnostic keeps the position of the line it is about, in that line's own constant.
- **One line alone:** a grammar error, an undeclared type, an unknown name or an unused import is reported once, for the constant that holds the line, whether or not anything uses it. An import line whose name equals a package-level name is also reported once, for its own constant, whether or not anything takes it, at the constant's position, as today.
- **Build rules:** an error is reported at the line that names the dependency, as the rules for uses say.
- **Combinations:** some problems appear only in a hierarchy that takes the line, such as a forwarder that would resolve another interface, a repeated guard that some build cannot compile, or an import name bound to two paths in one file. They are reported at the taken line, naming the hierarchy that took it and the names it went through.
- **Which problem comes first:** the generator reports one problem per run and then stops, and nothing is written, as today. That problem is the first one the generator finds. Its steps run one after another, in this order, which is today's order with one new step:
  1. Reading each file, in file order: its Go syntax and build-constraint lines; then each marked constant, in declaration order, with its grammar, the "has no level" check for a hierarchy without `from=`, the refusal of a chain with no level of its own, and its imports and type expressions; then the misplaced markers in the file's comments.
  2. New: resolving `from=` and `use`. First come the link checks, over every constant of every file, in this order:
     - duplicate names, each reported at its second declaration;
     - then the name checks, per constant in file and line order: unknown names, wrong kinds, such as a `use` that names a chain, a link to the other package clause, and a `from=` that names a hierarchy with no level of its own. This last check, reported at that `from=` line, replaces the "has no level" check for hierarchies with `from=`, which can never fail, because a chain without levels is refused while reading, and a `from=` that names a hierarchy without levels is refused by the link checks;
     - then cycles of `from=`, each reported at the first `from=` of the cycle in file and line order.

     Only when every link check passes is the list of hierarchies that take each line built. Then each chain and hierarchy, in file and line order, goes through one fixed list of checks: the build rule at its `from=` and `use` lines; the levels it takes; the component lines its levels add through `use`; then its leaf.

     The leaf comparison moves here from step 1 because the aliases it follows may be declared in any file, while a constant being read sees only the types declared before it.
  3. The operations lines: their interfaces, one operation per interface in the package, and a contract listed by two lines.
  4. The wrappers: their contracts and their holder.
  5. For each file: the names its specifications generate and import, against the package's names and each other's, and the predeclared names that generated code uses.
  6. Dot-imported names in copied signatures.
  7. Undeclared names in specification types.
  8. For each hierarchy: the forwarder check and the repeated-guard check, together.
  9. For each output file: writing it, including the check that one import name is bound to one path.

  Within a step, files and constants are taken in file and line order. The link checks pass for every constant before the per-constant list starts, so a cycle in `b.go` is reported before a build-rule error in `a.go`. A step that runs several checks for each hierarchy reports the first hierarchy, in file order, that fails any of them. So in step 8 a hierarchy that fails only the repeated-guard check is reported before a later one that fails only the forwarder check, while a hierarchy that fails step 8 is reported before one that fails step 9, even when the second comes first in file order. Added on 2026-11-24: two hierarchies with one name are duplicate names of the link checks, reported with the message of the generated-name check of step 5, so before every later problem.

  Moving the leaf comparison changes which problem is reported first for some invalid input; the exit status and the files written do not change, only the message:
  - A leaf mismatch is now reported after every problem that reading finds in a later constant or a later file, after any misplaced marker in the same file, and after every import or type-expression problem in the same constant, wherever its line is.
  - A problem in an earlier file, in an earlier constant of the same file, a grammar error in the same constant, or a Go syntax or build-constraint error anywhere in the same file, is still reported first, as today.
  - Link problems are reported before any leaf mismatch. Within the per-constant list, constants are taken in file and line order, so a problem of an earlier constant comes before a later constant's leaf mismatch. Problems found by steps 3 to 9 are still reported after it.
- **Other hierarchies that take the line:** whenever a problem found after the list is built, in the per-constant list of step 2 or later, is reported at a line that hierarchies other than the one the message names take, the message also lists them, in file and line order, after the words "also taken by". This holds for a taken line, for a hierarchy's own line that other hierarchies take, and for a chain's own line, including build-rule errors reported there. The header line of a chain, hierarchy or part counts as taken by every hierarchy that takes lines from that constant, so a build-rule error reported at a chain's `from=` gets the list. The list says nothing about whether those hierarchies were checked. It tells the user how far a fix to the line reaches. It is built only once every link check has passed, so a link problem gets no list. A problem found in step 1, while the files are read, is reported at once and without the list, as today: it must be fixed in its line whatever its reach. For example: `widget.go:52:2: widgetsChain, line 4 (taken by confirmButtonSpec through from=loggedButtons, from=buttons, from=widgets; also taken by buttonSpec, confirmMenuItemSpec, loggedButtonSpec and menuItemSpec): …`, for the chain's `node` line.
- **A problem that names no hierarchy:** when the message names no hierarchy, for example for an undeclared name in a chain or a part, it lists the hierarchies that take the line after "taken by", without "also", each with its path, and joins the entries with ", and by", for example: `confirmationPart, line 3 (taken by confirmButtonSpec through use confirming, and by confirmMenuItemSpec through use confirming): …`. The "also taken by" list, whose entries carry no path, keeps its form. A hierarchy's own line never lists that same hierarchy. Added on 2026-11-24: the rule that decides the form is which check finds the problem. A problem found for one hierarchy by its forwarder check, its repeated-guard check, or the check that each import name has one path names that hierarchy and lists the others after "also taken by"; any other problem lists every hierarchy that takes the line after "taken by", each with its path.
- **Paths:** a path through several constants spells each step as written, in the order they are followed: `through use confirming` for confirmButtonSpec's use of the part, and `through from=loggedButtons, from=buttons, from=widgets` for a path through chains and hierarchies.

## Alternatives considered

- **Guards on only the taken lines.** Each used constant's own output would declare a further constant holding only the lines that others take, and the `from=` target, and dependents would assert that instead of the whole text. The review built it by hand: after a comment-only edit to the chain, 1 output changed instead of 6, and after a partial run following an edit to a taken level, all five stale outputs failed to compile. It needs one more generated name per shared constant. It also needs a second rule for what an output depends on: the taken lines, the `from=` target, and the import lines they use. If that rule ever misses something, a stale file compiles: with the `from=` target left out of the taken text, the review changed a `from=` and a stale tree built. Rejected for that risk, in favor of whole texts, which cannot miss a change.
- **Guards on direct uses only:** a stale output passes after a run that fails partway, as question 2 shows.
- **Comparing the leaf with its last level by spelling,** as today, and documenting that the leaf must be spelled as the taken level is, with the same import name. Rejected: it refuses a valid input only because of how it is spelled.
- **Comparing package paths but local names as written,** without following aliases. Rejected: it refuses valid aliases and chains of aliases, such as `hierarchy things Btn` with `type Btn = node`. Following aliases matches interface identity and S10's reading of the leaf's holder through local aliases, and a wrong acceptance still fails to compile at the reference line.
- **Keeping today's comparison by spelling while files are read, and following aliases later.** Read literally, it refuses aliases while reading: today's check refuses `hierarchy things Btn` with `type Btn = node`, which breaks the rule. The workable reading accepts equal spellings while reading and defers the rest. Then the same cases as with the move report another problem first, because their names are local and could be aliases declared in a file not yet read. Only a mismatch between two types of other packages could still be refused while reading, since their aliases are never followed. Rejected: it gains almost nothing, at the cost of two places that compare and two order rules.
- **Reading every file before running any check.** It keeps the comparison where it is today for hierarchies without `from=`, so most invalid inputs keep today's first report. But it is not exact when a file does not parse: a type declared after a Go syntax error is lost, so with the alias after the error in `b.go`, it would report a leaf mismatch in `a.go` whose cause is in `b.go`. Fixing that means reporting the syntax error first, so that case changes anyway. It needs the reader's single loop split into two passes, and the comparison would still be needed in the new step for hierarchies with `from=`. Rejected: two places that compare and two order rules, without being exact.
- **Allowing a chain with `from=` and no level of its own,** or a `from=` that names a hierarchy with no level of its own. Either would need a new rule that asserts every constant on a `from=` path, including those that give no line, and one more assertion in every output that takes lines through it. It saves no line, and the review found no case that naming the target directly does not cover.
- **Accepting an ordinary chain that uses a test-file part, and refusing each ordinary hierarchy that takes it.** It also lets no stale file compile, and it accepts one more working program: an ordinary chain taken only by test hierarchies, which can instead move to a test file at no cost in lines. Rejected: the error would appear only once an ordinary hierarchy takes the chain, the check would have to follow every `from=` and `use` to the end, and the chain's file would not show who can take it.
- **Listing every hierarchy in which a taken line is faulty, in one run.** Every check returns at its first error today, and its results are kept on shared structures. Continuing would mean skipping later checks for each hierarchy that failed, which is the likeliest way to report false follow-on errors. Listing the other hierarchies that take the line, without saying whether each was checked, costs only the list that resolving `from=` and `use` builds anyway.
- **Holding back problems found while reading, to give them the "also taken by" list.** Reading would have to go on after a problem, keep partly read constants for their names and `from=`, and resolve `from=` and `use` over input with other problems without reporting them. That is a new code path that every user runs, and a second, silent mode of the new step. Its list cannot be right when another constant on a path has a problem: with chains `c1 from=c2` and `c2 from=c1`, a faulty line in `c1` and a hierarchy `H from=c2`, whether `H` takes the line depends on which `from=` the user removes. Rejected: a problem found while reading must be fixed in its line whatever its reach, so the list adds nothing there, and reporting at once needs no change.
- **`chain` and `from=` without parts:** 24 specification lines in 7 constants. But the confirmation component's line stays in every level that stores the component, so adding an operation to it still means editing each one.
- **Generated generic chain functions,** which dependents call, as handwritten code can do. They work, and with them a dependent's output would not change when the chain changes. Without guards in the dependents, though, a run that fails partway leaves old wrapper methods against new registrations, which gives the `UnknownCapability` panic that G1's review fix closed. With guards, dependents change whenever the chain does, so the benefit is gone, while the outputs change shape, gain new generated names, and lose the byte-for-byte comparison with the written-out form.
- **Registrations per provider type,** such as `registers <Type> <Op>...`: 32 lines. heiry lets one type register differently in different definitions ([ADR 0013](0013-optional-code-generator.md)), and the level lines would still be repeated, so inserting a level would still mean editing every specification.
- **Inclusion of whole constants:** 32 lines in 11 constants. It needs the same guards and the same import and build rules, plus rules for what an included text may hold, and one constant for each reused run of levels.
- **A shared root as a hierarchy of its own,** instead of a chain. An abstract root generates, but `Validate` reports MissingMethods for the operations that its kinds supply, so it fails the generation guide's recommended test and gives an unusable Definition.
- **No output for a file that holds only chains or parts:** it would add a second kind of specification-holding file to S10's rules for outputs, ownership and obsolete outputs.
- **Several parts in one constant, or a part beside a hierarchy:** that would need a rule for where a part's lines end. One part per constant needs none.

## Consequences

- **Code rework:** today one parsed constant is one hierarchy, and many places in the generator assume it. With `from=` and `use`, a hierarchy is assembled from lines of several constants, in a pass after every file is read, and each taken line keeps the constant it came from. The places that change:
  - `parseSpec`, which derives a level's default parent field and its inherited contracts from the previous level while it reads the line, and checks the root and the leaf for each constant. The leaf check leaves the reader and becomes the new comparison of types, with a function of its own, in the new step. The "has no level" check stays in the reader for a hierarchy without `from=`, and the refusal of a chain with no level of its own joins it there;
  - two of the parser's messages: the list of known keywords, and "a specification has a hierarchy line, an operations line, or both";
  - the places that use the hierarchy-name field `s.name` to mean "this constant holds a hierarchy", in `expected`, `existingUses`, `emitFile` and `generatedNames`. They break only if chain and part names are stored in that same field;
  - the 5 places that resolve package names through the constant's own imports (`s.qualifier`);
  - the 14 calls that report a diagnostic at the constant's positions (`lineError`), the 4 lookups of a line's position, and the per-operation line list `slot.lines`;
  - the 38 further uses of the constant's type expressions, imports and levels (`typeRefs`, `s.imports`, `s.levels`) in the parser, the analysis and the emitter;
  - `fileImport`, filled while the files are read, which stores no position: the conflict message needs one to name the import declaration of a Go file;
  - `importSet.add`, which reports a conflict at the constant's position and names only the two paths;
  - `emitFile`, which today adds every import line of every specification of the file, and must instead import what the emitted code uses;
  - `emitFreshness`, for the added assertions, and `emitProbes`, for comments that name several constants;
  - the step that resolves `from=` and `use`, which is new: it runs the link checks, then, for each chain and hierarchy, the build rule on each direct use, the levels it takes, the component lines its levels add through `use`, and the leaf comparison, and it records which hierarchies take each line, for the diagnostics.

  The risk is a place that keeps using the hierarchy's own imports or positions for a taken line: the first gives wrong types, the second wrong diagnostics. The byte-for-byte tests catch the first only when the two constants' import names differ, and never the second. The tests below cover both.
- **Order of work:** six parts, each stopping for the owner's approval. The owner commits once, at the end. Each part updates the documents its change owns, as AGENTS.md requires: S10, the architecture, the generation guide, the testing plan, the changelog and the roadmap entry. None of those edits is left to the last part.
  1. A refactor that gives each line the constant it came from, with no change in behavior. It is a pure refactor, so it has no failing test. Its evidence is the existing suite passing, the committed outputs staying byte-identical, and the comparison with today's generator, below.
  2. The leaf comparison, test first. This part creates the step that runs after reading, holding only the leaf comparison for now, so the order tests can run. The message for a taken last level belongs to part three.
  3. `chain` and `from=`, with the import rework.
  4. The "also taken by" lists.
  5. `part` and `use`.
  6. The widget measure, and a final consistency check of every document.
- **Tests:** parts two to five each start from failing tests, taken from this plan:
  - the byte-for-byte comparison with the written-out form, on several shapes. Each comparison is made in the same package and the same file, with the used constants still declared, because an output's local names avoid every package-level name and every name the other specifications of its file use, and its assertions name the used constants. The shapes include:
    - a taken line whose package or type name equals one of the output's default local names, such as `leaf`, `plan`, `heiry`, `probe`, `subject` or `generatedFrom`, because the names an output avoids are today collected from its own file's specifications only;
    - a taken constant whose untaken lines use names equal to default local names, or an import that only they use. The output must be identical to the written-out form, with no extra import. Today an output's avoided names and imports come from every specification of its file, so an implementation that extended this to whole taken constants would get it wrong;
    - a first own level after `from=` whose last taken level is qualified or generic, such as `lib.Base` or `box[int]`, because its default parent field is derived from that level, and the derivation moves from the parser to the new step;
    - a first own level that inherits a qualified contract, such as `lib.Pub`, across `from=`, because inherited contracts are copied as text today;
    - a taken registration whose existing line is only in the chain's file, because whether an output repeats a guard depends on the operations lines of the hierarchy's own file;
  - transitive freshness, including a run that fails partway, and one assertion per constant when a hierarchy reaches it twice, for example a part used at two levels;
  - every refusal: duplicate names, reported at the second declaration; unknown names and wrong kinds, also in unused chains and parts; another package clause; self and mutual cycles, reported at the first `from=` of the cycle; a chain with `omit=` or a wrapper; a chain with no level of its own, with and without `from=`, each with its message; a `from=` that names a hierarchy with no level of its own, whose message names that hierarchy's target; and misplaced `part` and `use` lines. As a control, a chain or part name equal to a Go name of the package is accepted;
  - the build rule, for a hierarchy and for a chain, through `from=` and through `use`, for a test file and for a file that some builds leave out, each error reported at the line that names the dependency; and passing controls: a file with the same conditions, and a test hierarchy taking lines from a non-test file;
  - imports: carried; conflicting within one hierarchy and between specifications of one file, with both bindings named; unused; and taken lines whose import names the hierarchy's own constant binds to another path, or does not bind at all;
  - each generated file importing only what its own code uses, including a chain beside an operations line in one constant;
  - the leaf and its last level, in the written-out and the shared forms:
    - accepted for two import names of one package, also inside type arguments, for a local alias, an alias of an alias, an alias inside a type argument, such as `box[MyInt]` against `box[int]`, an alias whose right side is spelled with spaces, and a parenthesized type;
    - refused, with the reason, for a name that is not declared, a type declared as a type of its own, an alias declared more than once, an alias with type parameters, an alias declared in another package, an alias whose right side uses a name from a dot import, a cycle of aliases, and an alias whose right side uses an import whose package name cannot be read from its path;
    - controls that must stay accepted: identical spellings in each refusal case, including an alias cycle and an unreadable import; among them `Btn` declared twice, `Btn[int]` for a generic alias, and `lib.Btn`;
    - an alias declared only in a test file, accepted, with a build that leaves the file out failing in the generated file;
    - a mismatch with a taken last level, reported at the leaf's line and naming the taken line with its constant;
  - component lines at one level: lines from the level and from parts merged into one slot, as in the written-out form; two provider types from lines in different constants, reported at the later line in written-out order and naming both constants; two from one constant, with today's message; a provider conflict in a merged slot of an unused chain, reported; and the probe comment of a merged slot, naming each constant once, also when several of the slot's lines are in one constant;
  - a position test for every diagnostic that combines lines from several constants;
  - the order of reports: one test of each kind in the rule under the diagnostics, including these cases, each of which reports the leaf mismatch first today:
    - a grammar error in a later file;
    - an unused import in the same constant;
    - a grammar error in a later constant of the same file;
    - a Go syntax error in a later file;
    - a misplaced marker later in the same file;
    - a misplaced marker earlier in the same file, since markers are checked after all of a file's constants;
    - a type expression that does not parse, in the same constant;
    - a package name with no import line, in the same constant;
    - a duplicate import name written above the `hierarchy` line;
    - a hierarchy with no level, in a later file;
    - two `//go:build` lines in a later file;
    - a marked constant that is not one string literal, in a later file;
  - a faulty line taken by several hierarchies: the problem found first in the order of the steps, and within a step in file and line order, with the other hierarchies that take the line listed after "also taken by". This includes two hierarchies that fail different checks: in step 8, the first in file order is reported whichever of the two checks it fails; across steps 8 and 9, the hierarchy that fails step 8 is reported even when it comes later in file order. It also includes a link problem in a later file, reported before a build-rule error in an earlier one, and without the list;
  - the lists: "also taken by" on a hierarchy's own line that other hierarchies take, and on a chain's own line, including a build-rule error at the chain's header line; "taken by", without "also" and with each path, for an undeclared name in a chain or a part, found in step 7; a hierarchy's own line never listing that hierarchy; and no list for a link problem or for a problem found while reading, such as a grammar error in a chain's line;
  - the probes on taken levels, and their comments: the constant that holds each level or component line; for the first level of a constant whose levels follow another constant's, both constants, among taken levels and between taken and own levels; and for a component that came through `use`, both the part's constant and the constant of the `use` line;
  - a checked-in fixture that uses `chain`, `from=`, `part` and `use`, added in part three, extended in part five, and added to the gate lists in [the testing plan](../testing.md) and `.github/workflows/test.yml`;
  - check mode and obsolete outputs, and the committed outputs, which must stay byte-identical.
- **Comparison with today's generator:** run at the end of each part, with its result recorded in the roadmap. It is not a permanent test.
  - The inputs are those of the generator's test suite as it is before part one starts.
  - Before part one, a reference generator is built from the working tree as it is then, and its binary is kept outside the repository.
  - The inputs are collected with a temporary hook in the test helpers, not checked in, that copies each test directory before generation, runs the reference binary on the copy, and compares the result.
    - The suite runs the generator in process through `gen()` in `harness_test.go`. Outside the excluded files, only `spec_test.go` and `fixtures_test.go` call `Generate` or `Check` directly, once each; those two calls are included.
    - The tests that inject file-system faults or test file handling are left out: `publish_test.go`, `write_test.go`, `case_test.go`, `files_test.go` and `lineend_test.go`.
    - `workflow_test.go` runs the command through `go generate` as a separate process, so the hook does not see it. That is accepted.
  - Both generation runs and `-check` runs are compared. Success is compared with refusal, not exit codes: in-process calls return 1 where the binary returns 3.
  - Before messages and findings are compared, the copy's directory is replaced with the original's, and the binary's `heirygen: ` prefix, which `main.go` adds, is removed. Error positions and `-check` findings name full file paths, so without this every message would differ.
  - Generation runs that succeed must give identical files, and check runs must report the same findings. Acceptance and messages may differ only where this record says they change, such as the newly accepted leaf spellings and the changed order of reports.
- **Measure of success:** measured on the implemented generator, with a copy of the widget program rewritten in the shared form. The program has 39 specification lines today, and this record counts 26 for the shared form. The report gives:
  - the specification lines, and the source lines including the constant declarations;
  - the constants edited per change: adding a root operation, 5 today, and changing the confirmation component, 2 today;
  - the outputs: their generated lines, `go vet`, the experiment's acceptance tests, and their equality with the written-out outputs apart from the stated exceptions.
- **Generated size:** measured on a scratch copy of the widget program, with the outputs built by hand by this record's rules, compiled, vetted, and run against the experiment's acceptance tests. The written-out outputs, generated by this tree, have 1,064 lines. The shared form has 1,237: 173 more, about 16%. That count includes the new output of the part's file, which holds its assertion, and the chain's assertion in the operations file's output. Dependents keep their full declarations and carry the texts of the constants they use.
- **Documents:** S10 gains the three forms, going from eight line forms to eleven, and the `from=` option. Its Input, Output, freshness guard and unsupported inputs change as described above, and so do its rules for the leaf and for imports. The architecture's parser, analysis, emitter and freshness entries change with them. So do the generation guide's grammar table, its sections "What the compiler catches, and what it cannot" and "Supported and unsupported inputs", a new section on sharing, and its troubleshooting rows. The testing plan's generator acceptance changes too, and its gate lists, like those of `.github/workflows/test.yml`, gain the new fixture. The [changelog](../../CHANGELOG.md) records the change under "Unreleased". ADR 0013's sentence that lists the grammar gains a pointer to this record.
- **Who is affected:** for authors who use none of the new lines, unchanged specifications generate the same output as before. A leaf and a last level that name one type are now accepted when they spell it differently: through two import names of one package, through local aliases, with other spacing in an alias, or with parentheses, such as `(T)` for `T`. Invalid input may report a different problem first, as the diagnostics describe. Added on 2026-11-24: a wrapper is now generated for a parenthesized leaf, also when the parentheses are on an alias's right side or around the struct the leaf is declared as, where it was refused before. Added on 2026-11-24: two hierarchies with one name, an input without the new lines, are now reported among the link checks, with the same message as before, so before problems that later checks find.

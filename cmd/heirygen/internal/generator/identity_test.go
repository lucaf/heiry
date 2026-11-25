package generator

import (
	"strings"
	"testing"
)

// twoFamilies holds two unrelated hierarchies in one package. Both have an
// operation whose method is called Name, with different signatures.
const twoFamilies = `package main

import (
	"errors"
	"fmt"

	"github.com/lucaf/heiry"
)

type (
	PetPublic  interface{ Name() string }
	PetPrivate interface{}
)

type (
	FilePublic  interface{ Name(dir string) string }
	FilePrivate interface{}
)

type pet struct{ heiry.Node }

func (p *pet) Name() string { return "rex" }

type file struct{ heiry.Node }

func (f *file) Name(dir string) string { return dir + "/notes" }

SPEC

func main() {
	p, f := &pet{}, &file{}
	if err := errors.Join(petsInit(p), filesInit(f)); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println((&petView{state: p}).Name(), (&fileView{state: f}).Name("tmp"))
	// The two operations are different keys: a pet has no file operation.
	defer func() {
		var diagnostic *heiry.Error
		if err, ok := recover().(error); ok && errors.As(err, &diagnostic) {
			fmt.Println(diagnostic.Code)
		}
	}()
	p.Node.Virtual[FileNameOp]()
}
`

func familiesSpec(petOps, fileOps string) string {
	return "//heiry:spec\nconst petsSpec = `\n" + petOps + "hierarchy pets pet\nlevel pet public=PetPublic private=PetPrivate\n\tnode PetNameOp\nwrapper petView\n`\n\n" +
		"//heiry:spec\nconst filesSpec = `\n" + fileOps + "hierarchy files file\nlevel file public=FilePublic private=FilePrivate\n\tnode FileNameOp\nwrapper fileView\n`"
}

// Equal method names with different signatures need different operation
// interfaces. One operations line gives one interface per method name, so
// the two families take a line each, with different names.
func TestEqualMethodNamesInTwoHierarchies(t *testing.T) {
	module := func(spec string) string {
		return tempModule(t, map[string]string{"main.go": strings.Replace(twoFamilies, "SPEC", spec, 1)})
	}

	dir := module(familiesSpec("operations PetPublic PetPrivate prefix=Pet suffix=Op\n", "operations FilePublic FilePrivate prefix=File suffix=Op\n"))
	mustGen(t, dir)
	mustBuild(t, dir)
	if got := mustRun(t, dir); got != "rex tmp/notes\nUnknownCapability" {
		t.Fatalf("output = %q", got)
	}

	// One line for both families is refused: the signatures differ.
	dir = module(familiesSpec("operations PetPublic PetPrivate FilePublic FilePrivate prefix=Pet suffix=Op\n", ""))
	if exit, message := gen(dir); exit != 1 || !strings.Contains(message, "one operations line gives one interface per method name") {
		t.Fatalf("one line for both: exit %d:\n%s", exit, message)
	}

	// Two lines that produce the same interface name are refused.
	dir = module(familiesSpec("operations PetPublic PetPrivate suffix=Op\n", "operations FilePublic FilePrivate suffix=Op\n"))
	if exit, message := gen(dir); exit != 1 || !strings.Contains(message, "would generate NameOp") {
		t.Fatalf("same generated name: exit %d:\n%s", exit, message)
	}

	// A contract listed twice would get two interfaces for one operation.
	dir = module(familiesSpec("operations PetPublic PetPrivate prefix=Pet suffix=Op\n", "operations FilePublic FilePrivate prefix=File suffix=Op\noperations PetPublic prefix=Other suffix=Op\n"))
	if exit, message := gen(dir); exit != 1 || !strings.Contains(message, "contract PetPublic is listed by two operations lines") {
		t.Fatalf("contract listed twice: exit %d:\n%s", exit, message)
	}

	// A name is needed, and it has to be an identifier.
	dir = module(familiesSpec("operations PetPublic PetPrivate\n", "operations FilePublic FilePrivate prefix=File suffix=Op\n"))
	if exit, message := gen(dir); exit != 1 || !strings.Contains(message, "give prefix= or suffix=") {
		t.Fatalf("no naming: exit %d:\n%s", exit, message)
	}
}

// sharedOps has two definitions over one leaf type. They share the
// contracts, the bodies, and therefore the operation interfaces; each has
// its own registrations, helpers, and wrapper.
const sharedOps = `package main

import (
	"fmt"

	"github.com/lucaf/heiry"
)

type (
	Public  interface{ Describe() string }
	Private interface{ Detail() string }
)

type base struct{ heiry.Node }

func (b *base) Describe() string { return "base(" + b.Node.Virtual[DetailOp]().Detail() + ")" }
func (b *base) Detail() string   { return "plain" }

type leaf struct {
	base
	heiry.Node
}

func (l *leaf) Describe() string { return "leaf > " + l.Node.Parent[DescribeOp]().Describe() }
func (l *leaf) Detail() string   { return "fine" }

//heiry:spec
const fullSpec = ` + "`" + `
operations Public Private suffix=Op
hierarchy full leaf
level base public=Public private=Private
	node DescribeOp DetailOp
level leaf
	node DescribeOp DetailOp
wrapper fullView
` + "`" + `

//heiry:spec
const plainSpec = ` + "`" + `
hierarchy plain leaf
level base public=Public private=Private
	node DescribeOp DetailOp
level leaf
wrapper plainView
` + "`" + `

func main() {
	a, b := &leaf{}, &leaf{}
	if err := fullInit(a); err != nil {
		fmt.Println("error:", err)
		return
	}
	if err := plainInit(b); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println((&fullView{state: a}).Describe())
	fmt.Println((&plainView{state: b}).Describe())
	fmt.Println(plainInitPlan(full.MustPrepare(), &leaf{}) != nil)
}
`

func TestOperationsSharedByDefinitionsOverOneLeaf(t *testing.T) {
	dir := tempModule(t, map[string]string{"main.go": sharedOps})
	mustGen(t, dir)
	mustBuild(t, dir)
	if got := mustRun(t, dir); got != "leaf > base(fine)\nbase(plain)\ntrue" {
		t.Fatalf("output = %q", got)
	}
}

// existingOps keeps two handwritten operation interfaces, one of them from
// another package, and generates the third.
const existingOps = `package main

import (
	"fmt"

	"github.com/lucaf/heiry"
)

type Namer interface{ Name() string }

type (
	Public interface {
		String() string
		Name() string
		Size() int
	}
	Private interface{}
)

type thing struct{ heiry.Node }

func (t *thing) String() string { return "thing " + t.Node.Virtual[Namer]().Name() }
func (t *thing) Name() string   { return "one" }
func (t *thing) Size() int      { return 1 }

//heiry:spec
const thingSpec = ` + "`" + `
import fmt "fmt"
operations Public Private suffix=Op
	existing String fmt.Stringer
	existing Name Namer
hierarchy things thing
level thing public=Public private=Private
	node fmt.Stringer Namer SizeOp
wrapper view
` + "`" + `

func main() {
	t := &thing{}
	if err := thingsInit(t); err != nil {
		fmt.Println("error:", err)
		return
	}
	var consumer fmt.Stringer = &view{state: t}
	fmt.Println(consumer, (&view{state: t}).Size(), t.Node.Virtual[SizeOp]().Size())
}
`

func TestExistingOperationInterfaces(t *testing.T) {
	dir := tempModule(t, map[string]string{"main.go": existingOps})
	mustGen(t, dir)
	mustBuild(t, dir)
	if got := mustRun(t, dir); got != "thing one 1 1" {
		t.Fatalf("output = %q", got)
	}
	generated := read(t, dir, mainOutput)
	if strings.Contains(generated, "StringOp") || strings.Contains(generated, "NameOp") || !strings.Contains(generated, "SizeOp interface{ Size() int }") {
		t.Fatalf("existing operations were generated, or the generated one is missing:\n%s", generated)
	}

	// The generator takes the existing interface on trust. The compiler
	// checks it: an interface with another signature does not fit the
	// contract.
	edit(t, dir, "main.go", "type Namer interface{ Name() string }", "type Namer interface{ Name(full bool) string }")
	buildFails(t, dir, "(wrong type for method Name)")

	for _, tc := range []struct{ name, old, new, want string }{
		{"existing line for an unknown method", "\texisting Name Namer\n", "\texisting Name Namer\n\texisting Missing Namer\n", "no listed contract has a method Missing"},
		{"existing line outside operations", "hierarchy things thing\n", "hierarchy things thing\n\texisting Name Namer\n", "an existing line directly follows an operations line"},
		{"embedded interface of another package", "\t\tString() string\n", "\t\tfmt.Stringer\n", "embeds fmt.Stringer, whose methods cannot be read from this package"},
		{"wrapper without operations", "operations Public Private suffix=Op\n\texisting String fmt.Stringer\n\texisting Name Namer\n", "", "which no operations line of this package lists"},
		{"contract that is not declared", "operations Public Private suffix=Op\n", "operations Public Private Absent suffix=Op\n", "contract Absent is not declared"},
		{"contract that is not an interface", "operations Public Private suffix=Op\n", "operations Public Private thing suffix=Op\n", "the generator reads only interfaces declared in this package"},
		{"unexported operation", "\t\tSize() int\n", "\t\tSize() int\n\t\thidden()\n", "operation hidden is not exported"},
	} {
		dir := tempModule(t, map[string]string{"main.go": existingOps})
		edit(t, dir, "main.go", tc.old, tc.new)
		if tc.name == "wrapper without operations" {
			edit(t, dir, "main.go", "import fmt \"fmt\"\n", "")
			edit(t, dir, "main.go", "\tnode fmt.Stringer Namer SizeOp\n", "\tnode Namer\n")
		}
		if exit, message := gen(dir); exit != 1 || !strings.Contains(message, tc.want) {
			t.Errorf("%s: exit %d, want a message containing %q:\n%s", tc.name, exit, tc.want, message)
		}
	}
}

// A handwritten interface that duplicates a generated one is a second key
// for the same method. Nothing generated can see it. The library reports it
// exactly as it does for two handwritten interfaces: at validation when both
// are registered, and at the call when a body resolves the unregistered one.
func TestHandwrittenDuplicateOfAGeneratedOperation(t *testing.T) {
	app := strings.Replace(sharedOps, "type base struct{ heiry.Node }", "type Detailer interface{ Detail() string }\n\ntype base struct{ heiry.Node }", 1)
	app = strings.Replace(app, `	fmt.Println((&fullView{state: a}).Describe())
	fmt.Println((&plainView{state: b}).Describe())
	fmt.Println(plainInitPlan(full.MustPrepare(), &leaf{}) != nil)
`, `	defer func() {
		var diagnostic *heiry.Error
		if err, ok := recover().(error); ok && errors.As(err, &diagnostic) {
			fmt.Println("at the call:", diagnostic.Code)
		}
	}()
	both := heiry.Define[leaf]().
		Level[base, Public, Private]("").
		Bind[DescribeOp]((*base)(nil), ".").
		Bind[DetailOp]((*base)(nil), ".").End().
		Level[leaf, Public, Private]("base").
		Bind[Detailer]((*leaf)(nil), ".").End()
	var diagnostic *heiry.Error
	if errors.As(both.Validate(), &diagnostic) {
		fmt.Println("both registered:", diagnostic.Code)
	}
	a.Node.Virtual[Detailer]()
`, 1)
	app = strings.Replace(app, "import (\n\t\"fmt\"\n", "import (\n\t\"errors\"\n\t\"fmt\"\n", 1)
	dir := tempModule(t, map[string]string{"main.go": app})
	mustGen(t, dir)
	mustBuild(t, dir)
	if got := mustRun(t, dir); got != "both registered: CapabilityIdentity\nat the call: UnknownCapability" {
		t.Fatalf("output = %q", got)
	}
}

// Generated operation interfaces and wrapper types are package-level names
// too: one that the application already declares is reported before
// anything is written.
func TestGeneratedTypeNameCollisions(t *testing.T) {
	for name, declaration := range map[string]string{
		"AuditOp": "type AuditOp interface{ Audit() string }",
		"view":    "type view struct{}",
	} {
		dir := tempModule(t, map[string]string{"main.go": fullApp, "probe.go": fullProbe, "other.go": "package main\n\n" + declaration + "\n"})
		exit, message := gen(dir)
		if exit != 1 || !strings.Contains(message, "would generate "+name) || !strings.Contains(message, "other.go") {
			t.Fatalf("%s: exit %d:\n%s", name, exit, message)
		}
	}
}

// growingContract has a private contract that grows at the leaf. The later
// contract embeds the earlier one, both are listed, and each method still
// gets one operation interface. The wrapper's guard uses the leaf's
// contract, so it names every private operation.
const growingContract = `package main

import (
	"fmt"

	"github.com/lucaf/heiry"
)

type (
	Public  interface{ Describe() string }
	Private interface{ Detail() string }
	// LeafPrivate adds an operation at the leaf level only.
	LeafPrivate interface {
		Private
		Limit() int
	}
)

type base struct{ heiry.Node }

func (b *base) Describe() string { return "base(" + b.Node.Virtual[DetailOp]().Detail() + ")" }
func (b *base) Detail() string   { return "plain" }

type leaf struct {
	base
	heiry.Node
}

func (l *leaf) Detail() string { return fmt.Sprint("limit ", l.Node.Virtual[LimitOp]().Limit()) }
func (l *leaf) Limit() int     { return 7 }

//heiry:spec
const growingSpec = ` + "`" + `
operations Public Private LeafPrivate suffix=Op
hierarchy growing leaf
level base public=Public private=Private
	node DescribeOp DetailOp
level leaf public=Public private=LeafPrivate
	node DetailOp LimitOp
wrapper view
` + "`" + `

func main() {
	l := &leaf{}
	if err := growingInit(l); err != nil {
		fmt.Println("error:", err)
		return
	}
	var consumer any = &view{state: l}
	_, detail := consumer.(DetailOp)
	_, limit := consumer.(LimitOp)
	fmt.Println((&view{state: l}).Describe(), detail, limit)
}
`

func TestContractThatGrowsAtALaterLevel(t *testing.T) {
	dir := tempModule(t, map[string]string{"main.go": growingContract})
	mustGen(t, dir)
	mustBuild(t, dir)
	if got := mustRun(t, dir); got != "base(limit 7) false false" {
		t.Fatalf("output = %q", got)
	}
	generated := read(t, dir, mainOutput)
	if strings.Count(generated, "interface{ Detail() string }") != 1 {
		t.Fatalf("Detail does not have exactly one operation interface:\n%s", generated)
	}

	// The guard covers the operation that only the leaf's contract has.
	write(t, dir, "extra.go", "package main\n\nfunc (v *view) Limit() int { return 0 }\n")
	output := buildFails(t, dir, "ambiguous selector probe.Limit")
	reportedIn(t, output, mainOutput, "ambiguous selector probe.Limit")

	// A method added to the embedded contract changes both listed
	// contracts, and both assertions notice.
	dir = tempModule(t, map[string]string{"main.go": growingContract})
	mustGen(t, dir)
	edit(t, dir, "main.go", "Private interface{ Detail() string }", "Private interface {\n\t\tDetail() string\n\t\tAudit()\n\t}")
	output = buildFails(t, dir, "does not implement Private (missing method Audit)")
	reportedIn(t, output, mainOutput, "does not implement Private (missing method Audit)")
	reportedIn(t, output, mainOutput, "does not implement LeafPrivate (missing method Audit)")
}

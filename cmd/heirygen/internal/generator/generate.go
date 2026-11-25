// Package generator implements the heirygen command: it reads heiry
// specifications from marked Go string constants and writes ordinary Go
// source that uses the heiry public API. Specification S10 and the
// generator section of the architecture describe what it does and how.
package generator

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// testFunction reports whether go test treats a function of this name in a
// test file as a test, benchmark, fuzz target, or example: the prefix is
// followed by nothing or by a rune that is not a lower-case letter.
func testFunction(name string) bool {
	for _, prefix := range []string{"Test", "Benchmark", "Fuzz", "Example"} {
		rest, found := strings.CutPrefix(name, prefix)
		if !found {
			continue
		}
		if rest == "" {
			return true
		}
		if r, _ := utf8.DecodeRuneInString(rest); !unicode.IsLower(r) {
			return true
		}
	}
	return false
}

// specMarker is the doc-comment line that marks a specification constant.
const specMarker = "//heiry:spec"

// A sourceFile is one handwritten file that holds at least one spec.
type sourceFile struct {
	name            string // base name
	pkg             string // package clause
	buildConstraint string // the //go:build line the output needs, if any
	specs           []*spec
}

// A pkgInfo collects what the handwritten files of one package clause
// declare at package level.
type pkgInfo struct {
	taken map[string]string // identifier -> position of its declaration
	index *packageIndex
	// ops holds the operations of every operations line of the package, in
	// file name order and then line order. Walks that can reach a diagnostic
	// or the output use it, never a map, so that every run gives the same
	// result.
	ops []operation
	// declaredIn lists the files that declare each package-level name; a
	// generated name is listed under the source file whose output has it.
	declaredIn map[string][]string
	builds     map[string]fileBuild // file name -> the builds that compile it
}

// A fileBuild describes the builds that compile a file, in the terms the
// generator compares: whether it is a test file, the //go:build line its
// output needs, and the platform its name selects.
type fileBuild struct {
	test       bool
	constraint string // with cgo for a file that imports "C"; "" for none
	platform   string // the name's last two parts that go/build reads; "" for none
}

// compiledWith reports whether every build that compiles a file with build
// b compiles this file too. It only compares: either this file has no
// condition at all, or it has exactly the conditions of the other. Files
// whose conditions differ but overlap count as not compiled together.
func (f fileBuild) compiledWith(b fileBuild) bool {
	if f.test && !b.test {
		return false
	}
	if f.constraint == "" && f.platform == "" {
		return true
	}
	return f.constraint == b.constraint && f.platform == b.platform
}

// platformOf returns what a file name selects for GOOS and GOARCH as
// go/build reads it, or "" when it selects nothing. A build context with no
// real platform rejects exactly the names that select one.
func platformOf(name string) string {
	ctxt := build.Context{GOOS: "none", GOARCH: "none", Compiler: "gc", OpenFile: func(string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("package p\n")), nil
	}}
	if match, err := ctxt.MatchFile("", name); err == nil && match {
		return ""
	}
	stem, _, _ := strings.Cut(name, ".")
	cut := strings.Index(stem, "_")
	if cut < 0 {
		return stem
	}
	parts := strings.Split(strings.TrimSuffix(stem[cut:], "_test"), "_")
	return strings.Join(parts[max(0, len(parts)-2):], "_")
}

// expected returns the outputs the generator writes for the handwritten
// files of dir, keyed by base name.
func expected(dir string, files []string, contents map[string][]byte) (map[string][]byte, error) {
	sources, packages, err := loadSources(dir, files, contents)
	if err != nil {
		return nil, err
	}
	if err := resolve(sources, packages); err != nil {
		return nil, err
	}
	if err := analyze(sources, packages); err != nil {
		return nil, err
	}
	claimed := make(map[string]map[string]string)
	for _, source := range sources {
		if claimed[source.pkg] == nil {
			claimed[source.pkg] = make(map[string]string)
		}
		taken := packages[source.pkg].taken
		for _, s := range source.specs {
			for _, name := range s.generatedNames() {
				if where, exists := taken[name]; exists {
					return nil, errorAt(s.pos, "%s would generate %s, which %s already declares", s.constName, name, where)
				}
				if where, exists := claimed[source.pkg][name]; exists {
					return nil, errorAt(s.pos, "%s would generate %s, which the specification at %s already generates", s.constName, name, where)
				}
				claimed[source.pkg][name] = s.pos.String()
				packages[source.pkg].declaredIn[name] = append(packages[source.pkg].declaredIn[name], source.name)
			}
			if strings.HasSuffix(source.name, "_test.go") && s.name != "" {
				for _, name := range s.helpers() {
					if testFunction(name) {
						return nil, errorAt(s.pos, "%s would generate the function %s in a test file, which go test treats as a test, benchmark, fuzz target, or example; rename hierarchy %s", s.constName, name, s.name)
					}
				}
			}
			for _, declared := range s.imports {
				if where, exists := taken[declared.name]; exists {
					return nil, errorAt(s.pos, "%s imports a package as %s, which %s already declares", s.constName, declared.name, where)
				}
			}
		}
		// Generated code relies on these predeclared identifiers. The
		// guards themselves use none that a package could redeclare to
		// disarm them.
		for _, name := range []string{"error", "nil", "new"} {
			if where, exists := taken[name]; exists {
				return nil, errorAt(token.Position{}, "%s redeclares %s, which generated code uses with its predeclared meaning", where, name)
			}
		}
	}
	if err := checkDotImports(sources, packages, claimed); err != nil {
		return nil, err
	}
	if err := checkSpecNames(sources, packages, claimed); err != nil {
		return nil, err
	}
	// With every name declared and distinct, each forwarder must resolve
	// what its hierarchy registers.
	for _, source := range sources {
		for _, s := range source.specs {
			if s.wrapper != nil {
				if err := canonicalDispatch(s, packages[source.pkg].index, packages[source.pkg].ops); err != nil {
					return nil, err
				}
			}
			uses, err := existingUses(source, s, packages[source.pkg])
			if err != nil {
				return nil, err
			}
			s.existingUses = uses
		}
	}
	outputs := make(map[string][]byte)
	for _, source := range sources {
		// The file must stay clear of the handwritten package-level names
		// and of the names generated for every file of the package.
		names := make(map[string]bool)
		for name := range packages[source.pkg].taken {
			names[name] = true
		}
		for name := range claimed[source.pkg] {
			names[name] = true
		}
		content, err := emitFile(source, names)
		if err != nil {
			return nil, err
		}
		outputs[outputName(source.name)] = content
	}
	return outputs, nil
}

// analyze reads the contracts of every operations line and records their
// operations in each package's ops, then gives each wrapper the operations
// of its leaf's contracts.
func analyze(sources []*sourceFile, packages map[string]*pkgInfo) error {
	covered := make(map[string]map[string][]operation)
	// The first declaration of each interface, per package clause: one
	// interface identifies one method in all of the package's input,
	// whichever file or build holds a line, or no build could check them all.
	identifies := make(map[string]map[string]operation)
	for _, source := range sources {
		if covered[source.pkg] == nil {
			covered[source.pkg] = make(map[string][]operation)
		}
		for _, s := range source.specs {
			for _, set := range s.operations {
				if err := packages[source.pkg].index.resolveOperations(s, set); err != nil {
					return err
				}
				for index := range set.ops {
					set.ops[index].origin = source.name
				}
				for contract, ops := range set.contractOps {
					for index := range ops {
						ops[index].origin = source.name
					}
					set.contractOps[contract] = ops
				}
				if identifies[source.pkg] == nil {
					identifies[source.pkg] = make(map[string]operation)
				}
				for _, op := range set.ops {
					first, known := identifies[source.pkg][op.key]
					if !known {
						identifies[source.pkg][op.key] = op
						continue
					}
					if first.method.name != op.method.name {
						return errorAt(op.pos, "%s names %s for method %s, but %s: %s names it for method %s; one interface identifies one operation in the whole package, whichever file or build holds the line: correct one of the two", op.decl, op.typ, op.method.name, first.pos, first.decl, first.method.name)
					}
				}
				packages[source.pkg].ops = append(packages[source.pkg].ops, set.ops...)
				for _, contract := range set.contracts {
					if _, twice := covered[source.pkg][contract]; twice {
						return s.lineError(set.line, "contract %s is listed by two operations lines; one interface identifies an operation", contract)
					}
					covered[source.pkg][contract] = set.contractOps[contract]
				}
			}
		}
	}
	for _, source := range sources {
		for _, s := range source.specs {
			w := s.wrapper
			if w == nil {
				continue
			}
			leaf := s.levels[len(s.levels)-1]
			w.public, w.private = leaf.public, leaf.private
			// The wrapper forwards the public contract and must not carry
			// the names of the private one. Both have to be listed, because
			// only a listed contract gets the equality assertion that turns
			// a later change into a compile error.
			for _, contract := range []string{w.public, w.private} {
				if _, known := covered[source.pkg][contract]; !known {
					return s.lineError(w.line, "wrapper %s depends on %s, which no operations line of this package lists", w.name, contract)
				}
			}
			w.forwards = covered[source.pkg][w.public]
			w.privateOps = covered[source.pkg][w.private]
			// The forwarders dispatch through the leaf's directly embedded
			// Node, the holder the library installs. Any other field named
			// as holder could be a Node that is never installed, or a
			// pointer to another object's, so the wrapper is written only
			// when the generator establishes the holder.
			nodes, reason := packages[source.pkg].index.embeddedNodes(s.leaf, s.qualifier)
			if reason != "" {
				return s.lineError(w.line, "wrapper %s: the generator cannot establish the directly embedded heiry.Node of the leaf: %s; remove the wrapper line and write the wrapper by hand; the declaration and helpers are generated without it", w.name, reason)
			}
			if nodes[0] != w.holder {
				given := "holder=" + w.holder
				if !w.holderGiven {
					given = "the default holder=" + w.holder
				}
				return s.lineError(w.line, "%s names a field that %s does not embed; its directly embedded heiry.Node is %s: write holder=%s", given, s.leaf, nodes[0], nodes[0])
			}
		}
	}
	return nil
}

// existingUses lists the existing interfaces that a hierarchy's
// registrations and its wrapper's forwarders use, other than those an
// operations line of the same source file names. The guard that ties each to
// its one method is in the operations line's output, which some builds that
// compile these registrations may leave out, so their output repeats it.
//
// An operations line of the hierarchy's own file guards each existing
// interface as that line spells it, in the same output. A use with that
// spelling needs no other guard. A use spelled otherwise, such as through a
// local alias, resolved to the interface only when the file was generated,
// so it gets its own guard.
//
// The guard copies the signature of one declaration of the interface, and
// every build that compiles the output must have the names it uses. The
// hierarchy's own file's declaration is taken when there is one; otherwise
// the first, in file and line order, whose unqualified names are declared in
// files that every such build compiles, or predeclared. With none, the
// registration is refused.
func existingUses(source *sourceFile, s *spec, pkg *pkgInfo) ([]existingUse, error) {
	if s.name == "" {
		return nil, nil
	}
	declarations := make(map[string][]operation) // type key -> declarations, in file and line order
	guarded := make(map[string]bool)             // expressions this file's output guards
	for _, op := range pkg.ops {
		if op.generated {
			continue
		}
		if op.origin == source.name {
			guarded[op.typ] = true
		}
		declarations[op.key] = append(declarations[op.key], op)
	}
	receiving := pkg.builds[source.name]
	// missingName returns a name of the signature that some build of the
	// output lacks, or "".
	missingName := func(m method) string {
		for _, name := range m.unqualified {
			files := pkg.declaredIn[name]
			available := len(files) == 0 && predeclared[name]
			for _, file := range files {
				available = available || file == source.name || pkg.builds[file].compiledWith(receiving)
			}
			if !available {
				return name
			}
		}
		return ""
	}
	// representative returns the declaration whose signature the guard
	// copies, or, when no declaration fits, the first one and the name its
	// signature lacks.
	representative := func(key string) (operation, string) {
		for _, op := range declarations[key] {
			if op.origin == source.name {
				return op, ""
			}
		}
		for _, op := range declarations[key] {
			if missingName(op.method) == "" {
				return op, ""
			}
		}
		first := declarations[key][0]
		return first, missingName(first.method)
	}
	var uses []existingUse
	seen := make(map[string]bool)
	add := func(expr string, op operation) {
		seen[expr] = true
		uses = append(uses, existingUse{expr: expr, op: op})
	}
	for _, level := range s.levels {
		for _, slot := range level.slots {
			for index, expr := range slot.ops {
				line := slot.lines[index]
				key := pkg.index.typeKey(expr, line.qualifier)
				if len(declarations[key]) == 0 || guarded[expr] || seen[expr] {
					continue
				}
				op, missing := representative(key)
				if missing != "" {
					return nil, line.errorf("registration %s needs a guard in the output of %s that ties it to its method, but the signature of every existing line that names its interface uses a name some builds of %s lack; the first, %s: %s, has the signature at %s, which uses %s, declared only in %s: declare %s in a non-test file without build constraints or a platform suffix, or name the interface with an existing line in %s", expr, source.name, source.name, op.pos, op.decl, op.method.pos, missing, strings.Join(pkg.declaredIn[missing], ", "), missing, source.name)
				}
				add(expr, op)
			}
		}
	}
	if s.wrapper != nil {
		// A forwarder copies its operation's signature anyway, so its guard
		// copies the same one.
		for _, op := range s.wrapper.forwards {
			if !op.generated && !guarded[op.typ] && !seen[op.typ] {
				add(op.typ, op)
			}
		}
	}
	return uses, nil
}

// canonicalDispatch requires each forwarder of a wrapper to resolve the
// interface its hierarchy registers for the forwarded method. A forwarder
// resolves the interface the operations line gives the method; when the
// hierarchy registers that method under another interface, the forwarder
// would find nothing at run time. Registrations are matched to methods
// through the package's operation interfaces, or by reading a single-method
// interface declared in the package. The generator never infers a
// registration: it only compares the ones the specification lists.
//
// Operations lines that name one interface for different methods are refused
// before this check, so every declaration of an interface gives it the same
// method.
func canonicalDispatch(s *spec, ix *packageIndex, ops []operation) error {
	w := s.wrapper
	methodOf := make(map[string]string) // type key -> method, for every operation interface
	for _, op := range ops {
		if _, known := methodOf[op.key]; !known {
			methodOf[op.key] = op.method.name
		}
	}
	type registration struct {
		expr, key, method string
		line              specLine
	}
	var registrations []registration
	for _, level := range s.levels {
		for _, slot := range level.slots {
			for index, expr := range slot.ops {
				line := slot.lines[index]
				r := registration{expr: expr, key: ix.typeKey(expr, line.qualifier), line: line}
				r.method = methodOf[r.key]
				if r.method == "" {
					r.method, _ = ix.singleMethod(expr, line.qualifier)
				}
				registrations = append(registrations, r)
			}
		}
	}
	paired := make(map[[2]string]bool)
	for _, op := range w.forwards {
		name := op.method.name
		confirmed := 0
		for _, r := range registrations {
			if r.method != name {
				continue
			}
			confirmed++
			if r.key != op.key {
				return r.line.errorf("wrapper %s forwards %s through %s, but hierarchy %s registers %s as %s; one interface identifies an operation: write `existing %s %s` on the operations line that lists %s",
					w.name, name, op.typ, s.name, name, r.expr, name, r.expr, w.public)
			}
			if pair := [2]string{op.typ, r.expr}; r.expr != op.typ && !paired[pair] {
				paired[pair] = true
				w.identities = append(w.identities, pair)
			}
		}
		if confirmed != 0 {
			continue
		}
		for _, r := range registrations {
			if r.method == "" {
				return r.line.errorf("wrapper %s forwards %s through %s, but the generator cannot tell which operation %s identifies; if it is %s, write `existing %s %s` on the operations line that lists %s",
					w.name, name, op.typ, r.expr, name, name, r.expr, w.public)
			}
		}
	}
	return nil
}

// checkDotImports refuses signatures that the generated file cannot copy:
// it does not have the contract file's dot imports, so an unqualified name
// that neither the package nor the universe declares may come from one and
// would be undefined there.
func checkDotImports(sources []*sourceFile, packages map[string]*pkgInfo, claimed map[string]map[string]string) error {
	for _, source := range sources {
		declared := func(name string) bool {
			_, handwritten := packages[source.pkg].taken[name]
			_, generated := claimed[source.pkg][name]
			return handwritten || generated || predeclared[name]
		}
		for _, s := range source.specs {
			var copied []method
			// Generated interfaces, the guards of existing ones, forwarders,
			// and the wrapper's contract guards all copy signatures.
			for _, set := range s.operations {
				for _, op := range set.ops {
					copied = append(copied, op.method)
				}
			}
			if s.wrapper != nil {
				for _, op := range append(s.wrapper.forwards, s.wrapper.privateOps...) {
					copied = append(copied, op.method)
				}
			}
			for _, m := range copied {
				if !m.file.dotImport() {
					continue
				}
				for _, name := range m.unqualified {
					if !declared(name) {
						return errorAt(m.pos, "%s uses %s, which %s may get from its dot import; the generated file cannot copy that signature: import that package with a name and qualify %s", m.name, name, m.file.name, name)
					}
				}
			}
		}
	}
	return nil
}

// checkSpecNames refuses unqualified names in specification types that the
// package does not declare. Such a type would be written into the generated
// file as is and could not compile there.
func checkSpecNames(sources []*sourceFile, packages map[string]*pkgInfo, claimed map[string]map[string]string) error {
	for _, source := range sources {
		for _, s := range source.specs {
			for _, ref := range s.typeRefs {
				expr, err := parser.ParseExpr(ref.expr)
				if err != nil {
					return s.lineError(ref.line, "%q is not a Go type expression", ref.expr)
				}
				for _, name := range typeNames(expr, nil) {
					_, handwritten := packages[source.pkg].taken[name]
					_, generated := claimed[source.pkg][name]
					if !handwritten && !generated && !predeclared[name] {
						return s.lineError(ref.line, "%s is not declared in the handwritten files of package %s; declare it, or qualify it and add an import line", name, source.pkg)
					}
				}
			}
		}
	}
	return nil
}

// loadSources parses the named handwritten files of dir. It collects their
// specs and, per package clause, the package-level identifiers and the type
// declarations. Handwritten files must be syntactically valid; they need not
// type-check.
func loadSources(dir string, files []string, contents map[string][]byte) ([]*sourceFile, map[string]*pkgInfo, error) {
	fset := token.NewFileSet()
	var sources []*sourceFile
	packages := make(map[string]*pkgInfo)
	for _, name := range files {
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), contents[name], parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return nil, nil, err
		}
		pkg := file.Name.Name
		if packages[pkg] == nil {
			packages[pkg] = &pkgInfo{
				taken:      make(map[string]string),
				index:      &packageIndex{fset: fset, types: make(map[string][]*typeDecl)},
				declaredIn: make(map[string][]string),
				builds:     make(map[string]fileBuild),
			}
		}
		info := packages[pkg]
		declare := func(ident *ast.Ident) {
			if ident.Name != "_" {
				info.taken[ident.Name] = fset.Position(ident.Pos()).String()
				info.declaredIn[ident.Name] = append(info.declaredIn[ident.Name], name)
			}
		}
		fileData := &fileInfo{name: name}
		for _, imported := range file.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				continue
			}
			entry := fileImport{path: path, pos: fset.Position(imported.Pos())}
			if imported.Name != nil {
				entry.name = imported.Name.Name
			}
			fileData.imports = append(fileData.imports, entry)
		}
		constraintLine, err := buildConstraint(contents[name], file)
		if err != nil {
			return nil, nil, errorAt(fset.Position(file.Package), "%v", err)
		}
		info.builds[name] = fileBuild{test: strings.HasSuffix(name, "_test.go"), constraint: constraintLine, platform: platformOf(name)}
		source := &sourceFile{name: name, pkg: pkg, buildConstraint: constraintLine}
		specDocs := make(map[*ast.CommentGroup]bool)
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if decl.Recv == nil {
					declare(decl.Name)
				}
			case *ast.GenDecl:
				for _, declared := range decl.Specs {
					switch declared := declared.(type) {
					case *ast.TypeSpec:
						declare(declared.Name)
						info.index.types[declared.Name.Name] = append(info.index.types[declared.Name.Name], &typeDecl{spec: declared, file: fileData})
					case *ast.ValueSpec:
						for _, ident := range declared.Names {
							declare(ident)
						}
						doc := declared.Doc
						if doc == nil && len(decl.Specs) == 1 {
							doc = decl.Doc
						}
						if decl.Tok != token.CONST || !hasMarker(doc) {
							continue
						}
						specDocs[doc] = true
						s, err := constSpec(fset, declared)
						if err != nil {
							return nil, nil, err
						}
						source.specs = append(source.specs, s)
					}
				}
			}
		}
		// Any other marker is an error: only a constant carries a
		// specification, because only a constant can be checked by the
		// generated freshness assertion.
		for _, group := range file.Comments {
			for _, comment := range group.List {
				if text, found := strings.CutPrefix(comment.Text, "//"); found && strings.TrimSpace(text) == specMarker[2:] && text != specMarker[2:] {
					return nil, nil, errorAt(fset.Position(comment.Pos()), "%q is not a marker, because Go directives have no space: write %s without a space", comment.Text, specMarker)
				}
			}
			if specDocs[group] || !hasMarker(group) {
				continue
			}
			return nil, nil, errorAt(fset.Position(group.Pos()), "%s marks a specification only in the doc comment of a string constant declared alone with one string literal", specMarker)
		}
		if len(source.specs) != 0 {
			sources = append(sources, source)
		}
	}
	return sources, packages, nil
}

// buildConstraint returns the //go:build line that selects the output in
// exactly the builds that select its source: the source's //go:build line,
// or else its legacy +build lines combined as the go command combines them,
// and in either case cgo when the source imports "C", because the go command
// leaves cgo files out of builds without cgo.
func buildConstraint(content []byte, file *ast.File) (string, error) {
	goBuild, plusBuild, err := fileHeader(content)
	if err != nil {
		return "", err
	}
	var expr constraint.Expr
	and := func(x constraint.Expr) {
		if expr == nil {
			expr = x
		} else {
			expr = &constraint.AndExpr{X: expr, Y: x}
		}
	}
	if goBuild != "" {
		x, err := constraint.Parse(goBuild)
		if err != nil {
			return "", fmt.Errorf("parsing its //go:build line: %v", err)
		}
		and(x)
	} else {
		for _, line := range plusBuild {
			// The go command skips a +build line it cannot parse.
			if x, err := constraint.Parse(line); err == nil {
				and(x)
			}
		}
	}
	for _, imported := range file.Imports {
		// The path may be written as an interpreted or a raw string.
		if path, err := strconv.Unquote(imported.Path.Value); err == nil && path == "C" {
			and(&constraint.TagExpr{Tag: "cgo"})
			break
		}
	}
	if expr == nil {
		return "", nil
	}
	return "//go:build " + expr.String(), nil
}

// fileHeader finds a file's build constraint lines as go/build does. A
// //go:build line may appear anywhere among the leading comments. A legacy
// "// +build" line counts only up to the last blank line before the first
// line that is not a comment, so one in the package doc comment does not.
func fileHeader(content []byte) (goBuild string, plusBuild []string, err error) {
	// The go command removes a leading UTF-8 byte order mark first.
	content = bytes.TrimPrefix(content, []byte("\ufeff"))
	end := 0
	p := content
	ended := false       // a line that is not a // comment was seen
	inSlashStar := false // inside a /* */ comment
Lines:
	for len(p) > 0 {
		line := p
		if i := bytes.IndexByte(line, '\n'); i >= 0 {
			line, p = line[:i], p[i+1:]
		} else {
			p = p[len(p):]
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 && !ended {
			end = len(content) - len(p)
			continue Lines
		}
		if !bytes.HasPrefix(line, []byte("//")) {
			ended = true
		}
		if !inSlashStar && constraint.IsGoBuild(string(line)) {
			if goBuild != "" {
				return "", nil, fmt.Errorf("it has more than one //go:build line")
			}
			goBuild = string(line)
		}
	Comments:
		for len(line) > 0 {
			if inSlashStar {
				if i := bytes.Index(line, []byte("*/")); i >= 0 {
					inSlashStar = false
					line = bytes.TrimSpace(line[i+len("*/"):])
					continue Comments
				}
				continue Lines
			}
			if bytes.HasPrefix(line, []byte("//")) {
				continue Lines
			}
			if bytes.HasPrefix(line, []byte("/*")) {
				inSlashStar = true
				line = bytes.TrimSpace(line[len("/*"):])
				continue Comments
			}
			break Lines
		}
	}
	for _, line := range strings.Split(string(content[:end]), "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "//") && constraint.IsPlusBuild(line) {
			plusBuild = append(plusBuild, line)
		}
	}
	return goBuild, plusBuild, nil
}

func hasMarker(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}
	for _, comment := range doc.List {
		if comment.Text == specMarker {
			return true
		}
	}
	return false
}

// constSpec parses the specification held by one marked constant.
func constSpec(fset *token.FileSet, value *ast.ValueSpec) (*spec, error) {
	pos := fset.Position(value.Pos())
	if len(value.Names) != 1 || len(value.Values) != 1 {
		return nil, errorAt(pos, "a %s constant is declared alone with one string literal", specMarker)
	}
	literal, ok := value.Values[0].(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return nil, errorAt(pos, "a %s constant is one string literal", specMarker)
	}
	text, err := strconv.Unquote(literal.Value)
	if err != nil {
		return nil, errorAt(pos, "%v", err)
	}
	s := &spec{
		constName: value.Names[0].Name, text: text, pos: pos,
		lines: lineMap{start: fset.Position(literal.Pos()), raw: literal.Value[0] == '`', text: strings.Split(text, "\n")},
	}
	if err := parseSpec(s); err != nil {
		return nil, err
	}
	return s, nil
}

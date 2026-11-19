package generator

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// A spec is one parsed specification constant. It may ask for operation
// interfaces, describe one hierarchy, or both.
type spec struct {
	imports    []specImport
	operations []*operationSet

	// The hierarchy part. name is empty when the spec has none.
	name    string // the generated Definition variable and helper prefix
	leaf    string // leaf type expression
	levels  []level
	wrapper *wrapperDecl

	constName string // name of the specification constant
	text      string // exact specification text
	pos       token.Position
	lines     lineMap

	hierarchyLine int
	typeRefs      []typeRef // every type expression, in specification order
	// existingUses are the existing interfaces the hierarchy's registrations
	// and the wrapper's forwarders use, whose guards the output repeats.
	existingUses []existingUse
}

type specImport struct {
	name, path string
	used       bool
	line       int // specification line, 0 for imports a signature needs
}

// A typeRef is one type expression of a specification and its line.
type typeRef struct {
	expr string
	line int
}

// An operationSet asks for one single-method interface per method of the
// listed contracts.
type operationSet struct {
	contracts      []string // handwritten contract interfaces of the package
	prefix, suffix string   // generated name: prefix + method + suffix
	existing       []existingOp
	line           int

	// Filled by analysis.
	ops         []operation            // every operation of the set, in order
	contractOps map[string][]operation // the operations of each listed contract
}

// An existingOp names the handwritten or imported interface that already
// identifies one method, so that none is generated for it.
type existingOp struct {
	method, typ string
	line        int
}

// An operation is one method of a contract and the interface identifying it.
type operation struct {
	method    method
	typ       string // operation interface type expression
	generated bool   // false when typ comes from an existing line
	key       string // typeKey of typ: equal keys denote the same type
	// typeImports are the specification imports typ uses.
	typeImports []specImport
	// origin is the source file whose operations line gives typ.
	origin string
	// pos and decl say where typ is given: its existing line, or the
	// operations line that generates it.
	pos  token.Position
	decl string
}

// An existingUse is an existing operation interface, as one specification
// spells it in a registration or a forwarder.
type existingUse struct {
	expr string
	op   operation
}

// A wrapperDecl asks for the public wrapper of the hierarchy.
type wrapperDecl struct {
	name   string // wrapper type
	field  string // the named field that holds the leaf
	holder string // the leaf's directly embedded holder field
	line   int
	// holderGiven is true when the specification names the holder.
	holderGiven bool

	// Filled by analysis.
	public, private string      // the leaf's effective contracts
	forwards        []operation // public operations
	privateOps      []operation // private operations
	// identities pairs a forwarder's interface with a registration that
	// spells the same type differently; the compiler confirms each pair.
	identities [][2]string
}

type level struct {
	node            string // node type expression
	parent          string // parent field name, "" for the root
	public, private string // effective contracts, carried forward when omitted
	slots           []slot // slots[0] is always the node slot
}

// A slot is the level's node (path ".") or one component.
type slot struct {
	path     string
	provider string // provider type expression
	ops      []string
	lines    []int // the specification line of each operation
}

// A lineMap gives the file position of a line of the specification text.
// For a raw string literal each line of the text is a line of the file, and
// the position is that of the line's first word; an interpreted literal
// maps every line to the literal itself.
type lineMap struct {
	start token.Position // position of the literal
	raw   bool
	text  []string // the specification's lines
}

func (m lineMap) position(line int) token.Position {
	if !m.raw || line < 1 || line > len(m.text) {
		return m.start
	}
	pos := m.start
	indent := len(m.text[line-1]) - len(strings.TrimLeft(m.text[line-1], " \t"))
	if line == 1 {
		pos.Column += 1 + indent // after the opening back quote
		return pos
	}
	pos.Line += line - 1
	pos.Column = 1 + indent
	return pos
}

// An Error is a diagnostic with the position it is about.
type Error struct {
	Pos token.Position
	Msg string
}

func (e *Error) Error() string {
	if !e.Pos.IsValid() {
		return e.Msg
	}
	return fmt.Sprintf("%s: %s", e.Pos, e.Msg)
}

func errorAt(pos token.Position, format string, args ...any) *Error {
	return &Error{Pos: pos, Msg: fmt.Sprintf(format, args...)}
}

// lineError reports a problem with one line of a specification.
func (s *spec) lineError(line int, format string, args ...any) *Error {
	return errorAt(s.lines.position(line), "%s, line %d: %s", s.constName, line, fmt.Sprintf(format, args...))
}

// parseSpec reads the line-oriented grammar of specification S10:
//
//	import <name> "<path>"
//	operations <Contract>... [prefix=<p>] [suffix=<s>]
//	existing <Method> <Type>
//	hierarchy <Name> <LeafType>
//	level <NodeType> [parent=<Field>] [public=<Type> private=<Type>]
//	node <Operation>...
//	component <path> <ProviderType> <Operation>...
//	wrapper <Type> [field=<name>] [holder=<Field>]
//
// Blank lines and lines starting with // are ignored. Types are Go type
// expressions without spaces. An existing line belongs to the operations
// line above it; level, node, component and wrapper lines belong to the
// hierarchy. The registrations, paths, parents and contracts of a hierarchy
// come from this text alone.
func parseSpec(s *spec) error {
	fail := func(line int, format string, args ...any) error {
		return s.lineError(line, format, args...)
	}
	options := func(number int, what string, args []string, allowed ...string) (map[string]string, error) {
		values := make(map[string]string)
		for _, option := range args {
			key, value, ok := strings.Cut(option, "=")
			known := false
			for _, name := range allowed {
				known = known || name == key
			}
			if !ok || value == "" || !known {
				return nil, fail(number, "unknown %s option %q; the options are %s=", what, option, strings.Join(allowed, "=, "))
			}
			if _, twice := values[key]; twice {
				return nil, fail(number, "option %s= is given twice", key)
			}
			values[key] = value
		}
		return values, nil
	}
	var current *level
	var set *operationSet
	for index, raw := range strings.Split(s.text, "\n") {
		number := index + 1
		fields := strings.Fields(raw)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "//") {
			continue
		}
		keyword, args := fields[0], fields[1:]
		switch keyword {
		case "level", "node", "component", "wrapper":
			if s.name == "" {
				return fail(number, "%s needs a preceding line: hierarchy <Name> <LeafType>", keyword)
			}
		}
		if keyword != "existing" && keyword != "operations" {
			// An existing line must directly follow its operations line.
			set = nil
		}
		switch keyword {
		case "hierarchy":
			if s.name != "" {
				return fail(number, "a specification describes at most one hierarchy; use another constant for another one")
			}
			if len(args) != 2 || !token.IsIdentifier(args[0]) {
				return fail(number, "want: hierarchy <Name> <LeafType>")
			}
			s.name, s.leaf, s.hierarchyLine = args[0], args[1], number
			s.typeRefs = append(s.typeRefs, typeRef{args[1], number})
		case "import":
			if len(s.levels) != 0 || len(s.operations) != 0 {
				return fail(number, "import lines precede operations and levels")
			}
			if len(args) != 2 || !token.IsIdentifier(args[0]) {
				return fail(number, `want: import <name> "<path>"`)
			}
			path, err := strconv.Unquote(args[1])
			if err != nil || path == "" {
				return fail(number, `want: import <name> "<path>"`)
			}
			s.imports = append(s.imports, specImport{name: args[0], path: path, line: number})
		case "operations":
			set = &operationSet{line: number}
			var named []string
			for _, arg := range args {
				if strings.Contains(arg, "=") {
					named = append(named, arg)
					continue
				}
				if !token.IsIdentifier(arg) {
					return fail(number, "%q is not the name of a contract declared in this package", arg)
				}
				set.contracts = append(set.contracts, arg)
			}
			values, err := options(number, "operations", named, "prefix", "suffix")
			if err != nil {
				return err
			}
			set.prefix, set.suffix = values["prefix"], values["suffix"]
			if len(set.contracts) == 0 {
				return fail(number, "want: operations <Contract>... [prefix=<p>] [suffix=<s>]")
			}
			s.operations = append(s.operations, set)
		case "existing":
			if set == nil {
				return fail(number, "an existing line directly follows an operations line or another existing line")
			}
			if len(args) != 2 || !token.IsIdentifier(args[0]) {
				return fail(number, "want: existing <Method> <Type>")
			}
			for _, known := range set.existing {
				if known.method == args[0] {
					return fail(number, "method %s has two existing lines", args[0])
				}
			}
			set.existing = append(set.existing, existingOp{method: args[0], typ: args[1], line: number})
			s.typeRefs = append(s.typeRefs, typeRef{args[1], number})
		case "level":
			if s.wrapper != nil {
				return fail(number, "the wrapper line follows the last level")
			}
			if len(args) == 0 {
				return fail(number, "want: level <NodeType> [parent=<Field>] [public=<Type> private=<Type>]")
			}
			values, err := options(number, "level", args[1:], "parent", "public", "private")
			if err != nil {
				return err
			}
			next := level{node: args[0], parent: values["parent"], public: values["public"], private: values["private"]}
			s.typeRefs = append(s.typeRefs, typeRef{next.node, number})
			if next.public != "" {
				s.typeRefs = append(s.typeRefs, typeRef{next.public, number}, typeRef{next.private, number})
			}
			if (next.public == "") != (next.private == "") {
				return fail(number, "public= and private= are given together or not at all")
			}
			if len(s.levels) == 0 {
				if next.parent != "" {
					return fail(number, "the root level has no parent")
				}
				if next.public == "" {
					return fail(number, "the root level names public= and private=")
				}
			} else {
				previous := s.levels[len(s.levels)-1]
				if next.public == "" {
					next.public, next.private = previous.public, previous.private
				}
				if next.parent == "" {
					// An anonymous field is named after its type. This is
					// the field's syntactic name, not a lookup: the library
					// validates the emitted name against the actual struct.
					next.parent = embeddedName(previous.node)
				}
				if !token.IsIdentifier(next.parent) {
					return fail(number, "the parent field of %s cannot be derived from %s; write parent=<Field>", next.node, previous.node)
				}
			}
			next.slots = []slot{{path: ".", provider: next.node}}
			s.levels = append(s.levels, next)
			current = &s.levels[len(s.levels)-1]
		case "node":
			if current == nil {
				return fail(number, "a node line needs a level line above it")
			}
			if s.wrapper != nil {
				return fail(number, "the wrapper line follows the last level")
			}
			if len(args) == 0 {
				return fail(number, "want: node <Operation>...")
			}
			current.slots[0].ops = append(current.slots[0].ops, args...)
			for _, op := range args {
				s.typeRefs = append(s.typeRefs, typeRef{op, number})
				current.slots[0].lines = append(current.slots[0].lines, number)
			}
		case "component":
			if current == nil {
				return fail(number, "a component line needs a level line above it")
			}
			if s.wrapper != nil {
				return fail(number, "the wrapper line follows the last level")
			}
			if len(args) < 3 {
				return fail(number, "want: component <path> <ProviderType> <Operation>...")
			}
			if !validPath(args[0]) {
				return fail(number, "%q is not a component path: use direct field names joined by dots; the node's own operations go on a node line", args[0])
			}
			index := -1
			for known, slot := range current.slots {
				if slot.path == args[0] {
					index = known
				}
			}
			if index < 0 {
				current.slots = append(current.slots, slot{path: args[0], provider: args[1]})
				index = len(current.slots) - 1
			} else if current.slots[index].provider != args[1] {
				return fail(number, "component %s is declared with provider types %s and %s", args[0], current.slots[index].provider, args[1])
			}
			current.slots[index].ops = append(current.slots[index].ops, args[2:]...)
			for range args[2:] {
				current.slots[index].lines = append(current.slots[index].lines, number)
			}
			for _, typ := range args[1:] {
				s.typeRefs = append(s.typeRefs, typeRef{typ, number})
			}
		case "wrapper":
			if current == nil {
				return fail(number, "the wrapper line follows the last level")
			}
			if s.wrapper != nil {
				return fail(number, "a hierarchy has at most one wrapper line")
			}
			if len(args) == 0 || !token.IsIdentifier(args[0]) {
				return fail(number, "want: wrapper <Type> [field=<name>] [holder=<Field>]")
			}
			values, err := options(number, "wrapper", args[1:], "field", "holder")
			if err != nil {
				return err
			}
			s.wrapper = &wrapperDecl{name: args[0], field: "state", holder: "Node", line: number}
			if values["field"] != "" {
				s.wrapper.field = values["field"]
			}
			if values["holder"] != "" {
				s.wrapper.holder, s.wrapper.holderGiven = values["holder"], true
			}
			if !token.IsIdentifier(s.wrapper.field) || !token.IsIdentifier(s.wrapper.holder) {
				return fail(number, "field= and holder= are field names")
			}
			// Specification S8: an unexported wrapper with named unexported
			// storage.
			switch w := s.wrapper; {
			case w.name == "_":
				return fail(number, "wrapper _ must be named: give the wrapper an unexported type name")
			case token.IsExported(w.name):
				return fail(number, "wrapper %s must be unexported, as specification S8 requires of a public wrapper: name it %s", w.name, unexported(w.name))
			case w.field == "_":
				return fail(number, "field=_ must be named: the wrapper keeps the object in a named unexported field")
			case token.IsExported(w.field):
				return fail(number, "field=%s must be unexported, as specification S8 requires of the wrapper's storage: write field=%s", w.field, unexported(w.field))
			}
		default:
			return fail(number, "unknown keyword %q; the keywords are import, operations, existing, hierarchy, level, node, component, and wrapper", keyword)
		}
	}
	if s.name == "" && len(s.operations) == 0 {
		return fail(1, "a specification has a hierarchy line, an operations line, or both")
	}
	if s.name != "" {
		if len(s.levels) == 0 {
			return fail(s.hierarchyLine, "hierarchy %s has no level", s.name)
		}
		if last := s.levels[len(s.levels)-1].node; last != s.leaf {
			return fail(s.hierarchyLine, "the last level is %s but the leaf is %s", last, s.leaf)
		}
	}
	return s.checkTypes()
}

// validPath reports whether path is a component path: direct field names
// joined by dots. "." is the node slot and is written with the node keyword.
func validPath(path string) bool {
	for _, part := range strings.Split(path, ".") {
		if part == "_" || !token.IsIdentifier(part) {
			return false
		}
	}
	return true
}

// embeddedName returns the name Go gives an anonymous field of the type.
func embeddedName(typ string) string {
	if cut := strings.IndexByte(typ, '['); cut >= 0 {
		typ = typ[:cut]
	}
	if cut := strings.LastIndexByte(typ, '.'); cut >= 0 {
		typ = typ[cut+1:]
	}
	return typ
}

// checkTypes parses every type expression and marks the imports they use.
func (s *spec) checkTypes() error {
	seen := make(map[string]bool)
	for _, declared := range s.imports {
		if seen[declared.name] {
			return s.lineError(declared.line, "import name %s is declared twice", declared.name)
		}
		seen[declared.name] = true
	}
	for _, ref := range s.typeRefs {
		fail := func(format string, args ...any) error {
			return s.lineError(ref.line, format, args...)
		}
		expr, err := parser.ParseExpr(ref.expr)
		if err != nil {
			return fail("%q is not a Go type expression", ref.expr)
		}
		var bad error
		ast.Inspect(expr, func(n ast.Node) bool {
			selector, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok {
				bad = fail("%q is not a Go type expression", ref.expr)
				return false
			}
			for index := range s.imports {
				if s.imports[index].name == pkg.Name {
					s.imports[index].used = true
					return false
				}
			}
			bad = fail("%s uses package %s; add a line: import %s \"<path>\"", ref.expr, pkg.Name, pkg.Name)
			return false
		})
		if bad != nil {
			return bad
		}
	}
	for _, declared := range s.imports {
		if !declared.used {
			return s.lineError(declared.line, "import %s is not used", declared.name)
		}
	}
	return nil
}

// typeExprs lists every type expression of the spec, in spec order.
func (s *spec) typeExprs() []string {
	types := make([]string, len(s.typeRefs))
	for index, ref := range s.typeRefs {
		types[index] = ref.expr
	}
	return types
}

// identifiers returns every identifier used by the spec's type expressions
// and contract names.
func (s *spec) identifiers() map[string]bool {
	names := make(map[string]bool)
	for _, set := range s.operations {
		for _, contract := range set.contracts {
			names[contract] = true
		}
	}
	for _, typ := range s.typeExprs() {
		expr, err := parser.ParseExpr(typ)
		if err != nil {
			continue
		}
		ast.Inspect(expr, func(n ast.Node) bool {
			if ident, ok := n.(*ast.Ident); ok {
				names[ident.Name] = true
			}
			return true
		})
	}
	return names
}

// qualifier resolves a package name that the specification's type
// expressions use to its import path.
func (s *spec) qualifier(name string) (string, bool) {
	for _, declared := range s.imports {
		if declared.name == name {
			return declared.path, true
		}
	}
	return "", false
}

// importsOf returns the specification imports a type expression uses.
func (s *spec) importsOf(typ string) []specImport {
	expr, err := parser.ParseExpr(typ)
	if err != nil {
		return nil
	}
	var used []specImport
	ast.Inspect(expr, func(n ast.Node) bool {
		selector, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := selector.X.(*ast.Ident); ok {
			for _, declared := range s.imports {
				if declared.name == pkg.Name {
					used = append(used, declared)
				}
			}
		}
		return false
	})
	return used
}

// unexported suggests an unexported spelling of an exported identifier.
func unexported(name string) string {
	return strings.ToLower(name[:1]) + name[1:]
}

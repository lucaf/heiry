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
	// omitInitPlan is set by omit=InitPlan: the Plan helper is not generated.
	omitInitPlan bool

	// The chain part, a named run of levels that generates no declaration.
	// chain is empty when the spec has none. A spec holds at most one
	// hierarchy, chain, or part.
	chain     string
	chainLine int
	// from names the hierarchy or chain whose levels come first, "" for
	// none. Until the levels are assembled, levels holds only the spec's own;
	// then it starts with those of the spec that from= names.
	from string

	// The part part: a named group of component lines, which use lines add
	// to levels. part is empty when the spec has none.
	part       string
	partLine   int
	components []componentLine

	// takers lists, in file and line order, the hierarchies that take this
	// constant's lines: a hierarchy's or chain's levels through from=, and a
	// part's component lines through use. Each comes with the from= and use
	// steps from it to this constant. takenLines holds the lines they take:
	// the header line and every line of the constant's own levels or, for a
	// part, of its component lines. Both are set once the links between
	// specifications are checked, so that a later diagnostic at such a line
	// lists them (ADR 0014).
	takers     []taker
	takenLines map[int]bool

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
	line       int   // specification line, 0 for imports a signature needs
	spec       *spec // the constant of an import line
	// written describes an import a signature needs, as its Go file
	// declares it, with its position.
	written string
}

// binding describes where an import's name is bound to its path.
func (i specImport) binding() string {
	if i.spec != nil {
		return fmt.Sprintf("%s, line %d: import %s %q", i.spec.constName, i.line, i.name, i.path)
	}
	return i.written
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
	node            string   // node type expression
	parent          string   // parent field name, "" for the root
	public, private string   // effective contracts, carried forward when omitted
	slots           []slot   // slots[0] is always the node slot
	line            specLine // the level line
	// contracts is the line that writes public= and private=; a level that
	// inherits them keeps the line of the level they come from, whose
	// constant's imports they are read with.
	contracts specLine
	// items are the level's component and use lines in written order, from
	// which the slots are built again when a use line adds a part's lines.
	items []levelItem
}

// A componentLine is one component line, as written.
type componentLine struct {
	path, provider string
	ops            []string
	line           int
}

// A levelItem is one component or use line of a level.
type levelItem struct {
	component *componentLine // nil for a use line
	use       string         // the part a use line names
	part      *spec          // that part, once resolved
	line      int
}

// A slot is the level's node (path ".") or one component. Its provider is
// written on the level line for the node slot. For a component, it is
// spelled on every line that adds to the slot, and read on each with that
// line's constant's imports.
type slot struct {
	path     string
	provider string // provider type expression
	ops      []string
	lines    []specLine // the line of each operation
}

// A specLine is one line of a specification constant. A level and each of
// its registrations keep the line and the constant they are written in, so
// that their type expressions are read with that constant's imports and a
// diagnostic about one names that constant's line.
type specLine struct {
	spec   *spec // the constant the line is written in
	number int   // the line's number in that constant's text, from 1
	// path lists the from= and use steps from the constant whose levels
	// hold this copy of the line to the line's own constant, empty for an
	// own line.
	path []string
	// usedIn is the constant of the use line that added a part's line.
	usedIn *spec
	// of is the hierarchy whose assembled levels hold this copy of the
	// line, for the checks that run per hierarchy; nil otherwise.
	of *spec
}

// A taker is a hierarchy that takes a constant's lines, a hierarchy's or
// chain's levels through from= or a part's component lines through use,
// and the from= and use steps from it to that constant.
type taker struct {
	spec *spec
	path []string
}

// takersOf returns the hierarchies that take a line of the constant.
func (s *spec) takersOf(line int) []taker {
	if s.takenLines[line] {
		return s.takers
	}
	return nil
}

// errorf reports a problem with the line. A problem found in the checks of
// one hierarchy names it, when it takes the line, and lists the other
// hierarchies that take the line after "also taken by". Any other problem
// is reported as lineError does.
func (l specLine) errorf(format string, args ...any) *Error {
	if l.of == nil {
		return l.spec.lineError(l.number, format, args...)
	}
	var parts, others []string
	if l.spec != l.of && len(l.path) != 0 {
		parts = append(parts, "taken by "+l.of.constName+" through "+strings.Join(l.path, ", "))
	}
	for _, t := range l.spec.takersOf(l.number) {
		if t.spec != l.of {
			others = append(others, t.spec.constName)
		}
	}
	if len(others) != 0 {
		parts = append(parts, "also taken by "+listNames(others))
	}
	if len(parts) == 0 {
		return errorAt(l.spec.lines.position(l.number), "%s, line %d: %s", l.spec.constName, l.number, fmt.Sprintf(format, args...))
	}
	return errorAt(l.spec.lines.position(l.number), "%s, line %d (%s): %s", l.spec.constName, l.number, strings.Join(parts, "; "), fmt.Sprintf(format, args...))
}

// listNames joins names as "a", "a and b", or "a, b and c".
func listNames(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// qualifier resolves a package name that the line uses to its import path,
// through the imports of the line's constant.
func (l specLine) qualifier(name string) (string, bool) {
	return l.spec.qualifier(name)
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

// lineError reports a problem with one line of a specification. Once the
// links between specifications are checked, a problem at a line that
// hierarchies take names no hierarchy, so it lists them all after "taken
// by", each with its path.
func (s *spec) lineError(line int, format string, args ...any) *Error {
	if takers := s.takersOf(line); len(takers) != 0 {
		entries := make([]string, len(takers))
		for index, t := range takers {
			entries[index] = t.spec.constName + " through " + strings.Join(t.path, ", ")
		}
		return errorAt(s.lines.position(line), "%s, line %d (taken by %s): %s", s.constName, line, strings.Join(entries, ", and by "), fmt.Sprintf(format, args...))
	}
	return errorAt(s.lines.position(line), "%s, line %d: %s", s.constName, line, fmt.Sprintf(format, args...))
}

// parseSpec reads the line-oriented grammar of specification S10:
//
//	import <name> "<path>"
//	operations <Contract>... [prefix=<p>] [suffix=<s>]
//	existing <Method> <Type>
//	hierarchy <Name> <LeafType> [from=<Name>] [omit=InitPlan]
//	chain <Name> [from=<Name>]
//	part <Name>
//	level <NodeType> [parent=<Field>] [public=<Type> private=<Type>]
//	node <Operation>...
//	component <path> <ProviderType> <Operation>...
//	use <Part>
//	wrapper <Type> [field=<name>] [holder=<Field>]
//
// Blank lines and lines starting with // are ignored. Types are Go type
// expressions without spaces. An existing line belongs to the operations
// line above it; level, node, component, use and wrapper lines belong to
// the hierarchy, and level, node, component and use lines to the chain; only
// component lines follow a part line. The registrations, paths, parents and
// contracts of a hierarchy come from this text, from the levels that from=
// names, and from the component lines of the parts that use lines name
// (ADR 0014). With from=, the
// first own level follows a level of another constant, so its default
// parent field and contracts are set when the levels are assembled.
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
		// Only component lines follow a part line.
		if s.part != "" {
			switch keyword {
			case "hierarchy", "chain", "part":
				return fail(number, "a specification holds at most one hierarchy, chain, or part; use another constant for another one")
			case "import", "operations", "existing":
				return fail(number, "only component lines follow a part line: write the %s line before it", keyword)
			case "level", "node", "use", "wrapper":
				return fail(number, "only component lines follow a part line: a part holds no %s line", keyword)
			}
		}
		switch keyword {
		case "level", "node", "component", "wrapper", "use":
			if s.name == "" && s.chain == "" && s.part == "" {
				// Only a hierarchy has a wrapper line, and a part line can
				// precede a component line too.
				switch keyword {
				case "wrapper":
					return fail(number, "%s needs a preceding line: hierarchy <Name> <LeafType>", keyword)
				case "component":
					return fail(number, "%s needs a preceding line: hierarchy <Name> <LeafType>, chain <Name>, or part <Name>", keyword)
				}
				return fail(number, "%s needs a preceding line: hierarchy <Name> <LeafType> or chain <Name>", keyword)
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
			if s.chain != "" {
				return fail(number, "a specification holds at most one hierarchy, chain, or part; use another constant for another one")
			}
			if len(args) < 2 || !token.IsIdentifier(args[0]) || strings.Contains(args[1], "=") {
				return fail(number, "want: hierarchy <Name> <LeafType> [from=<Name>] [omit=InitPlan]")
			}
			if args[0] == "_" {
				return fail(number, "hierarchy _ must be named: the generated declaration and its helpers are named after it")
			}
			values, err := options(number, "hierarchy", args[2:], "from", "omit")
			if err != nil {
				return err
			}
			if from, given := values["from"]; given {
				if !token.IsIdentifier(from) || from == "_" {
					return fail(number, "from= takes the name of a hierarchy or chain")
				}
				s.from = from
			}
			if omit, given := values["omit"]; given {
				if omit != "InitPlan" {
					return fail(number, "omit= takes InitPlan, the only helper that can be left out")
				}
				s.omitInitPlan = true
			}
			s.name, s.leaf, s.hierarchyLine = args[0], args[1], number
			s.typeRefs = append(s.typeRefs, typeRef{args[1], number})
		case "part":
			if s.name != "" || s.chain != "" {
				return fail(number, "a specification holds at most one hierarchy, chain, or part; use another constant for another one")
			}
			if len(args) != 1 || !token.IsIdentifier(args[0]) || args[0] == "_" {
				return fail(number, "want: part <Name>")
			}
			s.part, s.partLine = args[0], number
		case "use":
			if current == nil {
				return fail(number, "a use line needs a level line above it")
			}
			if s.wrapper != nil {
				return fail(number, "the wrapper line follows the last level")
			}
			if len(args) != 1 || !token.IsIdentifier(args[0]) || args[0] == "_" {
				return fail(number, "want: use <Part>")
			}
			current.items = append(current.items, levelItem{use: args[0], line: number})
		case "chain":
			if s.name != "" || s.chain != "" {
				return fail(number, "a specification holds at most one hierarchy, chain, or part; use another constant for another one")
			}
			if len(args) == 0 || !token.IsIdentifier(args[0]) || args[0] == "_" {
				return fail(number, "want: chain <Name> [from=<Name>]")
			}
			values, err := options(number, "chain", args[1:], "from")
			if err != nil {
				return err
			}
			if from, given := values["from"]; given {
				if !token.IsIdentifier(from) || from == "_" {
					return fail(number, "from= takes the name of a hierarchy or chain")
				}
				s.from = from
			}
			s.chain, s.chainLine = args[0], number
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
			s.imports = append(s.imports, specImport{name: args[0], path: path, line: number, spec: s})
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
			next := level{node: args[0], parent: values["parent"], public: values["public"], private: values["private"], line: specLine{spec: s, number: number}}
			if next.public != "" {
				next.contracts = next.line
			}
			s.typeRefs = append(s.typeRefs, typeRef{next.node, number})
			if next.public != "" {
				s.typeRefs = append(s.typeRefs, typeRef{next.public, number}, typeRef{next.private, number})
			}
			if (next.public == "") != (next.private == "") {
				return fail(number, "public= and private= are given together or not at all")
			}
			switch {
			case len(s.levels) == 0 && s.from != "":
				// The first own level after from= follows the last taken
				// level; its defaults are set when the levels are assembled.
			case len(s.levels) == 0:
				if next.parent != "" {
					return fail(number, "the root level has no parent")
				}
				if next.public == "" {
					return fail(number, "the root level names public= and private=")
				}
			default:
				previous := s.levels[len(s.levels)-1]
				if next.public == "" {
					next.public, next.private, next.contracts = previous.public, previous.private, previous.contracts
				}
				if next.parent == "" {
					// An anonymous field is named after its type. This is
					// the field's syntactic name, not a lookup: the library
					// validates the emitted name against the actual struct.
					next.parent = embeddedName(previous.node)
				}
				if !token.IsIdentifier(next.parent) {
					return next.line.errorf("the parent field of %s cannot be derived from %s; write parent=<Field>", next.node, previous.node)
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
				current.slots[0].lines = append(current.slots[0].lines, specLine{spec: s, number: number})
			}
		case "component":
			if s.part != "" {
				if len(args) < 3 {
					return fail(number, "want: component <path> <ProviderType> <Operation>...")
				}
				if !validPath(args[0]) {
					return fail(number, "%q is not a component path: use direct field names joined by dots; the node's own operations go on a node line", args[0])
				}
				for _, known := range s.components {
					if known.path == args[0] && known.provider != args[1] {
						return fail(number, "component %s is declared with provider types %s and %s", args[0], known.provider, args[1])
					}
				}
				s.components = append(s.components, componentLine{path: args[0], provider: args[1], ops: args[2:], line: number})
				for _, typ := range args[1:] {
					s.typeRefs = append(s.typeRefs, typeRef{typ, number})
				}
				continue
			}
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
				current.slots[index].lines = append(current.slots[index].lines, specLine{spec: s, number: number})
			}
			current.items = append(current.items, levelItem{component: &componentLine{path: args[0], provider: args[1], ops: args[2:], line: number}, line: number})
			for _, typ := range args[1:] {
				s.typeRefs = append(s.typeRefs, typeRef{typ, number})
			}
		case "wrapper":
			if s.chain != "" {
				return fail(number, "a chain has no wrapper line; give the wrapper to a hierarchy that takes the chain's levels with from=")
			}
			if current == nil && s.from == "" {
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
			return fail(number, "unknown keyword %q; the keywords are import, operations, existing, hierarchy, chain, part, level, node, component, use, and wrapper", keyword)
		}
	}
	if s.name == "" && s.chain == "" && s.part == "" && len(s.operations) == 0 {
		return fail(1, "a specification has a hierarchy, chain, or part line, an operations line, or both")
	}
	if s.part != "" && len(s.components) == 0 {
		return fail(s.partLine, "part %s has no component line", s.part)
	}
	// A hierarchy with from= may have no level of its own: it generates a
	// second definition. A chain may not, with or without from=.
	if s.name != "" && s.from == "" && len(s.levels) == 0 {
		return fail(s.hierarchyLine, "hierarchy %s has no level", s.name)
	}
	if s.chain != "" && len(s.levels) == 0 {
		if s.from == "" {
			return fail(s.chainLine, "chain %s has no level", s.chain)
		}
		return fail(s.chainLine, "chain %s has no level of its own; write from=%s in the hierarchy instead", s.chain, s.from)
	}
	// The leaf is compared with the last level after every file is read,
	// because the aliases the comparison follows may be declared in any file.
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

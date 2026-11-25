package generator

import (
	"go/token"
	"slices"
	"sort"
	"strings"
)

// header returns the kind, name, and line of a spec's hierarchy, chain, or
// part line; kind is empty when it has none.
func (s *spec) header() (kind, name string, line int) {
	switch {
	case s.name != "":
		return "hierarchy", s.name, s.hierarchyLine
	case s.chain != "":
		return "chain", s.chain, s.chainLine
	case s.part != "":
		return "part", s.part, s.partLine
	}
	return "", "", 0
}

// A named spec is a hierarchy, chain, or part with the file that holds it.
type named struct {
	s      *spec
	source *sourceFile
}

// resolve runs the checks that need every file read, before the analysis,
// and assembles each hierarchy's levels (ADR 0014). First come the checks of
// every link between specifications, in all files: duplicate names, then
// the names that from= and use give, then cycles of from=. Only when they
// pass does each chain and hierarchy, in file and line order, go through
// its own checks: the build rule at its from= line and at its use lines,
// the levels it takes, the merging of the parts' lines into its levels,
// and its leaf. Then the levels are assembled.
func resolve(sources []*sourceFile, packages map[string]*pkgInfo) error {
	byPkg := make(map[string]map[string]named)
	var order []named // every hierarchy, chain, and part, in file and line order
	for _, source := range sources {
		if byPkg[source.pkg] == nil {
			byPkg[source.pkg] = make(map[string]named)
		}
		for _, s := range source.specs {
			kind, name, line := s.header()
			if kind == "" {
				continue
			}
			if first, twice := byPkg[source.pkg][name]; twice {
				firstKind, _, firstLine := first.s.header()
				if firstKind == "hierarchy" && kind == "hierarchy" {
					// Two hierarchies of one name keep the message of the
					// generated-name check that reported them before.
					return errorAt(s.pos, "%s would generate %s, which the specification at %s already generates", s.constName, name, first.s.pos)
				}
				return s.lineError(line, "the name %s is already declared by the %s at %s; hierarchy, chain and part names are unique in a package", name, firstKind, first.s.lines.position(firstLine))
			}
			byPkg[source.pkg][name] = named{s, source}
			order = append(order, named{s, source})
		}
	}

	clauses := make([]string, 0, len(byPkg))
	for pkg := range byPkg {
		clauses = append(clauses, pkg)
	}
	sort.Strings(clauses)
	// lookup finds a name that from= or a use line gives, in the package
	// clause of n or, for the refusal, in another one.
	lookup := func(n named, line int, written, name, want string) (named, error) {
		target, found := byPkg[n.source.pkg][name]
		if !found {
			for _, other := range clauses {
				if t, known := byPkg[other][name]; known && other != n.source.pkg {
					kind, _, _ := t.s.header()
					return named{}, n.s.lineError(line, "%s names the %s %s of package %s, but this specification is in package %s; neither can see the other's constants", written, kind, name, other, n.source.pkg)
				}
			}
			return named{}, n.s.lineError(line, "%s names no %s of package %s", written, want, n.source.pkg)
		}
		return target, nil
	}
	targets := make(map[*spec]named)
	parts := make(map[*spec]*sourceFile) // each part that a use line names, and its file
	for _, n := range order {
		s := n.s
		if s.from != "" {
			_, _, line := s.header()
			target, err := lookup(n, line, "from="+s.from, s.from, "hierarchy or chain")
			if err != nil {
				return err
			}
			if kind, _, _ := target.s.header(); kind == "part" {
				return s.lineError(line, "from=%s names the part %s, but from= names a hierarchy or chain", s.from, s.from)
			}
			if err := noLevelTarget(byPkg[n.source.pkg], s, line, target); err != nil {
				return err
			}
			targets[s] = target
		}
		for index := range s.levels {
			for at := range s.levels[index].items {
				item := &s.levels[index].items[at]
				if item.use == "" {
					continue
				}
				target, err := lookup(n, item.line, "use "+item.use, item.use, "part")
				if err != nil {
					return err
				}
				if kind, _, _ := target.s.header(); kind != "part" {
					return s.lineError(item.line, "use %s names the %s %s, but use names a part", item.use, kind, item.use)
				}
				item.part, parts[target.s] = target.s, target.source
			}
		}
	}

	position := make(map[*spec]int)
	for index, n := range order {
		position[n.s] = index
	}
	for _, n := range order {
		var walk []*spec
		for s := n.s; s != nil; s = targets[s].s {
			if at := slices.Index(walk, s); at >= 0 {
				cycle := walk[at:]
				first := 0
				for index := range cycle {
					if position[cycle[index]] < position[cycle[first]] {
						first = index
					}
				}
				cycle = append(cycle[first:], cycle[:first]...)
				var names []string
				for _, member := range append(cycle, cycle[0]) {
					_, name, _ := member.header()
					names = append(names, name)
				}
				_, _, line := cycle[0].header()
				return cycle[0].lineError(line, "from=%s makes a cycle of from=: %s", cycle[0].from, strings.Join(names, ", "))
			}
			walk = append(walk, s)
		}
	}

	// With the links checked, record which hierarchies take each line:
	// every constant on a hierarchy's from= path, and every part that a use
	// line of one of their levels names, each with the steps that reach it.
	for _, n := range order {
		if n.s.name == "" {
			continue
		}
		var path []string
		for x := n.s; x != nil; x = targets[x].s {
			if x != n.s {
				_, name, _ := x.header()
				path = append(path, "from="+name)
				x.addTaker(n.s, path)
			}
			for _, l := range x.levels {
				for _, item := range l.items {
					if item.part != nil {
						item.part.addTaker(n.s, append(slices.Clone(path), "use "+item.use))
					}
				}
			}
		}
	}
	for _, n := range order {
		if len(n.s.takers) == 0 {
			continue
		}
		_, _, header := n.s.header()
		n.s.takenLines = map[int]bool{header: true}
		for _, c := range n.s.components {
			n.s.takenLines[c.line] = true
		}
		for _, l := range n.s.levels {
			n.s.takenLines[l.line.number] = true
			for _, sl := range l.slots {
				for _, line := range sl.lines {
					n.s.takenLines[line.number] = true
				}
			}
			for _, item := range l.items {
				n.s.takenLines[item.line] = true
			}
		}
	}

	for _, n := range order {
		s := n.s
		builds := packages[n.source.pkg].builds
		compiled := func(file string) bool {
			return file == n.source.name || builds[file].compiledWith(builds[n.source.name])
		}
		if s.from != "" {
			target := targets[s]
			_, _, line := s.header()
			if !compiled(target.source.name) {
				kind, _, _ := target.s.header()
				return s.lineError(line, "from=%s names the %s %s, declared in %s, which some builds that compile %s leave out; declare it in %s, in a non-test file without build constraints or a platform suffix, or in a file with exactly the conditions of %s", s.from, kind, s.from, target.source.name, n.source.name, n.source.name, n.source.name)
			}
		}
		for _, l := range s.levels {
			for _, item := range l.items {
				if item.part != nil && !compiled(parts[item.part].name) {
					return s.lineError(item.line, "use %s names the part %s, declared in %s, which some builds that compile %s leave out; declare it in %s, in a non-test file without build constraints or a platform suffix, or in a file with exactly the conditions of %s", item.use, item.use, parts[item.part].name, n.source.name, n.source.name, n.source.name)
				}
			}
		}
		if s.from != "" {
			target := targets[s]
			// The first own level follows the last taken level.
			if len(s.levels) != 0 {
				first, last := &s.levels[0], lastLevel(target.s, targets)
				if first.parent == "" {
					first.parent = embeddedName(last.node)
				}
				if !token.IsIdentifier(first.parent) {
					return first.line.errorf("the parent field of %s cannot be derived from %s; write parent=<Field>", first.node, last.node)
				}
			}
		}
		for index := range s.levels {
			if err := s.addParts(index); err != nil {
				return err
			}
		}
		if s.name != "" {
			if err := packages[n.source.pkg].index.checkLeaf(s, lastLevel(s, targets)); err != nil {
				return err
			}
		}
	}

	assembled := make(map[*spec]bool)
	for _, n := range order {
		assemble(n.s, targets, assembled)
	}
	// Each hierarchy's copy of its lines is checked for that hierarchy.
	for _, n := range order {
		if n.s.name == "" {
			continue
		}
		for index := range n.s.levels {
			l := &n.s.levels[index]
			l.line.of, l.contracts.of = n.s, n.s
			for _, sl := range l.slots {
				for at := range sl.lines {
					sl.lines[at].of = n.s
				}
			}
		}
	}
	return nil
}

// noLevelTarget refuses a from= that names a hierarchy with no level of its
// own, naming that hierarchy's target to use instead, unless that target
// would lead back to the spec that names it.
func noLevelTarget(names map[string]named, s *spec, line int, target named) error {
	if kind, _, _ := target.s.header(); kind != "hierarchy" || len(target.s.levels) != 0 {
		return nil
	}
	if leadsTo(names, target.s.from, s) {
		return s.lineError(line, "%s has no level of its own, so from= cannot name it", s.from)
	}
	return s.lineError(line, "%s has no level of its own, so from= cannot name it; name its target directly, with from=%s", s.from, target.s.from)
}

// leadsTo reports whether following from= from the named hierarchy or chain
// reaches s, before an unknown name or a cycle stops it.
func leadsTo(names map[string]named, name string, s *spec) bool {
	seen := make(map[*spec]bool)
	for {
		n, found := names[name]
		if !found || seen[n.s] {
			return false
		}
		if n.s == s {
			return true
		}
		seen[n.s] = true
		name = n.s.from
	}
}

// lastLevel returns a spec's last level: its own last one, or, for a
// hierarchy with no level of its own, the last level of the spec it takes
// its levels from, which has one.
func lastLevel(s *spec, targets map[*spec]named) level {
	if len(s.levels) == 0 {
		return lastLevel(targets[s].s, targets)
	}
	return s.levels[len(s.levels)-1]
}

// addTaker records that hierarchy h takes the constant's lines through
// path, unless it already does through another one.
func (s *spec) addTaker(h *spec, path []string) {
	for _, t := range s.takers {
		if t.spec == h {
			return
		}
	}
	s.takers = append(s.takers, taker{h, slices.Clone(path)})
}

// addParts builds a level's slots again when a use line adds a part's
// component lines, in written order, as the written-out level would have
// them: lines with one path merge into one slot. Two provider types for
// one slot are refused at the later line, naming both constants; two lines
// of one constant were refused when it was read.
func (s *spec) addParts(index int) error {
	l := &s.levels[index]
	if !slices.ContainsFunc(l.items, func(item levelItem) bool { return item.part != nil }) {
		return nil
	}
	slots := []slot{l.slots[0]}
	add := func(c componentLine, line specLine) error {
		for at := range slots[1:] {
			sl := &slots[1+at]
			if sl.path != c.path {
				continue
			}
			if sl.provider != c.provider {
				return line.spec.lineError(line.number, "component %s is declared with provider types %s, in %s, and %s, in %s", c.path, sl.provider, sl.lines[0].spec.constName, c.provider, line.spec.constName)
			}
			for range c.ops {
				sl.lines = append(sl.lines, line)
			}
			sl.ops = append(sl.ops, c.ops...)
			return nil
		}
		sl := slot{path: c.path, provider: c.provider, ops: slices.Clone(c.ops)}
		for range c.ops {
			sl.lines = append(sl.lines, line)
		}
		slots = append(slots, sl)
		return nil
	}
	for _, item := range l.items {
		if item.component != nil {
			if err := add(*item.component, specLine{spec: s, number: item.line}); err != nil {
				return err
			}
			continue
		}
		for _, c := range item.part.components {
			if err := add(c, specLine{spec: item.part, number: c.line, path: []string{"use " + item.use}, usedIn: s}); err != nil {
				return err
			}
		}
	}
	l.slots = slots
	return nil
}

// assemble gives a spec with from= its full levels: those of its target,
// assembled first, then its own. Its own levels without contracts take
// those of the level before them, with the line they are written on. Each
// taken line's path gains the from= step, for diagnostics.
func assemble(s *spec, targets map[*spec]named, assembled map[*spec]bool) {
	if assembled[s] {
		return
	}
	assembled[s] = true
	target, takes := targets[s]
	if !takes {
		return
	}
	assemble(target.s, targets, assembled)
	_, targetName, _ := target.s.header()
	levels := make([]level, 0, len(target.s.levels)+len(s.levels))
	step := "from=" + targetName
	through := func(line specLine) specLine {
		line.path = append([]string{step}, line.path...)
		return line
	}
	for _, taken := range target.s.levels {
		taken.line, taken.contracts = through(taken.line), through(taken.contracts)
		slots := make([]slot, len(taken.slots))
		for index, sl := range taken.slots {
			sl.ops = slices.Clone(sl.ops)
			lines := make([]specLine, len(sl.lines))
			for at, line := range sl.lines {
				lines[at] = through(line)
			}
			sl.lines = lines
			slots[index] = sl
		}
		taken.slots = slots
		levels = append(levels, taken)
	}
	previous := levels[len(levels)-1]
	for index := range s.levels {
		own := &s.levels[index]
		if own.public == "" {
			own.public, own.private, own.contracts = previous.public, previous.private, previous.contracts
		}
		previous = *own
	}
	s.levels = append(levels, s.levels...)
}

// takenConstants lists the constants a hierarchy takes lines from, other
// than its own, through from= or use, each once, in the order its
// written-out form first uses a line of each.
func (s *spec) takenConstants() []*spec {
	if s.name == "" {
		// A chain's or part's own output holds nothing for its lines but its
		// own freshness assertion.
		return nil
	}
	var constants []*spec
	s.writtenExprs(func(_ string, line specLine) {
		if line.spec != nil && line.spec != s && !slices.Contains(constants, line.spec) {
			constants = append(constants, line.spec)
		}
	})
	return constants
}

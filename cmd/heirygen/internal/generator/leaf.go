package generator

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/types"
	"slices"
	"strings"
)

// checkLeaf requires a hierarchy's leaf to be its last level (ADR 0014). A
// leaf spelled exactly as its last level is accepted before any alias is
// followed. Otherwise the two are compared as types by sameType, and a
// refusal gives the reason when one is known.
func (ix *packageIndex) checkLeaf(s *spec, last level) error {
	if last.node == s.leaf {
		return nil
	}
	same, reason := ix.sameType(s.leaf, s.qualifier, last.node, last.line.qualifier)
	if same {
		return nil
	}
	// A last level taken through from= is named with its constant.
	spelled := last.node
	if last.line.spec != s {
		spelled = fmt.Sprintf("%s, at %s, line %d,", last.node, last.line.spec.constName, last.line.number)
	}
	if reason != "" {
		return s.lineError(s.hierarchyLine, "the last level is %s but the leaf is %s: %s", spelled, s.leaf, reason)
	}
	return s.lineError(s.hierarchyLine, "the last level is %s but the leaf is %s", spelled, s.leaf)
}

// A typeSide is one of the two type expressions sameType compares, or a
// part of one, with what it is read with.
type typeSide struct {
	expr      ast.Expr
	qualifier func(string) (string, bool)
	// file is the Go file of the alias whose right side expr is in, and
	// alias is that alias; both are unset for a specification's expression.
	file  *fileInfo
	alias string
	// chain lists the aliases followed to reach expr, to find cycles.
	chain []string
}

// sameType reports whether two type expressions, each read with the
// qualifier of the line it is written in, denote one type. It drops
// parentheses, replaces package names by import paths, and follows local
// aliases declared once and without type parameters, reading each right
// side with the imports of its own file, also inside type arguments. When
// the two differ, reason says why the generator could not follow a name the
// comparison stopped at, or is empty when both were followed to the end.
//
// It has a function of its own, separate from interface identity's typeKey,
// so that a change to one does not change the other.
func (ix *packageIndex) sameType(a string, qa func(string) (string, bool), b string, qb func(string) (string, bool)) (bool, string) {
	left, err := parser.ParseExpr(a)
	if err != nil {
		return false, ""
	}
	right, err := parser.ParseExpr(b)
	if err != nil {
		return false, ""
	}
	return ix.compareTypes(typeSide{expr: left, qualifier: qa}, typeSide{expr: right, qualifier: qb})
}

func (ix *packageIndex) compareTypes(a, b typeSide) (bool, string) {
	a.expr, b.expr = ast.Unparen(a.expr), ast.Unparen(b.expr)
	var reasonA, reasonB string
	for {
		var idA, idB string
		var nextA, nextB *typeSide
		idA, nextA, reasonA = ix.resolveName(a)
		idB, nextB, reasonB = ix.resolveName(b)
		// One name on both sides matches before either is followed.
		if idA != "" && idA == idB {
			return true, ""
		}
		if nextA != nil {
			a = *nextA
			continue
		}
		if nextB != nil {
			b = *nextB
			continue
		}
		break
	}
	// Neither side can be followed further: compare the structure.
	switch x := a.expr.(type) {
	case *ast.StarExpr:
		if y, ok := b.expr.(*ast.StarExpr); ok {
			return ix.compareTypes(a.with(x.X), b.with(y.X))
		}
	case *ast.ArrayType:
		if y, ok := b.expr.(*ast.ArrayType); ok && (x.Len == nil) == (y.Len == nil) && (x.Len == nil || types.ExprString(x.Len) == types.ExprString(y.Len)) {
			return ix.compareTypes(a.with(x.Elt), b.with(y.Elt))
		}
	case *ast.MapType:
		if y, ok := b.expr.(*ast.MapType); ok {
			if same, reason := ix.compareTypes(a.with(x.Key), b.with(y.Key)); !same {
				return false, reason
			}
			return ix.compareTypes(a.with(x.Value), b.with(y.Value))
		}
	case *ast.ChanType:
		if y, ok := b.expr.(*ast.ChanType); ok && x.Dir == y.Dir {
			return ix.compareTypes(a.with(x.Value), b.with(y.Value))
		}
	case *ast.IndexExpr, *ast.IndexListExpr:
		baseA, argsA := instance(a.expr)
		baseB, argsB := instance(b.expr)
		if argsB != nil && len(argsA) == len(argsB) {
			if same, reason := ix.compareTypes(a.with(baseA), b.with(baseB)); !same {
				return false, reason
			}
			for index := range argsA {
				if same, reason := ix.compareTypes(a.with(argsA[index]), b.with(argsB[index])); !same {
					return false, reason
				}
			}
			return true, ""
		}
	case *ast.Ident, *ast.SelectorExpr:
		// Two names that differ, or a name and another expression.
	default:
		// Other type literals are compared as written.
		if types.ExprString(a.expr) == types.ExprString(b.expr) {
			return true, ""
		}
	}
	if reasonA != "" {
		return false, reasonA
	}
	return false, reasonB
}

// with returns the side for a part of its expression.
func (side typeSide) with(expr ast.Expr) typeSide {
	side.expr = expr
	return side
}

// instance splits an instantiated generic type into its type and its type
// arguments; args is nil for any other expression.
func instance(expr ast.Expr) (ast.Expr, []ast.Expr) {
	switch e := expr.(type) {
	case *ast.IndexExpr:
		return e.X, []ast.Expr{e.Index}
	case *ast.IndexListExpr:
		return e.X, e.Indices
	}
	return nil, nil
}

// resolveName reads a side whose expression is a name. It returns the
// name's identity, the side its alias leads to when it can be followed, and
// otherwise why it cannot. For any other expression, identity is empty.
func (ix *packageIndex) resolveName(side typeSide) (identity string, next *typeSide, reason string) {
	switch e := side.expr.(type) {
	case *ast.Ident:
		name := e.Name
		identity = "local:" + name
		decls := ix.types[name]
		switch {
		case len(decls) == 0 && predeclared[name]:
			return "universe:" + name, nil, ""
		case len(decls) == 0 && side.file != nil && side.file.dotImport():
			// Only the same file's dot imports give the name one meaning.
			return "dot:" + side.file.name + ":" + name, nil, fmt.Sprintf("%s's right side uses %s from a dot import in %s; import that package with a name", side.alias, name, side.file.name)
		case len(decls) == 0:
			return identity, nil, fmt.Sprintf("%s is not declared", name)
		case len(decls) > 1:
			return identity, nil, fmt.Sprintf("%s is declared more than once", name)
		}
		decl := decls[0]
		switch {
		case !decl.spec.Assign.IsValid():
			return identity, nil, fmt.Sprintf("%s is declared as a type of its own, not as an alias", name)
		case decl.spec.TypeParams != nil:
			return identity, nil, fmt.Sprintf("%s has type parameters", name)
		case slices.Contains(side.chain, name):
			cycle := append(side.chain[slices.Index(side.chain, name):], name)
			return identity, nil, fmt.Sprintf("the alias %s is in a cycle of aliases: %s", name, strings.Join(cycle, ", "))
		}
		return identity, &typeSide{
			expr:      ast.Unparen(decl.spec.Type),
			qualifier: decl.file.importPath,
			file:      decl.file,
			alias:     name,
			chain:     append(slices.Clip(side.chain), name),
		}, ""
	case *ast.SelectorExpr:
		pkg, ok := e.X.(*ast.Ident)
		if !ok {
			return "", nil, ""
		}
		spelled := pkg.Name + "." + e.Sel.Name
		path, found := side.qualifier(pkg.Name)
		if !found {
			// Only the same file's imports give the name one meaning. A
			// specification's package names are all imported, as reading
			// checks, so only an alias's right side gets here.
			if side.file == nil {
				return "", nil, ""
			}
			return "unknown:" + side.file.name + ":" + spelled, nil, fmt.Sprintf("%s's right side uses %s, but the generator cannot tell which import of %s provides %s; give that import the explicit name %s", side.alias, pkg.Name, side.file.name, pkg.Name, pkg.Name)
		}
		return "path:" + path + "." + e.Sel.Name, nil, fmt.Sprintf("%s is declared in another package, whose aliases the generator does not read", spelled)
	}
	return "", nil, ""
}

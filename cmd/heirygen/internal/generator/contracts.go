package generator

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
)

// A packageIndex holds the type declarations of the handwritten files of one
// package clause. Contracts are read from it by syntax alone: the generator
// never type-checks. What it reads is verified afterwards by the compiler,
// through the equality assertions the generated file carries.
type packageIndex struct {
	fset  *token.FileSet
	types map[string][]*typeDecl
}

type typeDecl struct {
	spec *ast.TypeSpec
	file *fileInfo
}

type fileInfo struct {
	name    string
	imports []fileImport
}

// dotImport reports whether the file has an import whose names are used
// without a qualifier.
func (f *fileInfo) dotImport() bool {
	for _, declared := range f.imports {
		if declared.name == "." {
			return true
		}
	}
	return false
}

type fileImport struct {
	name string // explicit name, or "" when the import has none
	path string
}

// A method is one method of a contract, as written.
type method struct {
	name      string
	signature string   // parameters and results, as written: "(msg string) error"
	params    []param  // flattened parameters
	results   []string // result types, as written, one per result
	imports   []specImport
	pos       token.Position
	file      *fileInfo
	// unqualified lists the unqualified names the signature's types use.
	unqualified []string
}

type param struct {
	name     string // "" when the parameter is unnamed
	typ      string // as written, including "..." for a variadic parameter
	variadic bool
}

// sameShape reports whether two spellings of a method could be the same Go
// signature: the same number of parameters and results, and the same
// variadic form. Whether the types are identical is not decided here. Names
// of parameters, of results, and of types can differ between two spellings
// of one signature, so only the compiler can tell; it does, through the
// equality assertion the generated file carries.
func (m method) sameShape(other method) bool {
	if len(m.params) != len(other.params) || len(m.results) != len(other.results) {
		return false
	}
	for index := range m.params {
		if m.params[index].variadic != other.params[index].variadic {
			return false
		}
	}
	return true
}

func (ix *packageIndex) print(node ast.Node) string {
	var b bytes.Buffer
	if err := printer.Fprint(&b, ix.fset, node); err != nil {
		return ""
	}
	return b.String()
}

// contractMethods returns the methods of the named contract in declaration
// order, following embedded contracts and aliases declared in the package.
func (ix *packageIndex) contractMethods(name string, use token.Position) ([]method, error) {
	var methods []method
	index := make(map[string]int)
	if err := ix.collect(name, use, map[string]bool{}, &methods, index); err != nil {
		return nil, err
	}
	return methods, nil
}

func (ix *packageIndex) collect(name string, use token.Position, visiting map[string]bool, methods *[]method, index map[string]int) error {
	decls := ix.types[name]
	if len(decls) == 0 {
		return errorAt(use, "contract %s is not declared in the handwritten files of this package", name)
	}
	if len(decls) > 1 {
		return errorAt(use, "contract %s is declared in %s and in %s; the generator does not evaluate build constraints, so a contract it reads is declared once", name, decls[0].file.name, decls[1].file.name)
	}
	if visiting[name] {
		return errorAt(use, "contract %s embeds itself", name)
	}
	visiting[name] = true
	defer delete(visiting, name)

	decl := decls[0]
	pos := ix.fset.Position(decl.spec.Pos())
	if decl.spec.TypeParams != nil {
		return errorAt(pos, "contract %s is generic, which the generator does not support", name)
	}
	switch typ := decl.spec.Type.(type) {
	case *ast.Ident:
		// An alias or a definition from another contract of the package.
		return ix.collect(typ.Name, pos, visiting, methods, index)
	case *ast.InterfaceType:
		for _, field := range typ.Methods.List {
			fn, isMethod := field.Type.(*ast.FuncType)
			switch {
			case isMethod && len(field.Names) == 1:
				m, err := ix.method(decl.file, field.Names[0], fn)
				if err != nil {
					return err
				}
				if known, exists := index[m.name]; exists {
					// Overlapping embedded methods are valid Go when their
					// signatures are identical. The first spelling is kept.
					if !(*methods)[known].sameShape(m) {
						return errorAt(m.pos, "method %s has another number of parameters or results here than at %s", m.name, (*methods)[known].pos)
					}
					continue
				}
				index[m.name] = len(*methods)
				*methods = append(*methods, m)
			case !isMethod && len(field.Names) == 0:
				switch embedded := field.Type.(type) {
				case *ast.Ident:
					if err := ix.collect(embedded.Name, ix.fset.Position(field.Pos()), visiting, methods, index); err != nil {
						return err
					}
				case *ast.SelectorExpr:
					return errorAt(ix.fset.Position(field.Pos()), "contract %s embeds %s, whose methods cannot be read from this package; write its methods out in %s, and add an existing line if %s identifies the operation",
						name, ix.print(field.Type), name, ix.print(field.Type))
				default:
					return errorAt(ix.fset.Position(field.Pos()), "contract %s has the element %s; the generator reads only methods and embedded interfaces declared in this package", name, ix.print(field.Type))
				}
			default:
				return errorAt(ix.fset.Position(field.Pos()), "unsupported element in contract %s", name)
			}
		}
		return nil
	default:
		return errorAt(pos, "%s is declared as %s; the generator reads only interfaces declared in this package", name, ix.print(decl.spec.Type))
	}
}

func (ix *packageIndex) method(file *fileInfo, name *ast.Ident, fn *ast.FuncType) (method, error) {
	m := method{name: name.Name, pos: ix.fset.Position(name.Pos()), file: file}
	if !name.IsExported() {
		return m, errorAt(m.pos, "operation %s is not exported", m.name)
	}
	if fn.TypeParams != nil {
		return m, errorAt(m.pos, "method %s has type parameters", m.name)
	}
	m.signature = strings.TrimPrefix(ix.print(fn), "func")
	if fn.Results != nil {
		for _, field := range fn.Results.List {
			typ := ix.print(field.Type)
			for range max(1, len(field.Names)) {
				m.results = append(m.results, typ)
			}
		}
	}
	for _, field := range fn.Params.List {
		_, variadic := field.Type.(*ast.Ellipsis)
		typ := ix.print(field.Type)
		if len(field.Names) == 0 {
			m.params = append(m.params, param{typ: typ, variadic: variadic})
		}
		for _, ident := range field.Names {
			m.params = append(m.params, param{name: ident.Name, typ: typ, variadic: variadic})
		}
	}
	for _, list := range []*ast.FieldList{fn.Params, fn.Results} {
		if list != nil {
			for _, field := range list.List {
				m.unqualified = typeNames(field.Type, m.unqualified)
			}
		}
	}
	// Every qualified identifier of the signature needs its import in the
	// generated file.
	var failure error
	seen := make(map[string]bool)
	ast.Inspect(fn, func(n ast.Node) bool {
		selector, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := selector.X.(*ast.Ident)
		if !ok || seen[pkg.Name] {
			return false
		}
		seen[pkg.Name] = true
		path, found := file.importPath(pkg.Name)
		if !found {
			failure = errorAt(m.pos, "cannot tell which import of %s provides %s; give that import the explicit name %s", file.name, pkg.Name, pkg.Name)
			return false
		}
		m.imports = append(m.imports, specImport{name: pkg.Name, path: path})
		return false
	})
	return m, failure
}

// importPath finds the import a package identifier refers to: by explicit
// name, or by the last element of an unnamed import's path.
func (f *fileInfo) importPath(name string) (string, bool) {
	for _, declared := range f.imports {
		if declared.name == name {
			return declared.path, true
		}
	}
	for _, declared := range f.imports {
		if declared.name == "" && declared.path[strings.LastIndexByte(declared.path, '/')+1:] == name {
			return declared.path, true
		}
	}
	return "", false
}

// resolveOperations fills an operation set from the package's contracts.
func (ix *packageIndex) resolveOperations(s *spec, set *operationSet) error {
	fail := func(format string, args ...any) error {
		return s.lineError(set.line, format, args...)
	}
	use := s.lines.position(set.line)
	existing := make(map[string]existingOp)
	for _, entry := range set.existing {
		existing[entry.method] = entry
	}
	set.contractOps = make(map[string][]operation)
	byMethod := make(map[string]operation)
	used := make(map[string]bool)
	for _, contract := range set.contracts {
		if _, twice := set.contractOps[contract]; twice {
			return fail("contract %s is listed twice", contract)
		}
		methods, err := ix.contractMethods(contract, use)
		if err != nil {
			return err
		}
		ops := []operation{}
		for _, m := range methods {
			op, known := byMethod[m.name]
			if known && !op.method.sameShape(m) {
				return fail("method %s has another number of parameters or results at %s than at %s; one operations line gives one interface per method name",
					m.name, m.pos, op.method.pos)
			}
			if !known {
				op = operation{method: m}
				if entry, given := existing[m.name]; given {
					op.typ = entry.typ
					op.key = ix.typeKey(entry.typ, s.qualifier)
					op.typeImports = s.importsOf(entry.typ)
					op.pos = s.lines.position(entry.line)
					op.decl = fmt.Sprintf("%s, line %d: existing %s %s", s.constName, entry.line, m.name, entry.typ)
					used[m.name] = true
				} else {
					if set.prefix == "" && set.suffix == "" {
						return fail("give prefix= or suffix= to name the interface of method %s", m.name)
					}
					op.typ, op.generated = set.prefix+m.name+set.suffix, true
					op.key = localKey(op.typ)
					op.pos = s.lines.position(set.line)
					op.decl = fmt.Sprintf("%s, line %d: the operations line that generates %s", s.constName, set.line, op.typ)
					if !token.IsIdentifier(op.typ) {
						return fail("%s is not a valid name for the interface of method %s", op.typ, m.name)
					}
				}
				byMethod[m.name] = op
				set.ops = append(set.ops, op)
			}
			// The contract's own operation keeps the one interface that
			// identifies the method, for dispatch, and the contract's own
			// spelling of the signature, which code that names the contract,
			// such as its wrapper, can compile wherever the contract is.
			own := op
			own.method = m
			ops = append(ops, own)
		}
		set.contractOps[contract] = ops
	}
	for _, entry := range set.existing {
		if !used[entry.method] {
			return fail("existing %s: no listed contract has a method %s", entry.method, entry.method)
		}
	}
	return nil
}

// typeNames appends the unqualified identifiers that a type expression uses
// as type or constant names. Field, method, and parameter names inside the
// expression are not included, and neither are qualified names.
func typeNames(expr ast.Expr, names []string) []string {
	fields := func(list *ast.FieldList) {
		if list != nil {
			for _, field := range list.List {
				names = typeNames(field.Type, names)
			}
		}
	}
	switch expr := expr.(type) {
	case *ast.Ident:
		names = append(names, expr.Name)
	case *ast.SelectorExpr:
	case *ast.StarExpr:
		names = typeNames(expr.X, names)
	case *ast.ParenExpr:
		names = typeNames(expr.X, names)
	case *ast.Ellipsis:
		names = typeNames(expr.Elt, names)
	case *ast.ArrayType:
		if expr.Len != nil {
			// A length is a constant expression; its unqualified names
			// count, its qualified ones do not.
			ast.Inspect(expr.Len, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.SelectorExpr:
					return false
				case *ast.Ident:
					names = append(names, n.Name)
				}
				return true
			})
		}
		names = typeNames(expr.Elt, names)
	case *ast.MapType:
		names = typeNames(expr.Value, typeNames(expr.Key, names))
	case *ast.ChanType:
		names = typeNames(expr.Value, names)
	case *ast.FuncType:
		fields(expr.Params)
		fields(expr.Results)
	case *ast.StructType:
		fields(expr.Fields)
	case *ast.InterfaceType:
		fields(expr.Methods)
	case *ast.IndexExpr:
		names = typeNames(expr.Index, typeNames(expr.X, names))
	case *ast.IndexListExpr:
		names = typeNames(expr.X, names)
		for _, index := range expr.Indices {
			names = typeNames(index, names)
		}
	case *ast.UnaryExpr:
		names = typeNames(expr.X, names)
	case *ast.BinaryExpr:
		names = typeNames(expr.Y, typeNames(expr.X, names))
	}
	return names
}

// predeclared holds Go's predeclared identifiers.
var predeclared = map[string]bool{
	"any": true, "bool": true, "byte": true, "comparable": true, "complex64": true, "complex128": true,
	"error": true, "float32": true, "float64": true, "int": true, "int8": true, "int16": true,
	"int32": true, "int64": true, "rune": true, "string": true, "uint": true, "uint8": true,
	"uint16": true, "uint32": true, "uint64": true, "uintptr": true,
	"true": true, "false": true, "iota": true, "nil": true,
	"append": true, "cap": true, "clear": true, "close": true, "complex": true, "copy": true,
	"delete": true, "imag": true, "len": true, "make": true, "max": true, "min": true, "new": true,
	"panic": true, "print": true, "println": true, "real": true, "recover": true,
}

// typeKey returns a key that is equal for two type expressions that denote
// the same type, as far as syntax can tell: aliases declared in the package
// are followed, and a qualified name is keyed by its import path. qualifier
// resolves the package names expr uses. Two expressions with different keys
// may still denote one type, for example through an alias declared in
// another package; generated identity assertions let the compiler decide
// where spellings differ.
func (ix *packageIndex) typeKey(expr string, qualifier func(string) (string, bool)) string {
	return ix.keyOf(expr, qualifier, 0)
}

func localKey(name string) string { return "local:" + name }

func (ix *packageIndex) keyOf(text string, qualifier func(string) (string, bool), depth int) string {
	expr, err := parser.ParseExpr(text)
	if err != nil {
		return "text:" + text
	}
	switch e := expr.(type) {
	case *ast.Ident:
		decls := ix.types[e.Name]
		if len(decls) == 1 && decls[0].spec.Assign.IsValid() && decls[0].spec.TypeParams == nil && depth < 32 {
			return ix.keyOf(ix.print(decls[0].spec.Type), decls[0].file.importPath, depth+1)
		}
		if len(decls) == 0 && predeclared[e.Name] {
			return "universe:" + e.Name
		}
		return localKey(e.Name)
	case *ast.SelectorExpr:
		if pkg, ok := e.X.(*ast.Ident); ok {
			if path, found := qualifier(pkg.Name); found {
				return "import:" + path + "." + e.Sel.Name
			}
		}
	}
	return "text:" + text
}

// singleMethod returns the method of an interface declared in the package
// with exactly one method, following local aliases. It reports false for
// anything else, including interfaces of other packages.
func (ix *packageIndex) singleMethod(expr string, qualifier func(string) (string, bool)) (string, bool) {
	key := ix.typeKey(expr, qualifier)
	name, local := strings.CutPrefix(key, "local:")
	if !local {
		return "", false
	}
	methods, err := ix.contractMethods(name, token.Position{})
	if err != nil || len(methods) != 1 {
		return "", false
	}
	return methods[0].name, true
}

// embeddedNodes returns the names of the directly embedded heiry.Node value
// fields of the leaf type. It reads the struct the leaf is declared as,
// following local aliases and type definitions, such as type node record,
// and generic instantiations to their generic declaration. An alias of
// heiry.Node declared in the package, such as type Dispatch = heiry.Node,
// embeds under the alias name. When the struct cannot be read, it returns
// the reason instead.
func (ix *packageIndex) embeddedNodes(leaf string, qualifier func(string) (string, bool)) ([]string, string) {
	text := leaf
	for depth := 0; depth < 32; depth++ {
		expr, err := parser.ParseExpr(text)
		if err != nil {
			break
		}
		// An instantiation has the fields of its generic declaration; a Node
		// value field does not depend on the type arguments.
		switch index := expr.(type) {
		case *ast.IndexExpr:
			expr = index.X
		case *ast.IndexListExpr:
			expr = index.X
		}
		ident, ok := expr.(*ast.Ident)
		if !ok {
			break
		}
		decls := ix.types[ident.Name]
		if len(decls) != 1 {
			break
		}
		decl := decls[0]
		structure, isStruct := decl.spec.Type.(*ast.StructType)
		if !isStruct {
			// An alias or a definition from another type: read that one, in
			// the file that declares it.
			text, qualifier = ix.print(decl.spec.Type), decl.file.importPath
			continue
		}
		var names []string
		for _, field := range structure.Fields.List {
			if len(field.Names) != 0 {
				continue
			}
			var fieldName string
			switch typ := field.Type.(type) {
			case *ast.Ident:
				fieldName = typ.Name
			case *ast.SelectorExpr:
				fieldName = typ.Sel.Name
			default:
				continue // pointers and instantiations are not a Node value
			}
			if ix.typeKey(ix.print(field.Type), decl.file.importPath) == "import:"+heiryPath+".Node" {
				names = append(names, fieldName)
			}
		}
		switch len(names) {
		case 1:
			return names, ""
		case 0:
			return nil, fmt.Sprintf("%s embeds no field that the generator can show is heiry.Node", leaf)
		default:
			return nil, fmt.Sprintf("%s embeds more than one heiry.Node", leaf)
		}
	}
	return nil, fmt.Sprintf("%s is not a struct type declared in this package, so the generator cannot read its fields", leaf)
}

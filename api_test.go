package heiry

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"testing"
)

// The exported API is the approved one, without the aliases that ADR 0015
// removed: exported types, functions, methods, constants, variables, struct
// fields and interface methods, by name. Adding or removing one fails this
// test until the list, and the decision behind it, are updated; a changed
// signature does not. An exported struct may not embed a type, and an
// exported type may not be an alias of an unexported type or be defined
// over one: the members those bring this test could not list.
func TestExportedAPI(t *testing.T) {
	want := []string{
		"Code", "Define", "Definition", "Error", "LevelHandle", "Method", "Node", "Of", "OperationToken", "ParentMethod", "Plan", "ProviderRef", "Scope",
		"AlreadyInitialized", "BindingMismatch", "CapabilityIdentity", "ContractCollision", "ContractExtension", "DuplicateBinding",
		"InvalidCapability", "InvalidContract", "InvalidDeclaration", "InvalidHierarchy", "InvalidPlan", "InvalidProvider",
		"MissingMethods", "MissingNode", "NoParentImplementation", "ProviderOwnership", "SignatureMismatch", "UndeclaredOperation",
		"UnknownCapability", "Uninitialized",
		"Error.Actual", "Error.Capability", "Error.Category", "Error.Cause", "Error.Code", "Error.Expected", "Error.Level",
		"Error.Name", "Error.Path", "Error.Target",
		"Definition.ComponentProvider", "Definition.Extend", "Definition.ExtendWith", "Definition.Init", "Definition.Level",
		"Definition.MustInit", "Definition.MustPrepare", "Definition.NodeProvider", "Definition.Prepare", "Definition.Validate",
		"Error.Error", "Error.Is", "Error.Unwrap",
		"LevelHandle.Bind", "LevelHandle.BindGroup", "LevelHandle.End",
		"Node.Parent", "Node.Scope", "Node.Virtual",
		"Plan.ComponentProvider", "Plan.Init", "Plan.MustInit", "Plan.MustPrecompute", "Plan.NodeProvider", "Plan.Precompute",
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	// The unexported types of the package, whose members an exported alias or
	// definition over them would bring in.
	hidden := make(map[string]bool)
	for _, file := range files {
		for _, decl := range file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok {
				for _, spec := range gen.Specs {
					if typ, ok := spec.(*ast.TypeSpec); ok && !typ.Name.IsExported() {
						hidden[typ.Name.Name] = true
					}
				}
			}
		}
	}
	var got []string
	for _, file := range files {
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if !decl.Name.IsExported() {
					continue
				}
				if decl.Recv == nil {
					got = append(got, decl.Name.Name)
					continue
				}
				receiver := decl.Recv.List[0].Type
				if star, ok := receiver.(*ast.StarExpr); ok {
					receiver = star.X
				}
				switch generic := receiver.(type) {
				case *ast.IndexExpr:
					receiver = generic.X
				case *ast.IndexListExpr:
					receiver = generic.X
				}
				ident, ok := receiver.(*ast.Ident)
				if !ok {
					t.Fatalf("%s: method %s has a receiver this test cannot read", fset.Position(decl.Pos()), decl.Name.Name)
				}
				if ident.IsExported() {
					got = append(got, ident.Name+"."+decl.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					switch spec := spec.(type) {
					case *ast.ValueSpec:
						for _, ident := range spec.Names {
							if ident.IsExported() {
								got = append(got, ident.Name)
							}
						}
					case *ast.TypeSpec:
						if !spec.Name.IsExported() {
							continue
						}
						got = append(got, spec.Name.Name)
						if base := typeBase(spec.Type); hidden[base] {
							t.Errorf("%s: exported type %s is an alias of, or defined over, the unexported type %s, whose members this test cannot list", fset.Position(spec.Pos()), spec.Name.Name, base)
						}
						var members *ast.FieldList
						switch typ := spec.Type.(type) {
						case *ast.StructType:
							members = typ.Fields
						case *ast.InterfaceType:
							members = typ.Methods
						}
						if members == nil {
							continue
						}
						for _, field := range members.List {
							if len(field.Names) == 0 {
								t.Errorf("%s: exported type %s embeds a type, whose promoted members this test cannot list", fset.Position(field.Pos()), spec.Name.Name)
							}
							for _, ident := range field.Names {
								if ident.IsExported() {
									got = append(got, spec.Name.Name+"."+ident.Name)
								}
							}
						}
					}
				}
			}
		}
	}
	slices.Sort(got)
	slices.Sort(want)
	var extra, missing []string
	for _, name := range got {
		if !slices.Contains(want, name) {
			extra = append(extra, name)
		}
	}
	for _, name := range want {
		if !slices.Contains(got, name) {
			missing = append(missing, name)
		}
	}
	if len(extra) != 0 || len(missing) != 0 {
		t.Fatalf("exported API differs from the approved one:\n extra:   %v\n missing: %v", extra, missing)
	}
}

// typeBase returns the name a type expression is built on, through pointers,
// type arguments, and parentheses, or "" when it is not a named type.
func typeBase(expr ast.Expr) string {
	for {
		switch typ := expr.(type) {
		case *ast.Ident:
			return typ.Name
		case *ast.StarExpr:
			expr = typ.X
		case *ast.IndexExpr:
			expr = typ.X
		case *ast.IndexListExpr:
			expr = typ.X
		case *ast.ParenExpr:
			expr = typ.X
		default:
			return ""
		}
	}
}

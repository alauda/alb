package tylua

import (
	"go/ast"
	"go/token"
	"log"
	"strings"
)

func (g *PackageGenerator) Resovle() {
	for _, file := range g.pkg.Syntax {
		// log.Printf("gen for file %v", file.Name)
		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			// GenDecl can be an import, type, var, or const expression
			case *ast.GenDecl:
				if x.Tok == token.IMPORT {
					return false
				}
				g.ResovleFile(x)
				return false
			}
			return true
		})
	}
}

func (g *PackageGenerator) IsPending(name string) bool {
	return g.pendingTypes[name]
}

func (g *PackageGenerator) AddPending(name string) {
	kn := map[string]bool{
		"int":    true,
		"string": true,
		"bool":   true,
		"uint":   true,
	}
	if _, ok := kn[name]; ok {
		return
	}
	if g.types[name] != "" {
		return
	}
	g.pendingTypes[name] = true
}

func (g *PackageGenerator) ResovleFile(decl *ast.GenDecl) {
	for _, spec := range decl.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if ok && ts.Name.IsExported() {
			if g.IsPending(ts.Name.Name) {
				log.Printf("resolve %v", ts.Name.Name)
				g.ResovleTypeSpec(ts)
			}
		}
	}
}

func (g *PackageGenerator) ResovleTypeSpec(spec *ast.TypeSpec) {
	name := spec.Name.Name
	s := strings.Builder{}
	ok := g.writeTypeSpec(&s, spec)
	if !ok {
		return
	}
	g.types[name] = s.String()
	log.Printf("resolved %v", name)
	if g.typesOrder[name] == 0 {
		g.typesOrder[name] = len(g.typesOrder) + 1
	}
	g.pendingTypes[name] = false
}

func (g *PackageGenerator) ShowPend() string {
	out := ""
	for k, v := range g.pendingTypes {
		if v {
			out += k
		}
	}
	return out
}

package tylua

import (
	"fmt"
	"go/ast"
	"go/token"
	"log"
	"strings"
)

type groupContext struct {
	isGroupedDeclaration bool
	doc                  *ast.CommentGroup
	groupValue           string
	groupType            string
	iotaValue            int
}

func (g *PackageGenerator) writeGroupDecl(s *strings.Builder, decl *ast.GenDecl) {
	// This checks whether the declaration is a group declaration like:
	// const (
	// 	  X = 3
	//    Y = "abc"
	// )
	isGroupedDeclaration := len(decl.Specs) > 1

	if !isGroupedDeclaration && g.PreserveTypeComments() {
		g.writeCommentGroupIfNotNil(s, decl.Doc, 0)
	}

	// We need a bit of state to handle syntax like
	// const (
	//   X SomeType = iota
	//   _
	//   Y
	//   Foo string = "Foo"
	//   _
	//   AlsoFoo
	// )
	group := &groupContext{
		isGroupedDeclaration: len(decl.Specs) > 1,
		doc:                  decl.Doc,
		groupType:            "",
		groupValue:           "",
		iotaValue:            -1,
	}

	for _, spec := range decl.Specs {
		g.writeSpec(s, spec, group)
	}
}

func (g *PackageGenerator) writeSpec(s *strings.Builder, spec ast.Spec, group *groupContext) {
	// e.g. "type Foo struct {}" or "type Bar = string"
	ts, ok := spec.(*ast.TypeSpec)
	if ok && ts.Name.IsExported() {
		g.writeTypeSpec(s, ts)
	}
}

func showType(t ast.Expr) string {
	switch t := t.(type) {
	case *ast.Ident:
		return fmt.Sprintf("ident %v", t)
	case *ast.ArrayType:
		return fmt.Sprintf("array %v", t)
	case *ast.StructType:
		return fmt.Sprintf("struct %v", t)
	case *ast.SelectorExpr:
		return fmt.Sprintf("selector %v", t)
	case *ast.StarExpr:
		return fmt.Sprintf("star %v", t)
	case *ast.MapType:
		return fmt.Sprintf("map %v", t)
	}
	return "unknown"
}

// Writing of type specs, which are expressions like
// `type X struct { ... }`
// or
// `type Bar = string`
func (g *PackageGenerator) writeTypeSpec(
	s *strings.Builder,
	ts *ast.TypeSpec,
) bool {
	if ts.Doc != nil &&
		g.PreserveTypeComments() { // The spec has its own comment, which overrules the grouped comment.
		g.writeCommentGroup(s, ts.Doc, 0)
	}
	isok := false
	log.Printf("type %s %s ", ts.Name, showType(ts.Type))
	switch at := ts.Type.(type) {
	case *ast.Ident:
		log.Printf("ident %v", at.Name)
		s.WriteString(at.Name)
		g.AddPending(at.Name)
		isok = true
	case *ast.ArrayType:
		log.Printf("array %v", ts.Name.Name)
		s.WriteString(fmt.Sprintf("---@alias %s ", ts.Name.Name))
		g.writeType(s, at.Elt, at, 0, true)
		s.WriteString("[]")
		isok = true
	case *ast.StructType:
		s.WriteString("---@class ")
		s.WriteString(ts.Name.Name)
		s.WriteString("\n")
		g.writeStructFields(s, at.Fields.List, 0)
		s.WriteString("\n")
		isok = true
	case *ast.StarExpr:
		g.writeType(s, at.X, at, 0, true)
		isok = true
	case *ast.MapType:
		log.Printf("maptype %s %s", ts.Name, at.Value)
		s.WriteString(fmt.Sprintf("---@alias %s table<string,%s>\n", ts.Name.Name, at.Value))
		g.AddPending(fmt.Sprintf("%s", at.Value))
		isok = true
	}
	if ts.Comment != nil && g.PreserveTypeComments() {
		g.writeSingleLineComment(s, ts.Comment)
	} else {
		s.WriteString("\n")
	}
	return isok
}

func getAnonymousFieldName(f ast.Expr) (name string, valid bool) {
	switch ft := f.(type) {
	case *ast.Ident:
		name = ft.Name
		if ft.Obj != nil && ft.Obj.Decl != nil {
			dcl, ok := ft.Obj.Decl.(*ast.TypeSpec)
			if ok {
				valid = dcl.Name.IsExported()
			}
		} else {
			// Types defined in the Go file after the parsed file in the same package
			valid = token.IsExported(name)
		}
	case *ast.IndexExpr:
		return getAnonymousFieldName(ft.X)
	case *ast.IndexListExpr:
		return getAnonymousFieldName(ft.X)
	case *ast.SelectorExpr:
		valid = ft.Sel.IsExported()
		name = ft.Sel.String()
	case *ast.StarExpr:
		return getAnonymousFieldName(ft.X)
	}

	return
}

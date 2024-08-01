package tylua

import (
	"fmt"
	"go/ast"
	"go/token"
	"log"
	"reflect"
	"regexp"
	"strings"

	"github.com/fatih/structtag"
)

var validJSNameRegexp = regexp.MustCompile(`(?m)^[\pL_][\pL\pN_]*$`)
var backquoteEscapeRegexp = regexp.MustCompile(`([$\\])`)
var octalPrefixRegexp = regexp.MustCompile(`^0[0-7]`)
var unicode8Regexp = regexp.MustCompile(`\\\\|\\U[\da-fA-F]{8}`)

// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/Operator_precedence#table
var jsNumberOperatorPrecedence = map[token.Token]int{
	token.MUL:     6,
	token.QUO:     6,
	token.REM:     6,
	token.ADD:     5,
	token.SUB:     5,
	token.SHL:     4,
	token.SHR:     4,
	token.AND:     3,
	token.AND_NOT: 3,
	token.OR:      2,
	token.XOR:     1,
}

func validJSName(n string) bool {
	return validJSNameRegexp.MatchString(n)
}

func getIdent(s string) string {
	switch s {
	case "bool":
		return "boolean"
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64",
		"complex64", "complex128":
		return "number /* " + s + " */"
	}
	return s
}

func (g *PackageGenerator) writeIndent(s *strings.Builder, depth int) {
	for i := 0; i < depth; i++ {
		s.WriteString(g.conf.Indent)
	}
}

func (g *PackageGenerator) writeType(
	s *strings.Builder,
	t ast.Expr,
	p ast.Expr,
	depth int,
	optionalParens bool,
) {
	log.Println("writeType:", reflect.TypeOf(t), t)
	switch t := t.(type) {
	case *ast.StarExpr:
		if optionalParens {
			s.WriteByte('(')
		}
		g.writeType(s, t.X, t, depth, false)
		s.WriteString(" | nil")
		if optionalParens {
			s.WriteByte(')')
		}
	case *ast.ArrayType:
		if v, ok := t.Elt.(*ast.Ident); ok && v.String() == "byte" {
			s.WriteString("string")
			break
		}
	case *ast.InterfaceType:
		s.WriteString(" table ")
	case *ast.StructType:
		s.WriteString("{\n")
		g.writeStructFields(s, t.Fields.List, depth+1)
		g.writeIndent(s, depth+1)
		s.WriteByte('}')
	case *ast.Ident:
		if t.String() == "any" {
			s.WriteString(getIdent(g.conf.FallbackType))
		} else {
			log.Printf("check pending %v", t)
			if _, has := g.types[t.String()]; !has {
				log.Printf("add pending %v", t)
				g.AddPending(t.String())
			}
			s.WriteString(getIdent(t.String()))
		}
	case *ast.SelectorExpr:
		longType := fmt.Sprintf("%s.%s", t.X, t.Sel)
		log.Printf("SelectorExpr %v", longType)
		mappedTsType, ok := g.conf.TypeMappings[longType]
		if ok {
			s.WriteString(mappedTsType)
		} else { // For unknown types we use the fallback type
			log.Printf("import x.x %v", t.Sel.String())
			s.WriteString(t.Sel.String())
			if _, has := g.types[t.Sel.String()]; !has {
				log.Printf("add pending %v", t)
				g.AddPending(t.Sel.String())
			}
		}
	case *ast.MapType:
		s.WriteString("table<")
		g.writeType(s, t.Key, t, depth, false)
		s.WriteString(",")
		g.writeType(s, t.Value, t, depth, false)
		s.WriteString(">")
	case *ast.ParenExpr:
		s.WriteByte('(')
		g.writeType(s, t.X, t, depth, false)
		s.WriteByte(')')
	default:
		err := fmt.Errorf("unhandled: %s\n %T", t, t)
		fmt.Println(err)
		panic(err)
	}
}

func (g *PackageGenerator) writeStructFields(s *strings.Builder, fields []*ast.Field, depth int) {
	for _, f := range fields {
		// fmt.Println(f.Type)
		optional := false
		required := false
		readonly := false

		var fieldName string
		if len(f.Names) == 0 { // anonymous field
			if name, valid := getAnonymousFieldName(f.Type); valid {
				fieldName = name
			}
		}
		if len(f.Names) != 0 && f.Names[0] != nil && len(f.Names[0].Name) != 0 {
			fieldName = f.Names[0].Name
		}
		if len(fieldName) == 0 || 'A' > fieldName[0] || fieldName[0] > 'Z' {
			continue
		}

		var name string
		var tstype string
		if f.Tag != nil {
			tags, err := structtag.Parse(f.Tag.Value[1 : len(f.Tag.Value)-1])
			if err != nil {
				panic(err)
			}

			jsonTag, err := tags.Get("json")
			if err == nil {
				name = jsonTag.Name
				if name == "-" {
					continue
				}

				optional = jsonTag.HasOption("omitempty")
			}
			yamlTag, err := tags.Get("yaml")
			if err == nil {
				name = yamlTag.Name
				if name == "-" {
					continue
				}

				optional = yamlTag.HasOption("omitempty")
			}

			tstypeTag, err := tags.Get("tstype")
			if err == nil {
				tstype = tstypeTag.Name
				if tstype == "-" || tstypeTag.HasOption("extends") {
					continue
				}
				required = tstypeTag.HasOption("required")
				readonly = tstypeTag.HasOption("readonly")
			}
		}

		if len(name) == 0 {
			if g.conf.Flavor == "yaml" {
				name = strings.ToLower(fieldName)
			} else {
				name = fieldName
			}
		}

		if g.PreserveTypeComments() {
			g.writeCommentGroupIfNotNil(s, f.Doc, depth+1)
		}

		quoted := !validJSName(name)
		if quoted {
			s.WriteByte('\'')
		}
		if readonly {
			s.WriteString("readonly ")
		}
		log.Printf("field %v", name)
		s.WriteString("---@field " + name)
		if quoted {
			s.WriteByte('\'')
		}

		switch t := f.Type.(type) {
		case *ast.StarExpr:
			optional = !required
			f.Type = t.X
		}

		if optional {
			s.WriteByte('?')
		}

		s.WriteString(" ")

		if tstype == "" {
			g.writeType(s, f.Type, nil, depth, false)
		} else {
			s.WriteString(tstype)
		}

		if f.Comment != nil && g.PreserveTypeComments() {
			// Line comment is present, that means a comment after the field.
			s.WriteString(" // ")
			s.WriteString(f.Comment.Text())
		} else {
			s.WriteByte('\n')
		}

	}
}

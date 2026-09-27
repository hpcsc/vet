package material

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
)

// declarations returns the top-level declarations of Go source, one per line and
// without their bodies. A rule about naming compares names; it does not need the
// code around them, and the bodies are most of the file.
//
// A file that is not Go source, or that does not parse, yields nothing. Being a
// best effort is deliberate: a repository can hold generated files, fixtures and
// templates alongside its Go, and one of them failing to parse should not fail a
// run over material that was only ever advisory.
func declarations(path, src string) string {
	file, err := parser.ParseFile(token.NewFileSet(), path, src, parser.SkipObjectResolution)
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			b.WriteString("func ")
			if d.Recv != nil && len(d.Recv.List) > 0 {
				b.WriteString(node(d.Recv.List[0].Type))
				b.WriteString(" ")
			}
			b.WriteString(d.Name.Name)
			b.WriteString(strings.TrimPrefix(node(d.Type), "func"))
			b.WriteString("\n")
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					b.WriteString("type ")
					b.WriteString(s.Name.Name)
					if s.TypeParams != nil {
						b.WriteString(strings.TrimPrefix(node(s.TypeParams), "["))
					}
					b.WriteString(" ")
					b.WriteString(node(s.Type))
					b.WriteString("\n")
				case *ast.ValueSpec:
					kind := strings.ToLower(d.Tok.String())
					for _, name := range s.Names {
						b.WriteString(kind)
						b.WriteString(" ")
						b.WriteString(name.Name)
						b.WriteString("\n")
					}
				}
			}
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

var nodePrinter = printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 4}

// node renders a syntax tree fragment as source, falling back to nothing rather
// than failing a run over one declaration it could not print.
func node(n any) string {
	var buf bytes.Buffer
	if err := nodePrinter.Fprint(&buf, token.NewFileSet(), n); err != nil {
		return ""
	}
	return buf.String()
}

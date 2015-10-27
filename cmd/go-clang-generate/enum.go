package main

import (
	"bytes"
	"io/ioutil"
	"strings"
	"text/template"

	"github.com/sbinet/go-clang"
)

type Enum struct {
	Name           string
	CName          string
	CNameIsTypeDef bool
	Receiver       Receiver
	Comment        string

	Items []Enumerator

	Methods []string
}

type Enumerator struct {
	Name    string
	CName   string
	Comment string
}

func handleEnumCursor(cursor clang.Cursor, cname string, cnameIsTypeDef bool) *Enum {
	e := Enum{
		CName:          cname,
		CNameIsTypeDef: cnameIsTypeDef,
		Comment:        cleanDoxygenComment(cursor.RawCommentText()),

		Items: []Enumerator{},
	}

	e.Name = trimClangPrefix(e.CName)
	e.Receiver.Name = receiverName(e.Name)
	if cnameIsTypeDef {
		e.Receiver.CName = e.CName
	} else {
		e.Receiver.CName = "enum_" + e.CName // TODO remove this hack somehow "enum_" is Go's way of using enums and struts without typedef it is not the real CName. We need this for example for the Go->C type conversion in clang_index_isEntityObjCContainerKind.
	}

	cursor.Visit(func(cursor, parent clang.Cursor) clang.ChildVisitResult {
		switch cursor.Kind() {
		case clang.CK_EnumConstantDecl:
			ei := Enumerator{
				CName:   cursor.Spelling(),
				Comment: cleanDoxygenComment(cursor.RawCommentText()), // TODO We are always using the same comment if there is none, see "TypeKind"
			}

			ei.Name = trimClangPrefix(ei.CName)
			// TODO remove underlines to make the names more Go idiomatic e.g. "C.CXComment_InlineCommand" should be "CommentInlineCommand"

			e.Items = append(e.Items, ei)
		default:
			panic(cursor.Kind())
		}

		return clang.CVR_Continue
	})

	if strings.HasSuffix(e.Name, "Error") {
		e.Receiver.PrimitiveType = "int32"
	} else {
		e.Receiver.PrimitiveType = "uint32"
	}

	return &e
}

var templateGenerateEnum = template.Must(template.New("go-clang-generate-enum").Parse(`package phoenix

// #include "go-clang.h"
import "C"

{{$.Comment}}
type {{$.Name}} {{$.Receiver.PrimitiveType}}

const (
{{range $i, $e := .Items}}	{{if $e.Comment}}{{$e.Comment}}
	{{end}}{{$e.Name}}{{if eq $i 0}} {{$.Name}}{{end}} = C.{{$e.CName}}
{{end}}
)

{{range $i, $m := .Methods}}
{{$m}}
{{end}}
`))

func generateEnum(e *Enum) error {
	var b bytes.Buffer
	if err := templateGenerateEnum.Execute(&b, e); err != nil {
		return err
	}

	// TODO remove "_" from names for files here?

	return ioutil.WriteFile(strings.ToLower(e.Name)+"_gen.go", b.Bytes(), 0600)
}

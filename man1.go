package main

import (
	"bytes"
	"go/ast"
	"path"
	"strings"
)

func doCommand(m *M) {
	m.sec = "1"

	//extract information
	if m.name == "" {
		m.name = grep_name(m.pkg)
	}
	flags, descrs := grep_flags(m)
	m.find_refs(descrs) //need name and sec first so we can ignore self references

	m.do_header("User Commands")
	m.do_name()

	//do synopsis
	m.section("SYNOPSIS")

	//name and discovered flags
	m.WriteString(".B ")
	m.WriteString(m.name)
	for _, flag := range flags.flags {
		m.WriteString("\n.RB [ \\-")
		m.Write(flag[1])       //name
		if len(flag[0]) != 0 { //varname, or "" if bool
			m.WriteString("\n.IR ")
			m.Write(flag[0])
		}
		m.WriteString(" ]")
	}

	//format extra usage flags
	for _, w := range inverseMatch(wrx, []byte(flags.usage)) {
		if bytes.HasPrefix(w, []byte("[")) && bytes.HasSuffix(w, []byte("]")) {
			m.WriteString("\n.RB [ ")
			m.Write(escape(w[1 : len(w)-1]))
			m.WriteString(" ]")
		} else {
			m.WriteString("\n.B ")
			m.Write(escape(w))
		}
	}

	m.do_description()

	//do options
	if x := len(flags.flags); x > 0 {
		m.section("OPTIONS")
		for i, flag := range flags.flags {
			if i != 0 {
				m.nl()
			}
			m.WriteString(".TP\n.BR \"\\-")
			m.Write(flag[1]) //name
			m.WriteString(" \"")
			if len(flag[0]) != 0 { //variable name
				m.WriteByte(' ')
				m.Write(flag[0])
				if len(flag[2]) != 0 { //default value
					m.WriteString(" \" = ")
					m.Write(flag[2])
					m.WriteByte('"')
				}
			}
			m.nl()
			m.text(flag[3]) //help string
		}
	}

	//put these in order, leave the rest as they come
	m.user_sections("DIAGNOSTICS", "ENVIRONMENT", "FILES")
	m.remaining_user_sections()
	m.do_bugs()
	m.do_see_also()
	m.do_endmatter()
}

func grep_name(p *ast.Package) string {
	for name, file := range p.Files {
		for _, decl := range file.Decls {
			if f, ok := decl.(*ast.FuncDecl); ok {
				if f.Name.Name == "main" {
					str := path.Base(name)
					return str[:len(str)-3] //cut off .go
				}
			}
		}
	}
	fatal("package main has no main function")
	panic("issue 65")
}

//BUG(jmf): Need to special case flag.Var to make it format properly.

//allow addition of more option info with a line in the comments above main
//matching the below regex
var usrx = RX("^[ \t]*Usage:([ ]+%name)?[ ]+(%flags?[ ]+)?")

type flags struct {
	usage string
	flags [][4][]byte //varname, name, default, help
}

//BUG(jmf): No way to group short/long name option pairs.

func grep_flags(m *M) (a flags, d []string) {
	out := flags{"", make([][4][]byte, 0, 8)}
	descrs := []string{}

	//see if there's an additional usage string
	for _, fnc := range m.docs.Funcs {
		if fnc.Name == "main" {
			for _, line := range strings.Split(fnc.Doc, "\n") {
				if s := usrx.FindStringIndex(line); s != nil {
					out.usage = line[s[1]:]
					break //only going to be one
				}
			}
		}
	}

	//find all var N = flag.Var(n, def, dscr) |-> (N, n, def, dscr)
	for _, Var := range m.docs.Vars {
		for _, gVal := range Var.Decl.Specs {
			Val := gVal.(*ast.ValueSpec)
			for i, val := range Val.Values {
				if c, ok := val.(*ast.CallExpr); ok {
					if s, ok := c.Fun.(*ast.SelectorExpr); ok {
						if id, ok := s.X.(*ast.Ident); ok && id.Name == "flag" {
							if len(c.Args) != 3 {
								fatal("Could not parse flags.")
							}

							//package up flag info
							descr := lit(c.Args[2])
							descrs = append(descrs, string(descr))
							group := [...][]byte{
								[]byte(Val.Names[i].Name),
								lit(c.Args[0]),
								lit(c.Args[1]),
								descr,
							}
							if s.Sel.Name == "Bool" {
								group[0] = nil
								group[2] = nil
							}

							//muss with slice
							if c := cap(out.flags); len(out.flags) == c {
								new := make([][4][]byte, c, c+c/2)
								copy(new, out.flags)
								out.flags = new
							}
							end := len(out.flags)
							out.flags = out.flags[:end+1]

							out.flags[end] = group
						}
					}
				}
			}
		}
	}

	return out, descrs
}

package main

import (
	"bytes"
	"strings"
	"container/vector"
	"unicode"
)

func escape(in []byte) []byte {
	var buf bytes.Buffer
	last := 0
	for _, rune := range bytes.Runes(in) {
		switch rune {
		case int('\n'):
			buf.WriteByte(' ')
			last = int(' ')
			continue
		case int('\\'):
			buf.WriteString("\\e")
			continue
		case int('-'):
			buf.WriteByte('\\')
		case int('.'):
			if last == 0 || unicode.IsSpace(last) {
				buf.WriteByte('\\')
			}
		case int('\''):
			if last == 0 || unicode.IsSpace(last) {
				buf.WriteString("\\(fm")
				last = int('\'')
				continue
			}
		}
		buf.WriteRune(rune)
		last = rune
	}
	return buf.Bytes()
}

type BR struct {
	c, v *vector.StringVector
	bold bool
}

func NewBR() *BR {
	return &BR{&vector.StringVector{}, &vector.StringVector{}, true}
}

func (s *BR) witch() {
	if s.c.Len() == 0 {
		return
	}
	s.v.Push("\"" + strings.Join([]string(*s.c), "") + "\"")
	s.c = &vector.StringVector{}
	s.bold = !s.bold
}

func (s *BR) B(str string) {
	if !s.bold {
		s.witch()
	}
	s.c.Push(str)
}

func (s *BR) R(str string) {
	if s.bold {
		s.witch()
	}
	s.c.Push(str)
}

type F struct {
	*bytes.Buffer
	BR *BR
}

func Formatter() *F {
	return &F{&bytes.Buffer{}, NewBR()}
}

func (m *F) br() {
	m.BR.witch()
	if m.BR.v.Len() == 0 {
		return
	}
	m.WriteString(".BR " + strings.Join([]string(*m.BR.v), " ") + "\n")
	m.BR.v = &vector.StringVector{}
	m.BR.bold = true
}

func (m *F) nl() {
	if m.Bytes()[m.Len()-1] != '\n' {
		m.WriteByte('\n')
	}
}

func (m *F) PP() {
	m.nl()
	m.WriteString(".PP\n")
}

func (m *F) section(name string) {
	m.nl()
	m.WriteString(".SH \"")
	m.WriteString(name)
	m.WriteString("\"\n")
}

func (m *F) text(p []byte) {
	for _, s := range sentences(p) {
		m.nl()
		m.Write(words(s))
	}
}

func (m *F) paras(ps [][]byte) {
	for i, p := range ps {
		if i != 0 {
			m.PP()
		}
		if p[0] == ' ' || p[0] == '\t' {
			l, i, _ := indents(p)
			m.code(l, i)
		} else {
			m.text(p)
		}
	}
}

//BUG(jmf): code formatter could balk on double spaced code or mishandle
//complex indentation

func (m *F) code(lines [][]byte, indents []int) {
	min := minin(indents)
	for i, in := range indents {
		indents[i] = in - min
	}
	last, cnt := 0, 0
	m.WriteString(".RS")
	for i, line := range lines {
		m.nl()
		line = bytes.TrimSpace(line)
		in := indents[i]
		if len(line) == 0 {
			m.WriteString(".sp\n")
			continue
		}
		if last < in {
			cnt++
			m.WriteString(".RS\n")
		}
		if last > in {
			cnt--
			if cnt < 0 {
				fatal("Impossible indentation")
			}
			m.WriteString(".RE\n")
		}
		m.Write(escape(line))
		m.nl()
		if i != len(lines)-1 {
			m.WriteString(".sp 0\n")
		}
		last = in
	}
	//make sure indentation balances, in case someone used python code
	for cnt++; cnt > 0; cnt-- {
		m.WriteString("\n.RE")
	}
}

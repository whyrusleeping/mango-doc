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
				buf.WriteString("\\&")
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
	m.WriteString(strings.TrimSpace(name))
	m.WriteString("\"\n")
}

var wrx = RX("[ \n\t]")

func (m *F) words(sentence []byte) {
	for _, word := range inverseMatch(wrx, bytes.TrimSpace(sentence)) {
		word = bytes.TrimSpace(word)
		if len(word) == 0 {
			continue
		}
		switch {
		case bytes.HasPrefix(word, []byte("-")):
			m.nl()
			m.WriteString(".B \\")
			m.Write(word)
			m.nl()
		case refrx.Match(word): //defined above find_refs()
			m.nl()
			m.WriteString(".BR ")
			piv := bytes.IndexByte(word, '(')
			m.Write(escape(word[:piv]))
			m.WriteByte(' ')
			m.Write(word[piv:])
			m.nl()
		default:
			m.Write(escape(word))
			m.WriteByte(' ')
		}
	}
}

func (m *F) text(p []byte) {
	for _, s := range sentences(p) {
		m.nl()
		m.words(s)
	}
}

func (m *F) paras(ps *vector.Vector) {
	for i, P := range *ps {
		if i != 0 {
			m.PP()
		}
		switch p := P.(type) {
		case []byte: // raw section
			m.nl()
			m.Write(p)
			m.nl()
		case [][]byte:
			for _, s := range p {
				m.nl()
				m.words(s)
			}
		case []*loc:
			last, depth := 0, 0
			for j, loc := range p {
				m.nl()
				line, in := loc.line, loc.indent
				if in == -1 {
					m.WriteString(".sp\n")
					continue
				} else if last < in {
					depth++
					m.WriteString(".RS\n")
				} else if last > in {
					depth--
					if depth < 0 {
						fatal("Impossible indentation.")
					}
					m.WriteString(".RE\n")
				}
				m.Write(escape(line))
				m.nl()
				if j != len(p)-1 {
					m.WriteString(".sp 0\n")
				}
				last = in
			}
			//make sure we unindent as much as we've indented
			for ; depth > 0; depth-- {
				m.nl()
				m.WriteString(".RE")
			}
		}
	}
}

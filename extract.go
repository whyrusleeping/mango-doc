package main

import (
	"unicode"
	"bytes"
)

type coll [][]byte

func newColl() *coll {
	r := coll(make([][]byte, 0, 16))
	return &r
}

func (c *coll) push(s []byte) {
	if len(s) == 0 {
		return
	}
	ln := len(*c)
	if cp := cap(*c); cp == ln {
		new := make([][]byte, ln, cp+cp/2)
		copy(new, *c)
		*c = new
	}
	*c = (*c)[:ln+1]
	new := make([]byte, len(s))
	copy(new, s)
	(*c)[ln] = new
}

func (c *coll) data() [][]byte {
	return [][]byte(*c)
}

func (c *coll) join() []byte {
	return bytes.Join(c.data(), nil)
}

var lrx = RX("\n")

func lines(p []byte) [][]byte {
	out := inverseMatch(lrx, p)
	for i := 0; i < len(out); i++ {
		out[i] = bytes.AddByte(out[i], '\n')
	}
	return out
}

func indents(p []byte) ([][]byte, []int) {
	lines := lines(p)
	indents := make([]int, len(lines))
	tab := false
	//determine indent mode. Mixed indents would screw this up but no one likes
	//people who mix indents, anyway
	for _, line := range lines {
		if line[0] == '\t' {
			tab = true
			break
		}
		//only count as indent if there are at least 3 spaces (go/doc cuts 1 sp)
		if bytes.HasPrefix(line, []byte("   ")) {
			break
		}
	}
	//count indents
	for lno, line := range lines {
		i := 0
		for ; i < len(line); i++ {
			if tab {
				if line[i] != '\t' {
					break
				}
			} else {
				if line[i] != ' ' {
					break
				}
			}
		}
		indents[lno] = i
	}
	if !tab {
		//assume 4 sp = 1 tab
		for i, in := range indents {
			//we need to add 1 since go/doc strips first space
			if in != 0 {
				indents[i] = (in + 1) % 4
			}
		}
	}
	return lines, indents
}

func minin(indents []int) int {
	min := indents[0]
	for _, in := range indents[1:] {
		if in < min {
			min = in
		}
	}
	return min
}

var prx = RX("\n" + SP + "*\n")

func paragraphs(in string) [][]byte {
	ps := inverseMatch(prx, []byte(in))
	out := newColl()
	for _, p := range ps {
		lines, indents := indents(p)
		min := minin(indents)
		for i, ln := 0, len(lines); i < ln; {
			acc := newColl()
			if indents[i] == min {
				for ; i < ln && indents[i] == min; i++ {
					acc.push(bytes.TrimLeftFunc(lines[i], unicode.IsSpace))
				}
			} else {
				for ; i < ln && indents[i] > min; i++ {
					acc.push(lines[i])
				}
			}
			out.push(acc.join())
		}
	}
	return out.data()
}

var srx = RX(NS + "[.!?][ \n\t]+")

func sentences(in []byte) [][]byte {
	out := inverseMatch(srx, in)
	last := len(out) - 1
	for i, s := range out[:last] {
		//inverse slices 'in' so this gets the last char and ending punctuation
		out[i] = s[:len(s)+2]
	}
	if cap(out[last])-len(out[last]) >= 2 {
		out[last] = out[last][:len(out[last])+2]
	}
	return out
}

func acronymp(a []byte) bool {
	if len(a) < 3 {
		return false
	}
	runes := bytes.Runes(a)
	end := len(runes) - 1
	for _, rune := range runes[:end] {
		if !(rune == int('.') || rune == int('-') || unicode.IsUpper(rune)) {
			return false
		}
	}
	last := runes[end]
	if !unicode.IsUpper(last) {
		switch last {
		default:
			return false
		case int('.'), int('!'), int('?'), int(','), int(';'), int(':'):
			return true
		}
	}
	return true
}

const pe = "(" + NS + "|\\\\ |\\\\\t)+"

var pathrx = RX("/?" + pe + "/(" + pe + "/)*(" + pe + ")?")

func pathp(word []byte) bool {
	if !pathrx.Match(word) {
		return false
	}
	last := word[0] == '/'
	for _, c := range word[1:] {
		cur := c == '/'
		if cur && last {
			return false
		}
		last = cur
	}
	return true
}

var wrx = RX("[ \n\t]")

func words(sentence []byte) []byte {
	var buf bytes.Buffer
	ms := inverseMatch(wrx, sentence)
	nl := func() {
		if x := buf.Len(); x != 0 && x != len(ms) && buf.Bytes()[x-1] != '\n' {
			buf.WriteByte('\n')
		}
	}
	for _, word := range ms {
		word = bytes.TrimSpace(word)
		if len(word) == 0 {
			continue
		}
		switch {
		case acronymp(word):
			nl()
			buf.WriteString(".SM ")
			buf.Write(word)
			nl()
		case inlinerefrx.Match(word): //defined above find_refs()
			nl()
			buf.WriteString(".BR ")
			piv := bytes.IndexByte(word, '(')
			buf.Write(escape(word[:piv]))
			buf.WriteByte(' ')
			buf.Write(word[piv:])
			nl()
		case pathp(word):
			nl()
			buf.WriteString(".I ")
			buf.Write(escape(word))
			nl()
		default:
			buf.Write(escape(word))
			buf.WriteByte(' ')
		}
	}
	return bytes.TrimSpace(buf.Bytes())
}

type section struct {
	name  string
	paras [][]byte
}

func isSecHdr(s []byte) bool {
	//paragraph of a single line
	if bytes.IndexByte(s, '\n') != -1 {
		return false
	}
	//all of whose words are uppercase
	for _, rune := range bytes.Runes(s) {
		if !(unicode.IsUpper(rune) || unicode.IsSpace(rune)) {
			return false
		}
	}
	return true
}

func sections(paras [][]byte) []*section {
	numsec, end := 1, 0
	//see if there are any other sections
	for i, v := range paras {
		if isSecHdr(v) {
			numsec++
			if end == 0 {
				end = i
			}
		}
	}
	//no supplementarty sections, return just the default section
	if end == 0 {
		return []*section{&section{"", paras}}
	}
	//make accumulator and store default section
	out := make([]*section, numsec)
	out[0] = &section{"", paras[:end]}
	start := end
	//separate out supplementary sections
	for i := 1; i < numsec; i++ {
		name := string(paras[start])
		end = start + 1
		for ; end < len(paras) && !isSecHdr(paras[end]); end++ {
		}
		out[i] = &section{name, paras[start+1 : end]}
		start = end
	}
	return out
}

//return 1 for regular PP, 0 for IP, -1 for code RS/RE fun
func pkind(p []byte) (int, [][]byte, []int) {
	lines, indents := indents(p)
	min := minin(indents)
	if len(lines) == 1 {
		return 1, nil, nil
	}
	//normalize indents
	for i, in := range indents {
		indents[i] = in - min
	}
	//scan the indents to figure out what kind of scheme they imply
	unindented := 0
	last, diff := indents[1], false
	for _, in := range indents[1:] {
		//once diff has been set, we want it to stay that way
		diff = !diff && (last != in)
		last = in
		unindented += in
	}
	//only first line of many was indented, treat as normal paragraph
	if unindented == 0 && len(lines) != 1 {
		return 1, nil, nil
	}
	//otherwise, it's 'code' because it has differing indents
	return -1, lines, indents
}

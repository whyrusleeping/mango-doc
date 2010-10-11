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

func empty(line []byte) bool {
	return len(bytes.TrimSpace(line)) == 0
}

func indents(p []byte) ([][]byte, []int, bool) {
	lines := lines(p)
	indents := make([]int, len(lines))
	tab := false
	//determine indent mode. Mixed indents would screw this up but no one likes
	//people who mix indents, anyway
	for _, line := range lines {
		if empty(line) {
			continue
		}
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
	return lines, indents, tab
}

func minin(indents []int) int {
	min := indents[0]
	for _, in := range indents[1:] {
		if in != -1 && in < min {
			min = in
		}
	}
	return min
}

func strip(line []byte, ins int, tab bool) []byte {
	if ins <= 0 {
		return line
	}
	if !tab {
		ins = 4*ins - 1
	}
	return line[ins-1:]
}

func paragraphs(in string) [][]byte {
	lines, indents, tab := indents([]byte(in))
	for i, line := range lines {
		if empty(line) {
			indents[i] = -1
		}
	}
	min := minin(indents)
	for i, line := range lines {
		in := indents[i]
		if in == -1 {
			continue
		}
		lines[i] = strip(line, min, tab)
		indents[i] -= min
	}
	out := newColl()
	ln := len(lines)
	for i := 0; i < ln; i++ {
		acc := newColl()
		for ; i < ln && indents[i] == -1; i++ {}
		if indents[i] == 0 {
			for ; i < ln && indents[i] != -1 && indents[i] == 0; i++ {
				acc.push(lines[i])
			}
		} else {
			for ; i < ln && indents[i] != -1 && indents[i] > 0; i++ {
				acc.push(lines[i])
			}
		}
		out.push(acc.join())
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
		case inlinerefrx.Match(word): //defined above find_refs()
			nl()
			buf.WriteString(".BR ")
			piv := bytes.IndexByte(word, '(')
			buf.Write(escape(word[:piv]))
			buf.WriteByte(' ')
			buf.Write(word[piv:])
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
	lines, indents, _ := indents(p)
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

package main

import (
	"unicode"
	"bytes"
	"container/vector"
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

type loc struct {
	indent int
	line   []byte
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
	return len(line) == 0 || len(bytes.TrimSpace(line)) == 0
}

func locify(ls [][]byte) []*loc {
	if len(ls) == 0 {
		return nil
	}
	ret := make([]*loc, len(ls))
	min := 1000
	for i, line := range ls {
		ind := 0
		if empty(line) {
			ret[i] = &loc{-1, nil}
			continue
		}
		tab := line[0] == '\t' || !bytes.HasPrefix(line, []byte("   "))
		comp := byte(' ')
		if tab {
			comp = byte('\t')
		}
		for ; ind < len(line) && line[ind] == comp; ind++ {
		}
		if !tab {
			//go/doc trims first space and we assume 4sp = 1tb
			ind = (ind + 1) / 4
		}
		if ind < min {
			min = ind
		}
		ret[i] = &loc{ind, bytes.TrimLeftFunc(line, unicode.IsSpace)}
	}
	//normalize indents
	for i := range ret {
		if ret[i].indent != -1 {
			ret[i].indent -= min
		}
	}
	return ret
}

func partition(locs []*loc) *vector.Vector {
	ln := len(locs)
	if ln == 0 {
		return nil
	}
	ret := &vector.Vector{}
	for i := 0; i < ln; {
		//skip blank lines
		for ; i < ln && locs[i].indent == -1; i++ {
		}
		//select mode
		if locs[i].indent == 0 {
			//paragraph mode
			acc := newColl()
			for ; i < ln && locs[i].indent == 0; i++ {
				acc.push(locs[i].line)
			}
			ret.Push(sentences(acc.join()))
		} else {
			//"code" mode
			start := i
			for ; i < ln && locs[i].indent != 0; i++ {
				locs[i].line = bytes.TrimSpace(locs[i].line)
			}
			//TODO should cleave off any extraneous blank lines at the end
			end := i
			if end-start > 0 {
				ret.Push(locs[start:end])
			}
		}
	}
	return ret
}

func unstring(in string) *vector.Vector {
	return partition(locify(lines([]byte(in))))
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

type section struct {
	name  string
	paras *vector.Vector // [][]byte or []*loc
}

func isSecHdr(s interface{}) bool {
	p, ok := s.([][]byte)
	if !ok || len(p) != 1 {
		return false
	}
	for _, rune := range bytes.Runes(p[0]) {
		if !(unicode.IsUpper(rune) || unicode.IsSpace(rune)) {
			return false
		}
	}
	return true
}

func sections(src *vector.Vector) []*section {
	num, end := 1, 0
	//check for other sections
	for i, v := range *src {
		if isSecHdr(v) {
			num++
			//mark first sec header
			if end == 0 {
				end = i
			}
		}
	}
	if end == 0 {
		return []*section{&section{"", src}}
	}
	secs := make([]*section, num)
	secs[0] = &section{"", src.Slice(0, end)}
	start := end
	for i := 1; i < num; i++ {
		p, ok := src.At(start).([][]byte)
		if !ok || len(p) > 1 {
			start++
			continue
		}
		name := string(p[0])
		start++
		for end = start; end < src.Len() && !isSecHdr(src.At(end)); end++ {
		}
		secs[i] = &section{name, src.Slice(start, end)}
		start = end
	}
	return secs
}

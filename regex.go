package main

import "regexp"

var RX = regexp.MustCompile

const SP = "[ \t]"
const NS = "[^ \t]"

var refrx = RX("..\\(.\\)$")                       //used in extract.go:words

func inverseMatch(r *regexp.Regexp, s []byte) [][]byte {
	in := r.FindAllIndex(s, -1)
	ln, sln := len(in), len(s)
	if ln == 0 {
		return [][]byte{s}
	}
	lopen, ropen := in[0][0] == 0, in[ln-1][1] == sln
	if ln == 1 && lopen && ropen {
		return [][]byte{}
	}
	new := make([][]int, ln+1)
	w := 0
	if !lopen {
		new[0] = []int{0, in[0][0]}
		w = 1
	}
	for i := 0; i < ln-1; i++ {
		new[w] = []int{in[i][1], in[i+1][0]}
		w++
	}
	if !ropen {
		new[w] = []int{in[ln-1][1], sln}
		w++
	}
	new = new[:w]
	out := make([][]byte, len(new))
	for i, v := range new {
		out[i] = s[v[0]:v[1]]
	}
	return out
}

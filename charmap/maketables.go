// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/moriyoshi/ebcdic-kana/internal/gen"
	"github.com/moriyoshi/simplefiletx"
)

const ascii = "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f" +
	"\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f" +
	` !"#$%&'()*+,-./0123456789:;<=>?` +
	`@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\]^_` +
	"`abcdefghijklmnopqrstuvwxyz{|}~\u007f"

var encodings = []struct {
	name        string
	comment     string
	varName     string
	replacement byte
	mapping     string
}{
	{
		"EBCDIC-K",
		"",
		"EBCDIC_K",
		0x3f,
		"file:ebcdic-k.ucm",
	},
}

func getUCM(url string) string {
	tx := &http.Transport{}
	tx.RegisterProtocol("file", simplefiletx.NewSimpleFileTransport("."))
	httpClient := &http.Client{Transport: tx}
	res, err := httpClient.Get(url)
	if err != nil {
		log.Fatalf("%q: Get: %v", url, err)
	}
	defer res.Body.Close()

	mapping := make([]rune, 256)
	for i := range mapping {
		mapping[i] = '\ufffd'
	}

	charsFound := 0
	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		s := strings.TrimSpace(scanner.Text())
		if s == "" || s[0] == '#' {
			continue
		}
		var c byte
		var r rune
		if _, err := fmt.Sscanf(s, `<U%x> \x%x |0`, &r, &c); err != nil {
			continue
		}
		mapping[c] = r
		charsFound++
	}

	if charsFound < 128 {
		log.Fatalf("%q: only %d characters found (wrong page format?)", url, charsFound)
	}

	return string(mapping)
}

func main() {
	all := []string{}

	w := gen.NewCodeWriter()
	defer w.WriteGoFile("tables.go", "charmap")

	printf := func(s string, a ...interface{}) { fmt.Fprintf(w, s, a...) }

	printf("import (\n")
	printf("\t\"golang.org/x/text/encoding\"\n")
	printf(")\n\n")
	for _, e := range encodings {
		varNames := strings.Split(e.varName, ",")
		all = append(all, varNames...)
		varName := varNames[0]
		e.mapping = getUCM(e.mapping)

		asciiSuperset, low := strings.HasPrefix(e.mapping, ascii), 0x00
		if asciiSuperset {
			low = 0x80
		}
		lvn := 0
		for ; lvn < len(varName); lvn++ {
			if !(varName[lvn] >= 'A' && varName[lvn] <= 'Z') {
				break
			}
		}
		lowerVarName := strings.ToLower(varName[:lvn]) + varName[lvn:]
		printf("// %s is the %s encoding.\n", varName, e.name)
		if e.comment != "" {
			printf("//\n// %s\n", e.comment)
		}
		printf("var %s *Charmap = &%s\n\nvar %s = Charmap{\nname: %q,\n",
			varName, lowerVarName, lowerVarName, e.name)
		printf("asciiSuperset: %t,\n", asciiSuperset)
		printf("low: 0x%02x,\n", low)
		printf("replacement: 0x%02x,\n", e.replacement)

		printf("decode: [256]utf8Enc{\n")
		i, backMapping := 0, map[rune]byte{}
		for _, c := range e.mapping {
			if _, ok := backMapping[c]; !ok && c != utf8.RuneError {
				backMapping[c] = byte(i)
			}
			var buf [8]byte
			n := utf8.EncodeRune(buf[:], c)
			if n > 3 {
				panic(fmt.Sprintf("rune %q (%U) is too long", c, c))
			}
			printf("{%d,[3]byte{0x%02x,0x%02x,0x%02x}},", n, buf[0], buf[1], buf[2])
			if i%2 == 1 {
				printf("\n")
			}
			i++
		}
		printf("},\n")

		printf("encode: [256]uint32{\n")
		encode := make([]uint32, 0, 256)
		for c, i := range backMapping {
			encode = append(encode, uint32(i)<<24|uint32(c))
		}
		sort.Sort(byRune(encode))
		for len(encode) < cap(encode) {
			encode = append(encode, encode[len(encode)-1])
		}
		for i, enc := range encode {
			printf("0x%08x,", enc)
			if i%8 == 7 {
				printf("\n")
			}
		}
		printf("},\n}\n")

		// Add an estimate of the size of a single Charmap{} struct value, which
		// includes two 256 elem arrays of 4 bytes and some extra fields, which
		// align to 3 uint64s on 64-bit architectures.
		w.Size += 2*4*256 + 3*8
	}
	// TODO: add proper line breaking.
	printf("var listAll = []encoding.Encoding{\n%s,\n}\n\n", strings.Join(all, ",\n"))
}

type byRune []uint32

func (b byRune) Len() int           { return len(b) }
func (b byRune) Less(i, j int) bool { return b[i]&0xffffff < b[j]&0xffffff }
func (b byRune) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

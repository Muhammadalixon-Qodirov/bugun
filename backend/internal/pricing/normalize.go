package pricing

import (
	"strings"
	"unicode"
)

var cyrToLat = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'ғ': "g'", 'д': "d", 'е': "e", 'ё': "yo",
	'ж': "j", 'з': "z", 'и': "i", 'й': "y", 'к': "k", 'қ': "q", 'л': "l", 'м': "m",
	'н': "n", 'о': "o", 'ў': "o'", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u",
	'ф': "f", 'х': "x", 'ҳ': "h", 'ц': "s", 'ч': "ch", 'ш': "sh", 'щ': "sh", 'ъ': "'",
	'ы': "i", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
}

// Normalize lowercases text, transliterates Cyrillic to Latin and removes
// apostrophe variants, so "Ўрик", "o‘rik" and "o'rik" all become "orik".
func Normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if lat, ok := cyrToLat[r]; ok {
			b.WriteString(strings.ReplaceAll(lat, "'", ""))
			continue
		}
		switch r {
		case '\'', '‘', '’', '`', 'ʻ', 'ʼ', '´':
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

type tokenKind int

const (
	tokWord tokenKind = iota
	tokNumber
	tokSlash
	tokBreak // line break or semicolon: a hard item boundary
	tokComma // separates numbers ("besh yuz, ikki tonna") but not items
)

type token struct {
	kind   tokenKind
	text   string
	merged bool // number built from space-separated thousand groups
}

// tokenize splits normalized text into words, numbers ("4500", "4,5", "4.5"),
// slashes, commas and hard breaks.
func tokenize(s string) []token {
	var toks []token
	rs := []rune(s)
	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case unicode.IsDigit(r):
			j := i
			for j < len(rs) && (unicode.IsDigit(rs[j]) ||
				((rs[j] == ',' || rs[j] == '.') && j+1 < len(rs) && unicode.IsDigit(rs[j+1]) && j > i)) {
				j++
			}
			toks = append(toks, token{kind: tokNumber, text: string(rs[i:j])})
			i = j
		case unicode.IsLetter(r):
			j := i
			for j < len(rs) && (unicode.IsLetter(rs[j]) || rs[j] == '-') {
				j++
			}
			toks = append(toks, token{kind: tokWord, text: strings.Trim(string(rs[i:j]), "-")})
			i = j
		case r == '/':
			toks = append(toks, token{kind: tokSlash, text: "/"})
			i++
		case r == '\n' || r == ';':
			toks = append(toks, token{kind: tokBreak})
			i++
		case r == ',':
			toks = append(toks, token{kind: tokComma})
			i++
		default:
			i++
		}
	}
	return mergeThousands(toks)
}

// mergeThousands joins "4 500" into "4500" and "1 200 000" into "1200000".
func mergeThousands(toks []token) []token {
	var out []token
	for _, t := range toks {
		n := len(out)
		if t.kind == tokNumber && n > 0 && out[n-1].kind == tokNumber &&
			len(t.text) == 3 && isDigits(t.text) && isDigits(out[n-1].text) &&
			(len(out[n-1].text) <= 3 || out[n-1].merged) {
			out[n-1].text += t.text
			out[n-1].merged = true
			continue
		}
		out = append(out, t)
	}
	return out
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

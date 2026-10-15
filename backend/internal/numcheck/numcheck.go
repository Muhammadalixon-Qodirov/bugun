// Package numcheck is the "no new numbers" check on text a model wrote: every
// number in it must be one the deterministic functions returned (spec §9).
package numcheck

import (
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// A number written as "4 500", "4,5 ming" or "12%" counts when it matches an
// allowed number (a share of 0.12 also stands for 12%).

// numberRe finds numbers with optional thousand groups ("4 500", "1 200 000")
// and a decimal part, and a scale word after them.
var numberRe = regexp.MustCompile(`(?i)(\d+(?:[   ]\d{3})*(?:[.,]\d+)?)(\s*(?:ming|минг|тыс\.?|тысяч[аи]?|thousand|k\b|mln|млн|million|mlrd|млрд|billion))?`)

// Written is a number in a text with its possible readings.
type Written struct {
	Values []float64 // readings of the number; any may match
	Text   string
}

// NumbersIn lists the numbers in a text with their possible readings.
func NumbersIn(text string) []Written {
	var out []Written
	for _, m := range numberRe.FindAllStringSubmatch(text, -1) {
		raw, scale := m[1], strings.ToLower(strings.TrimSpace(m[2]))
		compact := strings.NewReplacer(" ", "", " ", "", " ", "").Replace(raw)
		var readings []float64
		if i := strings.IndexAny(compact, ".,"); i >= 0 {
			intPart, frac := compact[:i], compact[i+1:]
			if v, err := strconv.ParseFloat(intPart+"."+frac, 64); err == nil {
				readings = append(readings, v)
			}
			if len(frac) == 3 { // "4,500" or "4.500" as a thousands group
				if v, err := strconv.ParseFloat(intPart+frac, 64); err == nil {
					readings = append(readings, v)
				}
			}
		} else if v, err := strconv.ParseFloat(compact, 64); err == nil {
			readings = append(readings, v)
		}
		mult := 1.0
		switch {
		case scale == "":
		case strings.HasPrefix(scale, "mln") || strings.HasPrefix(scale, "млн") || strings.HasPrefix(scale, "million"):
			mult = 1e6
		case strings.HasPrefix(scale, "mlrd") || strings.HasPrefix(scale, "млрд") || strings.HasPrefix(scale, "billion"):
			mult = 1e9
		default:
			mult = 1e3
		}
		if mult != 1 {
			n := len(readings)
			for _, v := range readings[:n] {
				readings = append(readings, v*mult)
			}
		}
		if len(readings) > 0 {
			out = append(out, Written{Values: readings, Text: m[0]})
		}
	}
	return out
}

// Allowed is the set of numbers a text may use.
type Allowed struct{ vals []float64 }

// New allows the given numbers.
func New(vals ...float64) *Allowed { return &Allowed{vals: vals} }

// AddText adds every number written in s.
func (a *Allowed) AddText(s string) {
	for _, w := range NumbersIn(s) {
		a.vals = append(a.vals, w.Values...)
	}
}

// AddJSON adds every number in a tool result (walked as JSON, so strings
// such as dates count too); a share between -1 and 1 also adds its percent.
func (a *Allowed) AddJSON(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	var x any
	if err := json.Unmarshal(b, &x); err != nil {
		return
	}
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			for _, v := range t {
				walk(v)
			}
		case []any:
			for _, v := range t {
				walk(v)
			}
		case float64:
			a.vals = append(a.vals, t, math.Abs(t))
			if math.Abs(t) <= 1 {
				a.vals = append(a.vals, math.Abs(t*100))
			}
		case string:
			a.AddText(t)
		}
	}
	walk(x)
}

// Has reports whether v matches an allowed number, allowing the rounding a
// sentence makes (12.3 → "12").
func (a *Allowed) Has(v float64) bool {
	for _, x := range a.vals {
		tol := math.Max(0.5, math.Abs(x)*0.005)
		if math.Abs(v-x) <= tol {
			return true
		}
	}
	return false
}

// Unchecked returns the numbers of text that are not allowed.
func (a *Allowed) Unchecked(text string) []string {
	var out []string
	for _, w := range NumbersIn(text) {
		ok := false
		for _, v := range w.Values {
			if a.Has(v) {
				ok = true
				break
			}
		}
		if !ok {
			out = append(out, strings.TrimSpace(w.Text))
		}
	}
	return out
}

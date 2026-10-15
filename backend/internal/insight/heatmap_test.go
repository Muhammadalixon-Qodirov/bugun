package insight

import "testing"

// A dear block of neighbouring viloyatlar is a hot spot; small differences
// and too few viloyatlar are never marked.
func TestMarkSpots(t *testing.T) {
	// A 5×5 grid, rook neighbours; the top-left 3×3 block is 40% dearer.
	const n = 5
	var nb [][]int
	grid := func() []*HeatCell {
		cells := make([]*HeatCell, n*n)
		for i := range cells {
			r, c := i/n, i%n
			d := -0.08 + 0.01*float64((r*3+c*7)%5)
			if r < 3 && c < 3 {
				d = 0.4 + 0.01*float64((r+c)%3)
			}
			cells[i] = &HeatCell{Median: 1000, Offers: 5, Diff: &d}
		}
		return cells
	}
	for i := 0; i < n*n; i++ {
		r, c := i/n, i%n
		var ns []int
		for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			if rr, cc := r+d[0], c+d[1]; rr >= 0 && rr < n && cc >= 0 && cc < n {
				ns = append(ns, rr*n+cc)
			}
		}
		nb = append(nb, ns)
	}
	cells := grid()
	markSpots(cells, nb)
	if cells[1*n+1].Spot != "hot" {
		t.Errorf("centre of the dear block: %q", cells[1*n+1].Spot)
	}
	for i := 0; i < n*n; i++ {
		if i/n >= 3 && i%n >= 3 && cells[i].Spot == "hot" {
			t.Errorf("cell %d in the cheap corner is hot", i)
		}
	}
	// Under 8 viloyatlar with a price nothing is marked.
	few := grid()
	for i := 7; i < len(few); i++ {
		few[i] = nil
	}
	markSpots(few, nb)
	for i, c := range few[:7] {
		if c.Spot != "" {
			t.Errorf("few viloyatlar, cell %d marked %q", i, c.Spot)
		}
	}
}

// Package geo holds the viloyat boundaries for the map (W-04). Source:
// hudud.stat.uz /api/v1/gis/geometries/ (State Statistics Agency, 2026-09-24),
// feature id = SOATO, coordinates rounded to 4 decimals (about 10 m).
package geo

import (
	_ "embed"
	"encoding/json"
	"math"
	"slices"
)

//go:embed viloyatlar.geojson
var viloyatlar []byte

// Feature is one viloyat boundary; Properties is filled by the caller.
type Feature struct {
	Type       string          `json:"type"`
	ID         string          `json:"id"`
	Properties map[string]any  `json:"properties"`
	Geometry   json.RawMessage `json:"geometry"`
}

// Provinces returns a fresh copy of the 14 viloyat features keyed by SOATO.
func Provinces() (map[int64]Feature, error) {
	var fc struct {
		Features []struct {
			ID         string `json:"id"`
			Properties struct {
				Soato int64  `json:"soato"`
				Name  string `json:"name"`
			} `json:"properties"`
			Geometry json.RawMessage `json:"geometry"`
		} `json:"features"`
	}
	if err := json.Unmarshal(viloyatlar, &fc); err != nil {
		return nil, err
	}
	out := make(map[int64]Feature, len(fc.Features))
	for _, f := range fc.Features {
		out[f.Properties.Soato] = Feature{
			Type: "Feature", ID: f.ID, Geometry: f.Geometry,
			Properties: map[string]any{"soato": f.Properties.Soato, "name": f.Properties.Name},
		}
	}
	return out, nil
}

// Point is a longitude and latitude in degrees.
type Point struct{ Lon, Lat float64 }

// Centroids returns an approximate centre of each viloyat keyed by SOATO: the
// mean of the outer ring vertices of its largest polygon. Good enough for
// road distances between viloyatlar, not for maps.
func Centroids() (map[int64]Point, error) {
	var fc struct {
		Features []struct {
			Properties struct {
				Soato int64 `json:"soato"`
			} `json:"properties"`
			Geometry struct {
				Type        string          `json:"type"`
				Coordinates json.RawMessage `json:"coordinates"`
			} `json:"geometry"`
		} `json:"features"`
	}
	if err := json.Unmarshal(viloyatlar, &fc); err != nil {
		return nil, err
	}
	out := make(map[int64]Point, len(fc.Features))
	for _, f := range fc.Features {
		var polys [][][][2]float64
		switch f.Geometry.Type {
		case "MultiPolygon":
			if err := json.Unmarshal(f.Geometry.Coordinates, &polys); err != nil {
				return nil, err
			}
		case "Polygon":
			var p [][][2]float64
			if err := json.Unmarshal(f.Geometry.Coordinates, &p); err != nil {
				return nil, err
			}
			polys = [][][][2]float64{p}
		default:
			continue
		}
		var ring [][2]float64
		for _, p := range polys {
			if len(p) > 0 && len(p[0]) > len(ring) {
				ring = p[0]
			}
		}
		if len(ring) == 0 {
			continue
		}
		var c Point
		for _, v := range ring {
			c.Lon += v[0]
			c.Lat += v[1]
		}
		c.Lon /= float64(len(ring))
		c.Lat /= float64(len(ring))
		out[f.Properties.Soato] = c
	}
	return out, nil
}

// RoadKm estimates the road distance between two points: the great-circle
// distance times 1.3, a usual detour factor.
func RoadKm(a, b Point) float64 {
	const r = 6371.0
	rad := math.Pi / 180
	dLat, dLon := (b.Lat-a.Lat)*rad, (b.Lon-a.Lon)*rad
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(a.Lat*rad)*math.Cos(b.Lat*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 1.3 * 2 * r * math.Asin(math.Sqrt(h))
}

// rings returns every ring of every viloyat keyed by SOATO.
func rings() (map[int64][][][2]float64, error) {
	var fc struct {
		Features []struct {
			Properties struct {
				Soato int64 `json:"soato"`
			} `json:"properties"`
			Geometry struct {
				Type        string          `json:"type"`
				Coordinates json.RawMessage `json:"coordinates"`
			} `json:"geometry"`
		} `json:"features"`
	}
	if err := json.Unmarshal(viloyatlar, &fc); err != nil {
		return nil, err
	}
	out := map[int64][][][2]float64{}
	for _, f := range fc.Features {
		var polys [][][][2]float64
		switch f.Geometry.Type {
		case "MultiPolygon":
			if err := json.Unmarshal(f.Geometry.Coordinates, &polys); err != nil {
				return nil, err
			}
		case "Polygon":
			var p [][][2]float64
			if err := json.Unmarshal(f.Geometry.Coordinates, &p); err != nil {
				return nil, err
			}
			polys = [][][][2]float64{p}
		}
		for _, p := range polys {
			out[f.Properties.Soato] = append(out[f.Properties.Soato], p...)
		}
	}
	return out, nil
}

// neighbourGrid: boundary points closer than about 1 km count as shared.
const neighbourGrid = 0.01

// Neighbors returns the viloyatlar that share a border with each one, keyed
// by SOATO: two boundaries are neighbours when at least two of their points
// fall into the same ~1 km cell (the boundaries come from one source, so a
// shared border has the same points). A viloyat with none, like Toshkent
// shahri inside Toshkent viloyati, gets the one with the nearest boundary.
func Neighbors() (map[int64][]int64, error) {
	rs, err := rings()
	if err != nil {
		return nil, err
	}
	type cellKey struct{ x, y int64 }
	cells := map[cellKey]map[int64]bool{}
	for soato, rr := range rs {
		for _, ring := range rr {
			for _, v := range ring {
				k := cellKey{int64(math.Floor(v[0] / neighbourGrid)), int64(math.Floor(v[1] / neighbourGrid))}
				if cells[k] == nil {
					cells[k] = map[int64]bool{}
				}
				cells[k][soato] = true
			}
		}
	}
	shared := map[[2]int64]int{}
	for _, owners := range cells {
		for a := range owners {
			for b := range owners {
				if a < b {
					shared[[2]int64{a, b}]++
				}
			}
		}
	}
	out := map[int64][]int64{}
	for soato := range rs {
		out[soato] = []int64{}
	}
	for pair, n := range shared {
		if n >= 2 {
			out[pair[0]] = append(out[pair[0]], pair[1])
			out[pair[1]] = append(out[pair[1]], pair[0])
		}
	}
	for soato, nb := range out {
		if len(nb) > 0 {
			continue
		}
		best, bestD := int64(0), math.Inf(1)
		for other, rr := range rs {
			if other == soato {
				continue
			}
			for _, a := range rs[soato] {
				for _, v := range a {
					for _, b := range rr {
						for _, w := range b {
							if d := math.Hypot(v[0]-w[0], v[1]-w[1]); d < bestD {
								best, bestD = other, d
							}
						}
					}
				}
			}
		}
		if best != 0 {
			out[soato] = append(out[soato], best)
			out[best] = append(out[best], soato)
		}
	}
	for k := range out {
		slices.Sort(out[k])
	}
	return out, nil
}

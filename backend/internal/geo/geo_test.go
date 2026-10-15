package geo

import (
	"slices"
	"testing"
)

func TestProvinces(t *testing.T) {
	p, err := Provinces()
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 14 {
		t.Fatalf("%d viloyat boundaries, want 14", len(p))
	}
	nam, ok := p[1714]
	if !ok || nam.Properties["name"] != "Namangan viloyati" || len(nam.Geometry) < 1000 {
		t.Errorf("Namangan: %v %v (%d bytes of geometry)", ok, nam.Properties, len(nam.Geometry))
	}
	// Callers fill properties; that must not leak into the next call.
	nam.Properties["median"] = 1
	again, _ := Provinces()
	if _, leaked := again[1714].Properties["median"]; leaked {
		t.Error("properties are shared between calls")
	}
}

func TestCentroidsAndDistance(t *testing.T) {
	c, err := Centroids()
	if err != nil || len(c) != 14 {
		t.Fatalf("centroids: %d, %v", len(c), err)
	}
	// Namangan (1714) to Toshkent shahri (1726) is about 290 km by road.
	d := RoadKm(c[1714], c[1726])
	if d < 200 || d > 400 {
		t.Errorf("Namangan–Toshkent %.0f km", d)
	}
	if RoadKm(c[1714], c[1714]) != 0 {
		t.Error("distance to itself")
	}
}

func TestNeighbors(t *testing.T) {
	nb, err := Neighbors()
	if err != nil {
		t.Fatal(err)
	}
	for soato, want := range map[int64][]int64{
		1726: {1727},             // Toshkent shahri: inside Toshkent viloyati
		1714: {1703, 1727, 1730}, // Namangan: Andijon, Toshkent viloyati, Fargʻona
		1722: {1710},             // Surxondaryo: Qashqadaryo only
	} {
		if !slices.Equal(nb[soato], want) {
			t.Errorf("%d: %v, want %v", soato, nb[soato], want)
		}
	}
	if len(nb) != 14 {
		t.Errorf("%d viloyatlar", len(nb))
	}
}

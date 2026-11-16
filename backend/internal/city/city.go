// Package city loads the embedded Bengaluru model: 243 BBMP wards, their adjacency graph,
// demand-component weights and provenance (built by data/pipeline/build_city.py).
package city

import (
	_ "embed"
	"encoding/json"
	"math"
)

//go:embed city.json
var cityJSON []byte

//go:embed wards.geojson
var wardsGeoJSON []byte

// Ward types, index order matters (used as policy features).
var TypeNames = []string{"Residential", "Tech & offices", "Transit hub", "Retail & leisure", "Mixed"}

type Ward struct {
	ID      int     `json:"id"`
	No      string  `json:"no"`
	Name    string  `json:"name"`
	Lng     float64 `json:"lng"`
	Lat     float64 `json:"lat"`
	AreaKm2 float64 `json:"area_km2"`
	CoreKm  float64 `json:"core_km"`
	Nbrs    []int   `json:"nbrs"`
	Schools int     `json:"schools"`
	Resid   float64 `json:"resid"`
	Employ  float64 `json:"employ"`
	Transit float64 `json:"transit"`
	Retail  float64 `json:"retail"`
	Type    string  `json:"type"`
	TypeIdx int     `json:"type_idx"`
}

type Source struct {
	Name    string `json:"name"`
	By      string `json:"by"`
	URL     string `json:"url"`
	Licence string `json:"licence"`
	UsedFor string `json:"used_for"`
}

type Anchor struct {
	Name   string  `json:"name"`
	Lat    float64 `json:"lat"`
	Lng    float64 `json:"lng"`
	Kind   string  `json:"kind"`
	Weight float64 `json:"weight"`
}

type Meta struct {
	City    string    `json:"city"`
	Wards   int       `json:"wards"`
	Origin  []float64 `json:"origin"`
	Sources []Source  `json:"sources"`
	Caveats []string  `json:"caveats"`
	Anchors []Anchor  `json:"anchors"`
}

type City struct {
	Meta  Meta   `json:"meta"`
	Wards []Ward `json:"wards"`

	// derived
	DistKm [][]float32 `json:"-"` // straight-line km between ward centroids
}

func Load() (*City, error) {
	var c City
	if err := json.Unmarshal(cityJSON, &c); err != nil {
		return nil, err
	}
	idx := map[string]int{}
	for i, n := range TypeNames {
		idx[n] = i
	}
	for i := range c.Wards {
		c.Wards[i].TypeIdx = idx[c.Wards[i].Type]
	}
	n := len(c.Wards)
	c.DistKm = make([][]float32, n)
	lat0 := c.Meta.Origin[1] * math.Pi / 180
	kx, ky := 111.32*math.Cos(lat0), 110.57
	for i := 0; i < n; i++ {
		c.DistKm[i] = make([]float32, n)
		for j := 0; j < n; j++ {
			dx := (c.Wards[i].Lng - c.Wards[j].Lng) * kx
			dy := (c.Wards[i].Lat - c.Wards[j].Lat) * ky
			c.DistKm[i][j] = float32(math.Hypot(dx, dy))
		}
	}
	return &c, nil
}

// GeoJSON returns the simplified ward polygons (served to the map).
func GeoJSON() []byte { return wardsGeoJSON }

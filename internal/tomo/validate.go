package tomo

import "math"

// Violation represents a link where the estimated delay is below the
// speed-of-light lower bound given the geographic distance.
type Violation struct {
	LinkID     int
	Estimated  float64 // estimated delay in ms
	LowerBound float64 // speed-of-light minimum delay in ms
	Distance   float64 // geographic distance in km
}

// ValidateDelays checks estimated delays against speed-of-light lower bounds.
// Returns violations where estimated delay < geo_distance * 0.005 ms/km.
// Also flags negative delays. Skips links whose nodes lack coordinates.
func ValidateDelays(sol *Solution, topo Topology) []Violation {
	nodes := topo.Nodes()
	nodeMap := make(map[int]Node, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID] = n
	}
	var out []Violation
	for i, l := range topo.Links() {
		est := sol.X.AtVec(i)
		if est < 0 {
			out = append(out, Violation{LinkID: l.ID, Estimated: est})
			continue
		}
		a, b := nodeMap[l.Src], nodeMap[l.Dst]
		if (a.Latitude == 0 && a.Longitude == 0) || (b.Latitude == 0 && b.Longitude == 0) {
			continue
		}
		d := geoDistKm(a, b)
		lb := d * 0.005
		if est < lb {
			out = append(out, Violation{LinkID: l.ID, Estimated: est, LowerBound: lb, Distance: d})
		}
	}
	return out
}

func geoDistKm(a, b Node) float64 {
	const R = 6371.0
	lat1, lat2 := a.Latitude*math.Pi/180, b.Latitude*math.Pi/180
	dlat := (b.Latitude - a.Latitude) * math.Pi / 180
	dlon := (b.Longitude - a.Longitude) * math.Pi / 180
	h := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dlon/2)*math.Sin(dlon/2)
	return 2 * R * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
}

package topology

import "testing"

func FuzzParseGraphML(f *testing.F) {
	f.Add([]byte(`<?xml version="1.0"?><graphml xmlns="http://graphml.graphdrawing.org/xmlns"><graph edgedefault="undirected" id="G"></graph></graphml>`))
	f.Add([]byte(`<?xml version="1.0"?><graphml xmlns="http://graphml.graphdrawing.org/xmlns"><key id="d0" for="node" attr.name="label" attr.type="string"/><graph edgedefault="undirected"><node id="0"><data key="d0">A</data></node><node id="1"><data key="d0">B</data></node><edge source="0" target="1"/></graph></graphml>`))
	f.Fuzz(func(t *testing.T, data []byte) {
		ParseGraphML(data) //nolint:errcheck // must not panic
	})
}

package topology

import (
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Darkroom4364/netlens/tomo"
)

// LoadGraphML parses a Topology Zoo GraphML file and returns a Graph.
func LoadGraphML(path string) (*Graph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read graphml: %w", err)
	}
	return ParseGraphML(data)
}

// ParseGraphML parses GraphML XML data into a Graph.
func ParseGraphML(data []byte) (*Graph, error) {
	var doc graphmlDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse graphml: %w", err)
	}

	// Build key index: id → attr.name
	keyNames := make(map[string]string)
	for _, k := range doc.Keys {
		keyNames[k.ID] = k.AttrName
	}

	g := New()

	// Parse nodes
	nodeIDMap := make(map[string]int) // graphml string ID → integer ID
	for i, n := range doc.Graph.Nodes {
		node := tomo.Node{ID: i}

		// Extract label and coordinates from data elements
		for _, d := range n.Data {
			name := keyNames[d.Key]
			switch strings.ToLower(name) {
			case "label":
				if label := strings.TrimSpace(d.Value); label != "" {
					node.Label = label
				}
			case "latitude":
				if v, err := strconv.ParseFloat(strings.TrimSpace(d.Value), 64); err == nil {
					node.Latitude = v
				}
			case "longitude":
				if v, err := strconv.ParseFloat(strings.TrimSpace(d.Value), 64); err == nil {
					node.Longitude = v
				}
			}

			// Try to extract label from yEd NodeLabel
			if node.Label == "" {
				node.Label = extractYEdLabel(d.InnerXML)
			}
		}

		// Try extracting coordinates from yEd Geometry as layout fallback
		if node.Latitude == 0 && node.Longitude == 0 {
			for _, d := range n.Data {
				if lat, lon, ok := extractYEdGeometry(d.InnerXML); ok {
					node.Latitude = lat
					node.Longitude = lon
					break
				}
			}
		}

		if node.Label == "" {
			node.Label = fmt.Sprintf("n%d", i)
		}

		nodeIDMap[n.ID] = i
		g.AddNode(node)
	}

	// Parse edges
	for _, e := range doc.Graph.Edges {
		src, srcOK := nodeIDMap[e.Source]
		dst, dstOK := nodeIDMap[e.Target]
		if !srcOK || !dstOK {
			continue
		}
		g.AddLink(src, dst)
	}

	return g, nil
}

// extractYEdLabel extracts the label text from yEd NodeLabel XML.
func extractYEdLabel(innerXML string) string {
	// Look for <y:NodeLabel ...>text</y:NodeLabel>
	const startTag = "<y:NodeLabel"
	idx := strings.Index(innerXML, startTag)
	if idx < 0 {
		return ""
	}
	// Find the closing >
	closeTag := strings.Index(innerXML[idx:], ">")
	if closeTag < 0 {
		return ""
	}
	rest := innerXML[idx+closeTag+1:]
	// Find </y:NodeLabel>
	endTag := strings.Index(rest, "</y:NodeLabel>")
	if endTag < 0 {
		return ""
	}
	text := rest[:endTag]
	// Strip nested XML elements (e.g. <y:LabelModel>...</y:LabelModel>).
	if cut := strings.Index(text, "<"); cut >= 0 {
		text = text[:cut]
	}
	return strings.TrimSpace(text)
}

// extractYEdGeometry extracts x, y coordinates from yEd Geometry element.
// These are layout coordinates, not geographic, but useful as relative positions.
func extractYEdGeometry(innerXML string) (lat, lon float64, ok bool) {
	const tag = "<y:Geometry"
	idx := strings.Index(innerXML, tag)
	if idx < 0 {
		return 0, 0, false
	}
	geom := innerXML[idx:]
	endIdx := strings.Index(geom, "/>")
	if endIdx < 0 {
		endIdx = strings.Index(geom, ">")
	}
	if endIdx < 0 {
		return 0, 0, false
	}
	geom = geom[:endIdx]

	x := extractAttr(geom, "x")
	y := extractAttr(geom, "y")
	if x == "" || y == "" {
		return 0, 0, false
	}

	xf, err1 := strconv.ParseFloat(x, 64)
	yf, err2 := strconv.ParseFloat(y, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}

	// Use y as latitude (north-south), x as longitude (east-west)
	return yf, xf, true
}

func extractAttr(s, name string) string {
	key := name + `="`
	idx := strings.Index(s, key)
	if idx < 0 {
		return ""
	}
	rest := s[idx+len(key):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// GraphML XML structures

type graphmlDoc struct {
	XMLName xml.Name     `xml:"graphml"`
	Keys    []graphmlKey `xml:"key"`
	Graph   graphmlGraph `xml:"graph"`
}

type graphmlKey struct {
	ID       string `xml:"id,attr"`
	For      string `xml:"for,attr"`
	AttrName string `xml:"attr.name,attr"`
	AttrType string `xml:"attr.type,attr"`
}

type graphmlGraph struct {
	Nodes []graphmlNode `xml:"node"`
	Edges []graphmlEdge `xml:"edge"`
}

type graphmlNode struct {
	ID   string        `xml:"id,attr"`
	Data []graphmlData `xml:"data"`
}

type graphmlEdge struct {
	Source string `xml:"source,attr"`
	Target string `xml:"target,attr"`
}

type graphmlData struct {
	Key      string `xml:"key,attr"`
	Value    string `xml:",chardata"`
	InnerXML string `xml:",innerxml"`
}

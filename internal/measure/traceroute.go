package measure

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/Darkroom4364/netlens/internal/tomo"
)

// ParseScamperJSON parses scamper's warts2json output (JSON array of traceroute results).
// Each element is a single probe's traceroute result.
func ParseScamperJSON(data []byte) ([]tomo.PathMeasurement, error) {
	var results []scamperTrace
	if err := json.Unmarshal(data, &results); err != nil {
		// Try single object (not array)
		var single scamperTrace
		if err2 := json.Unmarshal(data, &single); err2 != nil {
			return nil, fmt.Errorf("parse scamper json: %w", err)
		}
		results = []scamperTrace{single}
	}

	var measurements []tomo.PathMeasurement
	for _, r := range results {
		if r.Type != "trace" && r.Type != "traceroute" {
			continue
		}
		m := convertScamperTrace(r)
		measurements = append(measurements, m)
	}
	return measurements, nil
}

// ParseScamperFile reads and parses a scamper JSON file.
func ParseScamperFile(path string) ([]tomo.PathMeasurement, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scamper file: %w", err)
	}
	return ParseScamperJSON(data)
}

// ParseRIPEAtlasTraceroute parses RIPE Atlas traceroute result JSON.
// The input is a JSON array of traceroute results (as returned by the results endpoint).
func ParseRIPEAtlasTraceroute(data []byte) ([]tomo.PathMeasurement, error) {
	var results []ripeAtlasTrace
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("parse ripe atlas json: %w", err)
	}

	var measurements []tomo.PathMeasurement
	for _, r := range results {
		if r.Type != "traceroute" {
			continue
		}
		m := convertRIPETrace(r)
		measurements = append(measurements, m)
	}
	return measurements, nil
}

// ParseMTRJSON parses mtr --json output.
func ParseMTRJSON(data []byte) ([]tomo.PathMeasurement, error) {
	var report mtrReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("parse mtr json: %w", err)
	}

	m := convertMTRReport(report)
	return []tomo.PathMeasurement{m}, nil
}

// --- Scamper types ---

type scamperTrace struct {
	Type    string        `json:"type"`
	Src     string        `json:"src"`
	Dst     string        `json:"dst"`
	Start   scamperTime   `json:"start"`
	Hops    []scamperHop  `json:"hops"`
}

type scamperTime struct {
	Sec int64 `json:"sec"`
}

type scamperHop struct {
	Addr     string  `json:"addr"`
	ProbeTTL int     `json:"probe_ttl"`
	RTT      float64 `json:"rtt"` // microseconds in scamper
	ICMPExt  *scamperICMPExt `json:"icmpext,omitempty"`
}

type scamperICMPExt struct {
	MPLS []scamperMPLS `json:"mpls,omitempty"`
}

type scamperMPLS struct {
	Label int `json:"label"`
	TTL   int `json:"ttl"`
}

func convertScamperTrace(r scamperTrace) tomo.PathMeasurement {
	var hops []tomo.Hop
	var lastHopRTTs []time.Duration

	for _, h := range r.Hops {
		hop := tomo.Hop{
			IP:  h.Addr,
			TTL: h.ProbeTTL,
			RTT: time.Duration(h.RTT * float64(time.Microsecond)), // scamper uses μs
		}
		if h.Addr == "" || h.Addr == "0.0.0.0" {
			hop.Anonymous = true
		}
		if h.ICMPExt != nil && len(h.ICMPExt.MPLS) > 0 {
			hop.MPLS = true
		}
		hops = append(hops, hop)
	}

	// End-to-end RTTs: collect RTTs from the last hop (destination)
	if len(r.Hops) > 0 {
		lastTTL := r.Hops[len(r.Hops)-1].ProbeTTL
		for _, h := range r.Hops {
			if h.ProbeTTL == lastTTL && h.Addr != "" {
				lastHopRTTs = append(lastHopRTTs, time.Duration(h.RTT*float64(time.Microsecond)))
			}
		}
	}

	return tomo.PathMeasurement{
		Src:       r.Src,
		Dst:       r.Dst,
		Hops:      hops,
		RTTs:      lastHopRTTs,
		Timestamp: time.Unix(r.Start.Sec, 0),
		Weight:    1.0,
	}
}

// --- RIPE Atlas types ---

type ripeAtlasTrace struct {
	Type      string          `json:"type"`
	MsmID     int             `json:"msm_id"`
	ProbeID   int             `json:"probe_id"`
	SrcAddr   string          `json:"src_addr"`
	DstAddr   string          `json:"dst_addr"`
	DstName   string          `json:"dst_name"`
	Timestamp int64           `json:"timestamp"`
	Proto     string          `json:"proto"`
	ParisID   int             `json:"paris_id"`
	Result    []ripeAtlasHop  `json:"result"`
	LTS       int             `json:"lts"` // seconds since last time sync
}

type ripeAtlasHop struct {
	Hop    int                   `json:"hop"`
	Result []ripeAtlasHopResult  `json:"result"`
}

type ripeAtlasHopResult struct {
	From    string              `json:"from,omitempty"`
	RTT     float64             `json:"rtt,omitempty"`   // milliseconds
	Size    int                 `json:"size,omitempty"`
	TTL     int                 `json:"ttl,omitempty"`
	Timeout string              `json:"x,omitempty"`     // "*" if timeout
	ICMPExt *ripeAtlasICMPExt   `json:"icmpext,omitempty"`
}

type ripeAtlasICMPExt struct {
	Version int                 `json:"version"`
	RFC4884 int                 `json:"rfc4884"`
	Obj     []ripeAtlasExtObj   `json:"obj"`
}

type ripeAtlasExtObj struct {
	Class int                   `json:"class"`
	Type  int                   `json:"type"`
	MPLS  []ripeAtlasMPLS       `json:"mpls,omitempty"`
}

type ripeAtlasMPLS struct {
	Label int `json:"label"`
	Exp   int `json:"exp"`
	S     int `json:"s"`
	TTL   int `json:"ttl"`
}

func convertRIPETrace(r ripeAtlasTrace) tomo.PathMeasurement {
	var hops []tomo.Hop
	var lastHopRTTs []time.Duration

	for _, h := range r.Result {
		// Take the first responding result per hop for the hop list
		hop := tomo.Hop{TTL: h.Hop, Anonymous: true}

		for _, res := range h.Result {
			if res.Timeout == "*" || res.From == "" {
				continue
			}
			if hop.Anonymous { // first responding result
				hop.IP = res.From
				hop.RTT = time.Duration(res.RTT * float64(time.Millisecond))
				hop.Anonymous = false
				if res.ICMPExt != nil {
					for _, obj := range res.ICMPExt.Obj {
						if len(obj.MPLS) > 0 {
							hop.MPLS = true
						}
					}
				}
			}
		}
		hops = append(hops, hop)
	}

	// End-to-end RTTs: all RTT samples from the last hop
	if len(r.Result) > 0 {
		lastHop := r.Result[len(r.Result)-1]
		for _, res := range lastHop.Result {
			if res.Timeout != "*" && res.RTT > 0 {
				lastHopRTTs = append(lastHopRTTs, time.Duration(res.RTT*float64(time.Millisecond)))
			}
		}
	}

	return tomo.PathMeasurement{
		Src:       r.SrcAddr,
		Dst:       r.DstAddr,
		Hops:      hops,
		RTTs:      lastHopRTTs,
		Timestamp: time.Unix(r.Timestamp, 0),
		Weight:    1.0,
	}
}

// --- MTR types ---

type mtrReport struct {
	Report struct {
		Mtr struct {
			Src string `json:"src"`
			Dst string `json:"dst"`
		} `json:"mtr"`
		Hubs []mtrHub `json:"hubs"`
	} `json:"report"`
}

type mtrHub struct {
	Count int     `json:"count"`
	Host  string  `json:"host"`
	Loss  float64 `json:"Loss%"`
	Avg   float64 `json:"Avg"`
	Best  float64 `json:"Best"`
	Worst float64 `json:"Wrst"`
	StDev float64 `json:"StDev"`
}

func convertMTRReport(r mtrReport) tomo.PathMeasurement {
	var hops []tomo.Hop

	for i, h := range r.Report.Hubs {
		hop := tomo.Hop{
			IP:  h.Host,
			TTL: i + 1,
			RTT: time.Duration(h.Best * float64(time.Millisecond)),
		}
		if h.Host == "???" || h.Host == "" {
			hop.Anonymous = true
		}
		hops = append(hops, hop)
	}

	// End-to-end RTT from the last hub
	var rtts []time.Duration
	if len(r.Report.Hubs) > 0 {
		last := r.Report.Hubs[len(r.Report.Hubs)-1]
		rtts = append(rtts,
			time.Duration(last.Best*float64(time.Millisecond)),
			time.Duration(last.Avg*float64(time.Millisecond)),
			time.Duration(last.Worst*float64(time.Millisecond)),
		)
	}

	return tomo.PathMeasurement{
		Src:       r.Report.Mtr.Src,
		Dst:       r.Report.Mtr.Dst,
		Hops:      hops,
		RTTs:      rtts,
		Timestamp: time.Now(),
		Weight:    1.0,
	}
}

package measure

import (
	"fmt"
	"strings"
	"testing"
)

// mustNotPanic wraps a function call and fails the test if it panics.
func mustNotPanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("%s panicked: %v", name, r)
		}
	}()
	fn()
}

// ---------------------------------------------------------------------------
// Shared edge-case inputs
// ---------------------------------------------------------------------------

var (
	inputEmptyArray   = []byte(`[]`)
	inputEmptyObject  = []byte(`{}`)
	inputNull         = []byte(`null`)
	inputEmptyString  = []byte(`""`)
	inputTruncated    = []byte(`[{"type":"trace`)
	inputWrongType    = []byte(`[{"type":"dns","src":"1.1.1.1"}]`)
)

// ---------------------------------------------------------------------------
// ParseScamperJSON edge cases
// ---------------------------------------------------------------------------

func TestEdgeScamperEmptyArray(t *testing.T) {
	mustNotPanic(t, "ParseScamperJSON([])", func() {
		ms, err := ParseScamperJSON(inputEmptyArray)
		if err != nil {
			t.Logf("returned error (acceptable): %v", err)
		}
		t.Logf("measurements: %d", len(ms))
	})
}

func TestEdgeScamperEmptyObject(t *testing.T) {
	mustNotPanic(t, "ParseScamperJSON({})", func() {
		ms, err := ParseScamperJSON(inputEmptyObject)
		if err != nil {
			t.Logf("returned error (acceptable): %v", err)
		}
		t.Logf("measurements: %d", len(ms))
	})
}

func TestEdgeScamperNull(t *testing.T) {
	mustNotPanic(t, "ParseScamperJSON(null)", func() {
		ms, err := ParseScamperJSON(inputNull)
		if err != nil {
			t.Logf("returned error (acceptable): %v", err)
		}
		t.Logf("measurements: %d", len(ms))
	})
}

func TestEdgeScamperEmptyString(t *testing.T) {
	mustNotPanic(t, "ParseScamperJSON(\"\")", func() {
		ms, err := ParseScamperJSON(inputEmptyString)
		if err != nil {
			t.Logf("returned error (acceptable): %v", err)
		}
		t.Logf("measurements: %d", len(ms))
	})
}

func TestEdgeScamperTruncatedJSON(t *testing.T) {
	mustNotPanic(t, "ParseScamperJSON(truncated)", func() {
		_, err := ParseScamperJSON(inputTruncated)
		if err == nil {
			t.Fatal("expected error for truncated JSON, got nil")
		}
		t.Logf("returned error (expected): %v", err)
	})
}

func TestEdgeScamperWrongType(t *testing.T) {
	mustNotPanic(t, "ParseScamperJSON(wrong type)", func() {
		ms, err := ParseScamperJSON(inputWrongType)
		if err != nil {
			t.Logf("returned error (acceptable): %v", err)
		}
		// "dns" type should be skipped, so 0 measurements.
		if len(ms) != 0 {
			t.Fatalf("expected 0 measurements for wrong type, got %d", len(ms))
		}
	})
}

func TestEdgeScamperMissingHops(t *testing.T) {
	input := []byte(`[{"type":"trace","src":"1.1.1.1","dst":"2.2.2.2","hops":null}]`)
	mustNotPanic(t, "ParseScamperJSON(missing hops)", func() {
		ms, err := ParseScamperJSON(input)
		if err != nil {
			t.Logf("returned error (acceptable): %v", err)
			return
		}
		if len(ms) != 1 {
			t.Fatalf("expected 1 measurement, got %d", len(ms))
		}
		t.Logf("hops: %d", len(ms[0].Hops))
	})
}

func TestEdgeScamperAllAnonymousHops(t *testing.T) {
	input := []byte(`[{"type":"trace","src":"1.1.1.1","dst":"2.2.2.2","hops":[
		{"addr":"","probe_ttl":1,"rtt":0},
		{"addr":"","probe_ttl":2,"rtt":0},
		{"addr":"","probe_ttl":3,"rtt":0}
	]}]`)
	mustNotPanic(t, "ParseScamperJSON(all anonymous)", func() {
		ms, err := ParseScamperJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ms) != 1 {
			t.Fatalf("expected 1 measurement, got %d", len(ms))
		}
		for _, h := range ms[0].Hops {
			if !h.Anonymous {
				t.Fatalf("expected all hops anonymous, got non-anonymous hop at TTL %d", h.TTL)
			}
		}
	})
}

func TestEdgeScamperNegativeRTT(t *testing.T) {
	input := []byte(`[{"type":"trace","src":"1.1.1.1","dst":"2.2.2.2","hops":[
		{"addr":"10.0.0.1","probe_ttl":1,"rtt":-5.0}
	]}]`)
	mustNotPanic(t, "ParseScamperJSON(negative RTT)", func() {
		ms, err := ParseScamperJSON(input)
		if err != nil {
			t.Logf("returned error (acceptable): %v", err)
			return
		}
		t.Logf("hop RTT: %v", ms[0].Hops[0].RTT)
	})
}

func TestEdgeScamperZeroRTT(t *testing.T) {
	input := []byte(`[{"type":"trace","src":"1.1.1.1","dst":"2.2.2.2","hops":[
		{"addr":"10.0.0.1","probe_ttl":1,"rtt":0.0}
	]}]`)
	mustNotPanic(t, "ParseScamperJSON(zero RTT)", func() {
		ms, err := ParseScamperJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ms[0].Hops[0].RTT != 0 {
			t.Fatalf("expected zero RTT, got %v", ms[0].Hops[0].RTT)
		}
	})
}

func TestEdgeScamperExtremelyLargeRTT(t *testing.T) {
	input := []byte(`[{"type":"trace","src":"1.1.1.1","dst":"2.2.2.2","hops":[
		{"addr":"10.0.0.1","probe_ttl":1,"rtt":999999999.0}
	]}]`)
	mustNotPanic(t, "ParseScamperJSON(large RTT)", func() {
		ms, err := ParseScamperJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Logf("hop RTT: %v", ms[0].Hops[0].RTT)
	})
}

func TestEdgeScamperMissingFromField(t *testing.T) {
	// scamper uses "addr", a hop without addr
	input := []byte(`[{"type":"trace","src":"1.1.1.1","dst":"2.2.2.2","hops":[
		{"probe_ttl":1,"rtt":10.0}
	]}]`)
	mustNotPanic(t, "ParseScamperJSON(missing addr)", func() {
		ms, err := ParseScamperJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ms[0].Hops[0].Anonymous {
			t.Fatal("expected hop with missing addr to be anonymous")
		}
	})
}

func TestEdgeScamperDuplicateHopNumbers(t *testing.T) {
	input := []byte(`[{"type":"trace","src":"1.1.1.1","dst":"2.2.2.2","hops":[
		{"addr":"10.0.0.1","probe_ttl":1,"rtt":10.0},
		{"addr":"10.0.0.2","probe_ttl":1,"rtt":12.0},
		{"addr":"10.0.0.3","probe_ttl":2,"rtt":20.0}
	]}]`)
	mustNotPanic(t, "ParseScamperJSON(duplicate TTLs)", func() {
		ms, err := ParseScamperJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Logf("hops: %d", len(ms[0].Hops))
	})
}

func TestEdgeScamperHopsOutOfOrder(t *testing.T) {
	input := []byte(`[{"type":"trace","src":"1.1.1.1","dst":"2.2.2.2","hops":[
		{"addr":"10.0.0.3","probe_ttl":3,"rtt":30.0},
		{"addr":"10.0.0.1","probe_ttl":1,"rtt":10.0},
		{"addr":"10.0.0.2","probe_ttl":2,"rtt":20.0}
	]}]`)
	mustNotPanic(t, "ParseScamperJSON(out of order)", func() {
		ms, err := ParseScamperJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Logf("hops: %d", len(ms[0].Hops))
	})
}

func TestEdgeScamperEmptyIPString(t *testing.T) {
	input := []byte(`[{"type":"trace","src":"1.1.1.1","dst":"2.2.2.2","hops":[
		{"addr":"","probe_ttl":1,"rtt":1.0}
	]}]`)
	mustNotPanic(t, "ParseScamperJSON(empty addr)", func() {
		ms, err := ParseScamperJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ms[0].Hops[0].Anonymous {
			t.Fatal("expected empty addr hop to be anonymous")
		}
	})
}

func TestEdgeScamperIPv6(t *testing.T) {
	input := []byte(`[{"type":"trace","src":"2001:db8::1","dst":"2001:db8::2","hops":[
		{"addr":"2001:db8::1","probe_ttl":1,"rtt":5.0}
	]}]`)
	mustNotPanic(t, "ParseScamperJSON(IPv6)", func() {
		ms, err := ParseScamperJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ms[0].Hops[0].IP != "2001:db8::1" {
			t.Fatalf("expected IPv6 addr preserved, got %q", ms[0].Hops[0].IP)
		}
	})
}

func TestEdgeScamperMPLSEmptyLabelArray(t *testing.T) {
	input := []byte(`[{"type":"trace","src":"1.1.1.1","dst":"2.2.2.2","hops":[
		{"addr":"10.0.0.1","probe_ttl":1,"rtt":5.0,"icmpext":{"mpls":[]}}
	]}]`)
	mustNotPanic(t, "ParseScamperJSON(empty MPLS)", func() {
		ms, err := ParseScamperJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ms[0].Hops[0].MPLS {
			t.Fatal("expected MPLS=false for empty label array")
		}
	})
}

func TestEdgeScamperMassiveHopCount(t *testing.T) {
	var hops []string
	for i := 1; i <= 256; i++ {
		hops = append(hops, fmt.Sprintf(`{"addr":"10.0.%d.%d","probe_ttl":%d,"rtt":%d}`, i/256, i%256, i, i*10))
	}
	input := []byte(fmt.Sprintf(`[{"type":"trace","src":"1.1.1.1","dst":"2.2.2.2","hops":[%s]}]`, strings.Join(hops, ",")))
	mustNotPanic(t, "ParseScamperJSON(256 hops)", func() {
		ms, err := ParseScamperJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ms[0].Hops) != 256 {
			t.Fatalf("expected 256 hops, got %d", len(ms[0].Hops))
		}
	})
}

func TestEdgeScamperUnicodeInFields(t *testing.T) {
	input := []byte(`[{"type":"trace","src":"1.1.1.1","dst":"2.2.2.2","hops":[
		{"addr":"日本語","probe_ttl":1,"rtt":1.0}
	]}]`)
	mustNotPanic(t, "ParseScamperJSON(unicode addr)", func() {
		ms, err := ParseScamperJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ms[0].Hops[0].IP != "日本語" {
			t.Fatalf("expected unicode preserved, got %q", ms[0].Hops[0].IP)
		}
	})
}

func TestEdgeScamperIntegerWhereFloatExpected(t *testing.T) {
	input := []byte(`[{"type":"trace","src":"1.1.1.1","dst":"2.2.2.2","hops":[
		{"addr":"10.0.0.1","probe_ttl":1,"rtt":5}
	]}]`)
	mustNotPanic(t, "ParseScamperJSON(int RTT)", func() {
		ms, err := ParseScamperJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Logf("hop RTT: %v", ms[0].Hops[0].RTT)
	})
}

// ---------------------------------------------------------------------------
// ParseRIPEAtlasTraceroute edge cases
// ---------------------------------------------------------------------------

func TestEdgeRIPEEmptyArray(t *testing.T) {
	mustNotPanic(t, "ParseRIPEAtlasTraceroute([])", func() {
		ms, err := ParseRIPEAtlasTraceroute(inputEmptyArray)
		if err != nil {
			t.Logf("returned error (acceptable): %v", err)
		}
		t.Logf("measurements: %d", len(ms))
	})
}

func TestEdgeRIPEEmptyObject(t *testing.T) {
	mustNotPanic(t, "ParseRIPEAtlasTraceroute({})", func() {
		_, err := ParseRIPEAtlasTraceroute(inputEmptyObject)
		if err != nil {
			t.Logf("returned error (expected): %v", err)
		}
	})
}

func TestEdgeRIPENull(t *testing.T) {
	mustNotPanic(t, "ParseRIPEAtlasTraceroute(null)", func() {
		ms, err := ParseRIPEAtlasTraceroute(inputNull)
		if err != nil {
			t.Logf("returned error (acceptable): %v", err)
		}
		t.Logf("measurements: %d", len(ms))
	})
}

func TestEdgeRIPEEmptyString(t *testing.T) {
	mustNotPanic(t, "ParseRIPEAtlasTraceroute(\"\")", func() {
		_, err := ParseRIPEAtlasTraceroute(inputEmptyString)
		if err != nil {
			t.Logf("returned error (expected): %v", err)
		}
	})
}

func TestEdgeRIPETruncatedJSON(t *testing.T) {
	mustNotPanic(t, "ParseRIPEAtlasTraceroute(truncated)", func() {
		_, err := ParseRIPEAtlasTraceroute(inputTruncated)
		if err == nil {
			t.Fatal("expected error for truncated JSON, got nil")
		}
	})
}

func TestEdgeRIPEWrongType(t *testing.T) {
	input := []byte(`[{"type":"dns","src_addr":"1.1.1.1"}]`)
	mustNotPanic(t, "ParseRIPEAtlasTraceroute(wrong type)", func() {
		ms, err := ParseRIPEAtlasTraceroute(input)
		if err != nil {
			t.Logf("returned error (acceptable): %v", err)
			return
		}
		if len(ms) != 0 {
			t.Fatalf("expected 0 measurements for wrong type, got %d", len(ms))
		}
	})
}

func TestEdgeRIPEMissingHops(t *testing.T) {
	input := []byte(`[{"type":"traceroute","src_addr":"1.1.1.1","dst_addr":"2.2.2.2"}]`)
	mustNotPanic(t, "ParseRIPEAtlasTraceroute(no result array)", func() {
		ms, err := ParseRIPEAtlasTraceroute(input)
		if err != nil {
			t.Logf("returned error (acceptable): %v", err)
			return
		}
		if len(ms) != 1 {
			t.Fatalf("expected 1 measurement, got %d", len(ms))
		}
		t.Logf("hops: %d", len(ms[0].Hops))
	})
}

func TestEdgeRIPEAllAnonymousHops(t *testing.T) {
	input := []byte(`[{"type":"traceroute","src_addr":"1.1.1.1","dst_addr":"2.2.2.2","result":[
		{"hop":1,"result":[{"x":"*"}]},
		{"hop":2,"result":[{"x":"*"}]},
		{"hop":3,"result":[{"x":"*"}]}
	]}]`)
	mustNotPanic(t, "ParseRIPEAtlasTraceroute(all anonymous)", func() {
		ms, err := ParseRIPEAtlasTraceroute(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, h := range ms[0].Hops {
			if !h.Anonymous {
				t.Fatalf("expected all anonymous, got responding hop at TTL %d", h.TTL)
			}
		}
	})
}

func TestEdgeRIPENegativeRTT(t *testing.T) {
	input := []byte(`[{"type":"traceroute","src_addr":"1.1.1.1","dst_addr":"2.2.2.2","result":[
		{"hop":1,"result":[{"from":"10.0.0.1","rtt":-5.0}]}
	]}]`)
	mustNotPanic(t, "ParseRIPEAtlasTraceroute(negative RTT)", func() {
		ms, err := ParseRIPEAtlasTraceroute(input)
		if err != nil {
			t.Logf("returned error (acceptable): %v", err)
			return
		}
		t.Logf("hop RTT: %v", ms[0].Hops[0].RTT)
	})
}

func TestEdgeRIPEZeroRTT(t *testing.T) {
	input := []byte(`[{"type":"traceroute","src_addr":"1.1.1.1","dst_addr":"2.2.2.2","result":[
		{"hop":1,"result":[{"from":"10.0.0.1","rtt":0.0}]}
	]}]`)
	mustNotPanic(t, "ParseRIPEAtlasTraceroute(zero RTT)", func() {
		ms, err := ParseRIPEAtlasTraceroute(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Logf("hop RTT: %v", ms[0].Hops[0].RTT)
	})
}

func TestEdgeRIPEExtremelyLargeRTT(t *testing.T) {
	input := []byte(`[{"type":"traceroute","src_addr":"1.1.1.1","dst_addr":"2.2.2.2","result":[
		{"hop":1,"result":[{"from":"10.0.0.1","rtt":999999999.0}]}
	]}]`)
	mustNotPanic(t, "ParseRIPEAtlasTraceroute(large RTT)", func() {
		ms, err := ParseRIPEAtlasTraceroute(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Logf("hop RTT: %v", ms[0].Hops[0].RTT)
	})
}

func TestEdgeRIPEMissingFromField(t *testing.T) {
	input := []byte(`[{"type":"traceroute","src_addr":"1.1.1.1","dst_addr":"2.2.2.2","result":[
		{"hop":1,"result":[{"rtt":10.0}]}
	]}]`)
	mustNotPanic(t, "ParseRIPEAtlasTraceroute(missing from)", func() {
		ms, err := ParseRIPEAtlasTraceroute(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ms[0].Hops[0].Anonymous {
			t.Fatal("expected hop with missing from to be anonymous")
		}
	})
}

func TestEdgeRIPEDuplicateHopNumbers(t *testing.T) {
	input := []byte(`[{"type":"traceroute","src_addr":"1.1.1.1","dst_addr":"2.2.2.2","result":[
		{"hop":1,"result":[{"from":"10.0.0.1","rtt":10.0}]},
		{"hop":1,"result":[{"from":"10.0.0.2","rtt":12.0}]},
		{"hop":2,"result":[{"from":"10.0.0.3","rtt":20.0}]}
	]}]`)
	mustNotPanic(t, "ParseRIPEAtlasTraceroute(duplicate hops)", func() {
		ms, err := ParseRIPEAtlasTraceroute(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Logf("hops: %d", len(ms[0].Hops))
	})
}

func TestEdgeRIPEHopsOutOfOrder(t *testing.T) {
	input := []byte(`[{"type":"traceroute","src_addr":"1.1.1.1","dst_addr":"2.2.2.2","result":[
		{"hop":3,"result":[{"from":"10.0.0.3","rtt":30.0}]},
		{"hop":1,"result":[{"from":"10.0.0.1","rtt":10.0}]},
		{"hop":2,"result":[{"from":"10.0.0.2","rtt":20.0}]}
	]}]`)
	mustNotPanic(t, "ParseRIPEAtlasTraceroute(out of order)", func() {
		ms, err := ParseRIPEAtlasTraceroute(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Logf("hops: %d", len(ms[0].Hops))
	})
}

func TestEdgeRIPEEmptyIPString(t *testing.T) {
	input := []byte(`[{"type":"traceroute","src_addr":"1.1.1.1","dst_addr":"2.2.2.2","result":[
		{"hop":1,"result":[{"from":"","rtt":1.0}]}
	]}]`)
	mustNotPanic(t, "ParseRIPEAtlasTraceroute(empty from)", func() {
		ms, err := ParseRIPEAtlasTraceroute(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ms[0].Hops[0].Anonymous {
			t.Fatal("expected empty from to be anonymous")
		}
	})
}

func TestEdgeRIPEIPv6(t *testing.T) {
	input := []byte(`[{"type":"traceroute","src_addr":"2001:db8::1","dst_addr":"2001:db8::2","result":[
		{"hop":1,"result":[{"from":"2001:db8::1","rtt":5.0}]}
	]}]`)
	mustNotPanic(t, "ParseRIPEAtlasTraceroute(IPv6)", func() {
		ms, err := ParseRIPEAtlasTraceroute(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ms[0].Hops[0].IP != "2001:db8::1" {
			t.Fatalf("expected IPv6 preserved, got %q", ms[0].Hops[0].IP)
		}
	})
}

func TestEdgeRIPEMPLSEmptyLabelArray(t *testing.T) {
	input := []byte(`[{"type":"traceroute","src_addr":"1.1.1.1","dst_addr":"2.2.2.2","result":[
		{"hop":1,"result":[{"from":"10.0.0.1","rtt":5.0,"icmpext":{"version":2,"rfc4884":0,"obj":[{"class":1,"type":1,"mpls":[]}]}}]}
	]}]`)
	mustNotPanic(t, "ParseRIPEAtlasTraceroute(empty MPLS)", func() {
		ms, err := ParseRIPEAtlasTraceroute(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ms[0].Hops[0].MPLS {
			t.Fatal("expected MPLS=false for empty label array")
		}
	})
}

func TestEdgeRIPEMassiveHopCount(t *testing.T) {
	var hops []string
	for i := 1; i <= 256; i++ {
		hops = append(hops, fmt.Sprintf(`{"hop":%d,"result":[{"from":"10.0.%d.%d","rtt":%d}]}`, i, i/256, i%256, i*10))
	}
	input := []byte(fmt.Sprintf(`[{"type":"traceroute","src_addr":"1.1.1.1","dst_addr":"2.2.2.2","result":[%s]}]`, strings.Join(hops, ",")))
	mustNotPanic(t, "ParseRIPEAtlasTraceroute(256 hops)", func() {
		ms, err := ParseRIPEAtlasTraceroute(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ms[0].Hops) != 256 {
			t.Fatalf("expected 256 hops, got %d", len(ms[0].Hops))
		}
	})
}

func TestEdgeRIPEUnicodeInFields(t *testing.T) {
	input := []byte(`[{"type":"traceroute","src_addr":"1.1.1.1","dst_addr":"2.2.2.2","result":[
		{"hop":1,"result":[{"from":"日本語","rtt":1.0}]}
	]}]`)
	mustNotPanic(t, "ParseRIPEAtlasTraceroute(unicode)", func() {
		ms, err := ParseRIPEAtlasTraceroute(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Logf("hop IP: %q", ms[0].Hops[0].IP)
	})
}

func TestEdgeRIPEIntegerWhereFloatExpected(t *testing.T) {
	input := []byte(`[{"type":"traceroute","src_addr":"1.1.1.1","dst_addr":"2.2.2.2","result":[
		{"hop":1,"result":[{"from":"10.0.0.1","rtt":5}]}
	]}]`)
	mustNotPanic(t, "ParseRIPEAtlasTraceroute(int RTT)", func() {
		ms, err := ParseRIPEAtlasTraceroute(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Logf("hop RTT: %v", ms[0].Hops[0].RTT)
	})
}

// --- RIPE Atlas specific edge cases ---

func TestEdgeRIPEMissingTimestamp(t *testing.T) {
	input := []byte(`[{"type":"traceroute","src_addr":"1.1.1.1","dst_addr":"2.2.2.2","result":[
		{"hop":1,"result":[{"from":"10.0.0.1","rtt":5.0}]}
	]}]`)
	mustNotPanic(t, "ParseRIPEAtlasTraceroute(missing timestamp)", func() {
		ms, err := ParseRIPEAtlasTraceroute(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// timestamp defaults to zero (Unix epoch)
		t.Logf("timestamp: %v", ms[0].Timestamp)
	})
}

func TestEdgeRIPEProbeIDZero(t *testing.T) {
	input := []byte(`[{"type":"traceroute","probe_id":0,"src_addr":"1.1.1.1","dst_addr":"2.2.2.2","result":[
		{"hop":1,"result":[{"from":"10.0.0.1","rtt":5.0}]}
	]}]`)
	mustNotPanic(t, "ParseRIPEAtlasTraceroute(probe_id=0)", func() {
		ms, err := ParseRIPEAtlasTraceroute(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ms) != 1 {
			t.Fatalf("expected 1 measurement, got %d", len(ms))
		}
	})
}

func TestEdgeRIPEEmptySrcDstAddr(t *testing.T) {
	input := []byte(`[{"type":"traceroute","src_addr":"","dst_addr":"","result":[
		{"hop":1,"result":[{"from":"10.0.0.1","rtt":5.0}]}
	]}]`)
	mustNotPanic(t, "ParseRIPEAtlasTraceroute(empty src/dst)", func() {
		ms, err := ParseRIPEAtlasTraceroute(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ms[0].Src != "" || ms[0].Dst != "" {
			t.Fatalf("expected empty src/dst, got src=%q dst=%q", ms[0].Src, ms[0].Dst)
		}
	})
}

// ---------------------------------------------------------------------------
// ParseMTRJSON edge cases
// ---------------------------------------------------------------------------

func TestEdgeMTREmptyArray(t *testing.T) {
	mustNotPanic(t, "ParseMTRJSON([])", func() {
		_, err := ParseMTRJSON(inputEmptyArray)
		if err != nil {
			t.Logf("returned error (expected): %v", err)
		}
	})
}

func TestEdgeMTREmptyObject(t *testing.T) {
	mustNotPanic(t, "ParseMTRJSON({})", func() {
		ms, err := ParseMTRJSON(inputEmptyObject)
		if err != nil {
			t.Logf("returned error (acceptable): %v", err)
			return
		}
		t.Logf("measurements: %d, hops: %d", len(ms), len(ms[0].Hops))
	})
}

func TestEdgeMTRNull(t *testing.T) {
	mustNotPanic(t, "ParseMTRJSON(null)", func() {
		_, err := ParseMTRJSON(inputNull)
		if err != nil {
			t.Logf("returned error (acceptable): %v", err)
		}
	})
}

func TestEdgeMTREmptyString(t *testing.T) {
	mustNotPanic(t, "ParseMTRJSON(\"\")", func() {
		_, err := ParseMTRJSON(inputEmptyString)
		if err != nil {
			t.Logf("returned error (expected): %v", err)
		}
	})
}

func TestEdgeMTRTruncatedJSON(t *testing.T) {
	mustNotPanic(t, "ParseMTRJSON(truncated)", func() {
		_, err := ParseMTRJSON(inputTruncated)
		if err == nil {
			t.Fatal("expected error for truncated JSON, got nil")
		}
	})
}

func TestEdgeMTRMissingHops(t *testing.T) {
	input := []byte(`{"report":{"mtr":{"src":"1.1.1.1","dst":"2.2.2.2"}}}`)
	mustNotPanic(t, "ParseMTRJSON(no hubs)", func() {
		ms, err := ParseMTRJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ms) != 1 {
			t.Fatalf("expected 1 measurement, got %d", len(ms))
		}
		if len(ms[0].Hops) != 0 {
			t.Fatalf("expected 0 hops, got %d", len(ms[0].Hops))
		}
	})
}

func TestEdgeMTRAllAnonymousHops(t *testing.T) {
	input := []byte(`{"report":{"mtr":{"src":"1.1.1.1","dst":"2.2.2.2"},"hubs":[
		{"count":10,"host":"???","Loss%":100,"Avg":0,"Best":0,"Wrst":0,"StDev":0},
		{"count":10,"host":"","Loss%":100,"Avg":0,"Best":0,"Wrst":0,"StDev":0}
	]}}`)
	mustNotPanic(t, "ParseMTRJSON(all anonymous)", func() {
		ms, err := ParseMTRJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, h := range ms[0].Hops {
			if !h.Anonymous {
				t.Fatalf("expected all anonymous, got responding hop at TTL %d", h.TTL)
			}
		}
	})
}

func TestEdgeMTRNegativeRTT(t *testing.T) {
	input := []byte(`{"report":{"mtr":{"src":"1.1.1.1","dst":"2.2.2.2"},"hubs":[
		{"count":10,"host":"10.0.0.1","Loss%":0,"Avg":-5.0,"Best":-5.0,"Wrst":-5.0,"StDev":0}
	]}}`)
	mustNotPanic(t, "ParseMTRJSON(negative RTT)", func() {
		ms, err := ParseMTRJSON(input)
		if err != nil {
			t.Logf("returned error (acceptable): %v", err)
			return
		}
		t.Logf("hop RTT: %v", ms[0].Hops[0].RTT)
	})
}

func TestEdgeMTRZeroRTT(t *testing.T) {
	input := []byte(`{"report":{"mtr":{"src":"1.1.1.1","dst":"2.2.2.2"},"hubs":[
		{"count":10,"host":"10.0.0.1","Loss%":0,"Avg":0.0,"Best":0.0,"Wrst":0.0,"StDev":0}
	]}}`)
	mustNotPanic(t, "ParseMTRJSON(zero RTT)", func() {
		ms, err := ParseMTRJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ms[0].Hops[0].RTT != 0 {
			t.Fatalf("expected zero RTT, got %v", ms[0].Hops[0].RTT)
		}
	})
}

func TestEdgeMTRExtremelyLargeRTT(t *testing.T) {
	input := []byte(`{"report":{"mtr":{"src":"1.1.1.1","dst":"2.2.2.2"},"hubs":[
		{"count":10,"host":"10.0.0.1","Loss%":0,"Avg":999999999.0,"Best":999999999.0,"Wrst":999999999.0,"StDev":0}
	]}}`)
	mustNotPanic(t, "ParseMTRJSON(large RTT)", func() {
		ms, err := ParseMTRJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Logf("hop RTT: %v", ms[0].Hops[0].RTT)
	})
}

func TestEdgeMTREmptyIPString(t *testing.T) {
	input := []byte(`{"report":{"mtr":{"src":"1.1.1.1","dst":"2.2.2.2"},"hubs":[
		{"count":10,"host":"","Loss%":0,"Avg":1.0,"Best":1.0,"Wrst":1.0,"StDev":0}
	]}}`)
	mustNotPanic(t, "ParseMTRJSON(empty host)", func() {
		ms, err := ParseMTRJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ms[0].Hops[0].Anonymous {
			t.Fatal("expected empty host to be anonymous")
		}
	})
}

func TestEdgeMTRIPv6(t *testing.T) {
	input := []byte(`{"report":{"mtr":{"src":"2001:db8::1","dst":"2001:db8::2"},"hubs":[
		{"count":10,"host":"2001:db8::1","Loss%":0,"Avg":5.0,"Best":5.0,"Wrst":5.0,"StDev":0}
	]}}`)
	mustNotPanic(t, "ParseMTRJSON(IPv6)", func() {
		ms, err := ParseMTRJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ms[0].Hops[0].IP != "2001:db8::1" {
			t.Fatalf("expected IPv6 preserved, got %q", ms[0].Hops[0].IP)
		}
	})
}

func TestEdgeMTRMassiveHopCount(t *testing.T) {
	var hubs []string
	for i := 0; i < 256; i++ {
		hubs = append(hubs, fmt.Sprintf(`{"count":10,"host":"10.0.%d.%d","Loss%%":0,"Avg":%d,"Best":%d,"Wrst":%d,"StDev":0}`, i/256, i%256, (i+1)*10, (i+1)*10, (i+1)*10))
	}
	input := []byte(fmt.Sprintf(`{"report":{"mtr":{"src":"1.1.1.1","dst":"2.2.2.2"},"hubs":[%s]}}`, strings.Join(hubs, ",")))
	mustNotPanic(t, "ParseMTRJSON(256 hops)", func() {
		ms, err := ParseMTRJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ms[0].Hops) != 256 {
			t.Fatalf("expected 256 hops, got %d", len(ms[0].Hops))
		}
	})
}

func TestEdgeMTRUnicodeInFields(t *testing.T) {
	input := []byte(`{"report":{"mtr":{"src":"1.1.1.1","dst":"2.2.2.2"},"hubs":[
		{"count":10,"host":"日本語","Loss%":0,"Avg":1.0,"Best":1.0,"Wrst":1.0,"StDev":0}
	]}}`)
	mustNotPanic(t, "ParseMTRJSON(unicode host)", func() {
		ms, err := ParseMTRJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Logf("host: %q", ms[0].Hops[0].IP)
	})
}

func TestEdgeMTRIntegerWhereFloatExpected(t *testing.T) {
	input := []byte(`{"report":{"mtr":{"src":"1.1.1.1","dst":"2.2.2.2"},"hubs":[
		{"count":10,"host":"10.0.0.1","Loss%":0,"Avg":5,"Best":5,"Wrst":5,"StDev":0}
	]}}`)
	mustNotPanic(t, "ParseMTRJSON(int RTT)", func() {
		ms, err := ParseMTRJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Logf("hop RTT: %v", ms[0].Hops[0].RTT)
	})
}

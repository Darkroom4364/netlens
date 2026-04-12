package measure

import "testing"

func FuzzParseScamperJSON(f *testing.F) {
	f.Add([]byte(`[{"type":"trace","src":"1.1.1.1","dst":"2.2.2.2","start":{"sec":1},"hops":[]}]`))
	f.Add([]byte(`{"type":"trace","src":"10.0.0.1","dst":"10.0.0.2","start":{"sec":0},"hops":[{"addr":"10.0.0.3","probe_ttl":1,"rtt":1234.5}]}`))
	f.Add([]byte(`[]`))
	f.Fuzz(func(t *testing.T, data []byte) {
		ParseScamperJSON(data) //nolint:errcheck // must not panic
	})
}

func FuzzParseRIPEAtlasTraceroute(f *testing.F) {
	f.Add([]byte(`[{"type":"traceroute","msm_id":1,"probe_id":1,"src_addr":"1.1.1.1","dst_addr":"2.2.2.2","timestamp":1,"proto":"UDP","paris_id":0,"result":[{"hop":1,"result":[{"from":"3.3.3.3","rtt":12.5,"size":60,"ttl":252}]}],"lts":30}]`))
	f.Add([]byte(`[]`))
	f.Fuzz(func(t *testing.T, data []byte) {
		ParseRIPEAtlasTraceroute(data) //nolint:errcheck // must not panic
	})
}

func FuzzParseMTRJSON(f *testing.F) {
	f.Add([]byte(`{"report":{"mtr":{"src":"1.1.1.1","dst":"2.2.2.2"},"hubs":[{"count":10,"host":"3.3.3.3","Loss%":0.0,"Avg":15.2,"Best":12.1,"Wrst":20.3,"StDev":2.1}]}}`))
	f.Add([]byte(`{"report":{"mtr":{"src":"","dst":""},"hubs":[]}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		ParseMTRJSON(data) //nolint:errcheck // must not panic
	})
}

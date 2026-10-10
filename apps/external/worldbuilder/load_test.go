package worldbuilder

import (
	"fmt"
	"testing"
)

// Offline upper-bound domain microbenchmark, not representative LAN throughput.
func BenchmarkBoundedCanon5000(b *testing.B) {
	snapshot := Snapshot{"main": {ID: "main", Kind: "timeline", Label: "Main"}, "person": {ID: "person", Kind: "type", Label: "Person"}}
	for i := 0; i < 4998; i++ {
		id := fmt.Sprintf("person_%04d", i)
		snapshot[id] = Record{ID: id, Kind: "entity", Type: "person", Label: "Bridge traveller " + id}
	}
	record := Record{ID: "person_0000", Kind: "entity", Type: "person", Label: "Updated traveller"}
	op := []Operation{{Action: "put", ID: record.ID, Record: &record}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		after, err := Apply(snapshot, op)
		if err != nil {
			b.Fatal(err)
		}
		if len(Records(after, "main", "traveller", nil)) != 4998 {
			b.Fatal("bounded canon query lost records")
		}
	}
}

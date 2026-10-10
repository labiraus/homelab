package worldbuilder

// Connections returns explicit forward/back references at one immutable scope.
// It never follows attributes as URLs or interprets source text as references.
func Connections(snapshot Snapshot, id, timeline string, at *int64) ([]Record, error) {
	if snapshot[timeline].Kind != "timeline" {
		return nil, bad("unknown timeline")
	}
	root, ok := snapshot[id]
	if !ok || !Visible(root, timeline, at) {
		return nil, missing
	}
	wanted := map[string]bool{}
	for _, ref := range recordReferences(root) {
		wanted[ref] = true
	}
	result := []Record{}
	for _, record := range Records(snapshot, timeline, "", at) {
		if record.ID == id {
			continue
		}
		linked := wanted[record.ID]
		for _, ref := range recordReferences(record) {
			if ref == id {
				linked = true
			}
		}
		if linked {
			result = append(result, record)
		}
	}
	return result, nil
}

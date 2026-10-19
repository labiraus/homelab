package worldbuilder

type InventoryItem struct {
	EntityID string `json:"entityId"`
	Quantity int    `json:"quantity"`
}
type SceneState struct {
	Location  string          `json:"location,omitempty"`
	Inventory []InventoryItem `json:"inventory,omitempty"`
}

func (s SceneState) validate(snapshot Snapshot, timeline string) error {
	visible := func(id string) bool {
		r, ok := snapshot[id]
		return ok && r.Kind == "entity" && Visible(r, timeline, nil)
	}
	if s.Location != "" && !visible(s.Location) {
		return bad("location must reference a visible entity")
	}
	if len(s.Inventory) > 100 {
		return bad("inventory is limited to 100 entries")
	}
	seen := map[string]bool{}
	for _, item := range s.Inventory {
		if !visible(item.EntityID) || seen[item.EntityID] || item.Quantity < 1 || item.Quantity > 1000000 {
			return bad("inventory needs unique visible entities and quantities 1–1000000")
		}
		seen[item.EntityID] = true
	}
	return nil
}

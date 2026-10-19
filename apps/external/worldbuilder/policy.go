package worldbuilder

import (
	"context"
	"reflect"
)

type PublicationPolicy struct {
	Mode              string `json:"mode"`
	RequiredApprovals int    `json:"requiredApprovals"`
}

func (p PublicationPolicy) validate() error {
	if p.Mode != "independent" && p.Mode != "direct" && p.Mode != "unanimous" {
		return bad("publication policy must be independent, direct or unanimous")
	}
	if p.RequiredApprovals < 1 || p.RequiredApprovals > 5 {
		return bad("independent approval count must be 1–5")
	}
	return nil
}
func (sc *scope) protectLastAuthor(ctx context.Context, principal string) error {
	var last bool
	err := sc.tx.QueryRow(ctx, `SELECT EXISTS(SELECT FROM worldbuilder.member WHERE world_id=$1 AND principal=$2 AND role='author') AND (SELECT count(*) FROM worldbuilder.member WHERE world_id=$1 AND role='author')<=1`, sc.world.ID, principal).Scan(&last)
	if err != nil {
		return err
	}
	if last {
		return bad("add another Author before removing the final Author")
	}
	return nil
}
func (sc *scope) requiredApprovals(ctx context.Context) (int, error) {
	switch sc.world.Policy.Mode {
	case "direct":
		return 0, nil
	case "unanimous":
		var n int
		err := sc.tx.QueryRow(ctx, `SELECT count(*) FROM worldbuilder.member WHERE world_id=$1 AND role='author'`, sc.world.ID).Scan(&n)
		return n, err
	default:
		return sc.world.Policy.RequiredApprovals, nil
	}
}
func (sc *scope) hasQuorum(ctx context.Context, c Change) (bool, error) {
	if sc.world.Policy.Mode != "unanimous" {
		n := 0
		for _, actor := range c.Approvals {
			if actor != c.Author {
				n++
			}
		}
		return n >= sc.world.Policy.RequiredApprovals || sc.world.Policy.Mode == "direct", nil
	}
	var missing int
	err := sc.tx.QueryRow(ctx, `SELECT count(*) FROM worldbuilder.member WHERE world_id=$1 AND role='author' AND NOT(principal=ANY($2))`, sc.world.ID, c.Approvals).Scan(&missing)
	return missing == 0, err
}
func (sc *scope) checkPolicyEdit(before, after Snapshot) error {
	for id, r := range after {
		if r.Kind == "world_settings" && !reflect.DeepEqual(r.ReviewPolicy, before[id].ReviewPolicy) {
			return bad("publication policy is managed by Administrators in World Settings, not by canon edits")
		}
	}
	return nil
}

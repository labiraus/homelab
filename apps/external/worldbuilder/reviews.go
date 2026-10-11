package worldbuilder

import (
	"context"
	"time"
)

type ReviewPolicy struct {
	RequiredApprovals int `json:"requiredApprovals"`
}

type Review struct {
	Reviewer  string     `json:"reviewer"`
	Decision  string     `json:"decision"`
	Reason    string     `json:"reason"`
	CreatedAt *time.Time `json:"createdAt"`
	Legacy    bool       `json:"legacy"`
	Eligible  bool       `json:"eligible"`
}

func snapshotReviewPolicy(s Snapshot) ReviewPolicy {
	for _, r := range s {
		if r.Kind == "world_settings" && r.ReviewPolicy != nil {
			return *r.ReviewPolicy
		}
	}
	return ReviewPolicy{RequiredApprovals: 1}
}

func requiredReviews(before, after Snapshot) int {
	a, b := snapshotReviewPolicy(before).RequiredApprovals, snapshotReviewPolicy(after).RequiredApprovals
	if b > a {
		return b
	}
	return a
}

func (sc *scope) loadReviews(ctx context.Context, c *Change) error {
	rows, err := sc.tx.Query(ctx, `SELECT r.reviewer,r.decision,r.reason,r.created_at,r.legacy,
 COALESCE(m.role IN ('owner','admin','reviewer') AND m.authority_id=r.authority_id,false)
 FROM worldbuilder.change_review r LEFT JOIN worldbuilder.member m ON m.world_id=r.world_id AND m.principal=r.reviewer
 WHERE r.world_id=$1 AND r.change_id=$2 ORDER BY r.created_at NULLS FIRST,r.reviewer,r.authority_id`, sc.world.ID, c.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	c.Reviews = []Review{}
	if c.Status != "published" {
		c.Approvals = []string{}
	}
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.Reviewer, &r.Decision, &r.Reason, &r.CreatedAt, &r.Legacy, &r.Eligible); err != nil {
			return err
		}
		c.Reviews = append(c.Reviews, r)
		if c.Status != "published" && r.Eligible && r.Decision == "approved" {
			c.Approvals = append(c.Approvals, r.Reviewer)
		}
	}
	return rows.Err()
}

package worldbuilder

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Member struct {
	Principal string `json:"principal"`
	Role      string `json:"role"`
}

func (sc *scope) members(ctx context.Context, limit, offset int64) ([]Member, error) {
	if !sc.can("owner") {
		return nil, forbidden
	}
	rows, err := sc.tx.Query(ctx, `SELECT principal,role FROM worldbuilder.member WHERE world_id=$1 ORDER BY principal LIMIT $2 OFFSET $3`, sc.world.ID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Member{}
	for rows.Next() {
		var m Member
		if err = rows.Scan(&m.Principal, &m.Role); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

type AdminRequest struct {
	RequestID       string `json:"requestId"`
	ExpectedVersion int64  `json:"expectedVersion"`
	Action          string `json:"action"`
	Principal       string `json:"principal,omitempty"`
	Role            string `json:"role,omitempty"`
	Reason          string `json:"reason"`
}

func (sc *scope) administer(ctx context.Context, r AdminRequest) (World, error) {
	if _, err := uuid.Parse(r.RequestID); err != nil || strings.TrimSpace(r.Reason) == "" || len(r.Reason) > 2000 {
		return World{}, bad("administration requires UUID requestId and reason (max 2000 characters)")
	}
	r.Principal = strings.ToLower(strings.TrimSpace(r.Principal))
	request, _ := json.Marshal(r)
	var previous []byte
	var same bool
	err := sc.tx.QueryRow(ctx, `SELECT detail->'result', detail->'request'=$4::jsonb AND actor=$3 FROM worldbuilder.audit WHERE world_id=$1 AND resource=$2 AND action='world.administer'`, sc.world.ID, r.RequestID, sc.actor, request).Scan(&previous, &same)
	if err == nil {
		if !same {
			return World{}, conflict("administration request id reused")
		}
		var result World
		err = json.Unmarshal(previous, &result)
		return result, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return World{}, err
	}
	if !sc.can("owner") {
		return World{}, forbidden
	}
	if r.ExpectedVersion != sc.world.AdminVersion {
		return World{}, conflict("world administration changed; reload before continuing")
	}
	if r.Action != "set_member" && r.Role != "" {
		return World{}, bad("role is only valid for set_member")
	}
	switch r.Action {
	case "archive", "unarchive":
		if r.Principal != "" {
			return World{}, bad("archive does not accept a principal")
		}
		sc.world.Archived = r.Action == "archive"
	case "set_member":
		// member keeps the existing assignment route compatible and advances the
		// administration version. This transaction writes the same next version.
		if err = sc.member(ctx, r.Principal, r.Role); err != nil {
			return World{}, err
		}
	case "remove_member":
		if r.Principal == sc.actor || r.Principal == "" {
			return World{}, bad("cannot remove the owner")
		}
		if _, err = sc.tx.Exec(ctx, `DELETE FROM worldbuilder.member WHERE world_id=$1 AND principal=$2 AND role<>'owner'`, sc.world.ID, r.Principal); err != nil {
			return World{}, err
		}
	case "transfer_owner":
		if r.Principal == sc.actor {
			return World{}, bad("choose another current member")
		}
		var role string
		if err = sc.tx.QueryRow(ctx, `SELECT role FROM worldbuilder.member WHERE world_id=$1 AND principal=$2`, sc.world.ID, r.Principal).Scan(&role); errors.Is(err, pgx.ErrNoRows) {
			return World{}, bad("new owner must already be a member")
		} else if err != nil {
			return World{}, err
		}
		if _, err = sc.tx.Exec(ctx, `UPDATE worldbuilder.member SET role='admin' WHERE world_id=$1 AND principal=$2`, sc.world.ID, sc.actor); err != nil {
			return World{}, err
		}
		if _, err = sc.tx.Exec(ctx, `UPDATE worldbuilder.member SET role='owner' WHERE world_id=$1 AND principal=$2`, sc.world.ID, r.Principal); err != nil {
			return World{}, err
		}
		sc.world.Role = "admin"
	default:
		return World{}, bad("unsupported administration action")
	}
	sc.world.AdminVersion++
	if _, err = sc.tx.Exec(ctx, `UPDATE worldbuilder.world SET archived=$2,admin_version=$3 WHERE id=$1`, sc.world.ID, sc.world.Archived, sc.world.AdminVersion); err != nil {
		return World{}, err
	}
	err = sc.audit(ctx, "world.administer", r.RequestID, map[string]any{"request": r, "result": sc.world})
	return sc.world, err
}

func (sc *scope) validateIdentity(ctx context.Context, ops []Operation) error {
	ids := []string{}
	for _, op := range ops {
		if op.Record != nil {
			ids = append(ids, op.ID)
		}
	}
	rows, err := sc.tx.Query(ctx, `SELECT id,kind FROM worldbuilder.record_identity WHERE world_id=$1 AND id=ANY($2)`, sc.world.ID, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	known := map[string]string{}
	for rows.Next() {
		var id, kind string
		if err = rows.Scan(&id, &kind); err != nil {
			return err
		}
		known[id] = kind
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for _, op := range ops {
		if op.Record != nil && known[op.ID] != "" && known[op.ID] != op.Record.Kind {
			return bad("record kind cannot change after deletion: " + op.ID)
		}
	}
	return nil
}

func (sc *scope) validateDefaultTimeline(after Snapshot) error {
	timeline := sc.world.DefaultTimeline
	for _, record := range after {
		if record.Kind == "world_settings" {
			timeline = record.Timeline
		}
	}
	if after[timeline].Kind != "timeline" {
		return bad("world default timeline must remain valid; propose settings with a replacement timeline")
	}
	return nil
}

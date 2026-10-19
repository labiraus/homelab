package worldbuilder

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
)

func (s *Store) setAdministrator(ctx context.Context, actor, principal string, enabled bool, reason string) error {
	principal = strings.ToLower(strings.TrimSpace(principal))
	if strings.TrimSpace(reason) == "" || len(reason) > 2000 {
		return bad("a reason is required")
	}
	var allowed bool
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT FROM worldbuilder.superuser WHERE principal=$1)`, actor).Scan(&allowed); err != nil {
		return err
	}
	if !allowed {
		return forbidden
	}
	var registered bool
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT FROM auth.users WHERE email=$1)`, principal).Scan(&registered); err != nil {
		return err
	}
	if !registered {
		return bad("register this user first")
	}
	_, err := s.Pool.Exec(ctx, `SELECT worldbuilder.set_administrator($1,$2,$3,$4)`, actor, principal, enabled, reason)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "P0001" {
		return bad(pg.Message)
	}
	return err
}
func (s *Store) globalSearch(ctx context.Context, actor, query string) (any, error) {
	if strings.TrimSpace(query) == "" || len(query) > 4000 {
		return nil, bad("search text required (max 4000 bytes)")
	}
	worlds, err := s.Worlds(ctx, actor, 500, 0)
	if err != nil {
		return nil, err
	}
	results := []map[string]any{}
	for _, world := range worlds {
		value, err := s.inWorld(ctx, world.ID, actor, func(sc *scope) (any, error) {
			snap, err := sc.snapshot(ctx, sc.world.Revision)
			if err != nil {
				return nil, err
			}
			items := Records(snap, sc.world.DefaultTimeline, query, nil)
			if len(items) > 20 {
				items = items[:20]
			}
			return Citations(world.ID, sc.world.Revision, sc.world.DefaultTimeline, items), nil
		})
		if err == missing {
			continue
		}
		if err != nil {
			return nil, err
		}
		results = append(results, map[string]any{"world": world, "citations": value})
	}
	return map[string]any{"worlds": results, "searchedWorlds": len(worlds), "limited": len(worlds) == 500}, nil
}
func (s *Store) activity(ctx context.Context, actor string) (any, error) {
	rows, err := s.Pool.Query(ctx, `SELECT a.id,a.world_id::text,w.name,a.actor,a.action,a.resource,a.created_at::text FROM worldbuilder.audit a JOIN worldbuilder.world w ON w.id=a.world_id
 WHERE (EXISTS(SELECT FROM worldbuilder.member m WHERE m.world_id=a.world_id AND m.principal=$1) OR EXISTS(SELECT FROM worldbuilder.superuser WHERE principal=$1))
 AND a.action IN ('world.create','change.submit','change.publish') ORDER BY a.id DESC LIMIT 100`, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var world, name, who, action, resource, at string
		if err = rows.Scan(&id, &world, &name, &who, &action, &resource, &at); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"id": id, "worldId": world, "worldName": name, "actor": who, "action": action, "resource": resource, "createdAt": at})
	}
	return items, rows.Err()
}

// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
)

func (s *Store) CanonicalID(ctx context.Context, id string) (string, error) {
	var out string
	e := s.pool.QueryRow(ctx, `SELECT canonical_id::text FROM title_identity WHERE id::text=$1`, id).Scan(&out)
	if errors.Is(e, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return out, e
}

// ReconcileIdentities keeps observations and source-local IDs intact. Names alone
// never establish identity; every shared provider must agree across the cluster.
func (s *Store) ReconcileIdentities(ctx context.Context) error {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(726431210)`); e != nil {
		return e
	}
	type item struct {
		id, kind  string
		ids       map[string]string
		manual    bool
		canonical string
	}
	rows, e := tx.Query(ctx, `SELECT id::text,category,provider_ids,identity_manual,COALESCE(canonical_id::text,'') FROM library_titles ORDER BY created_at,id`)
	if e != nil {
		return e
	}
	items := []item{}
	byID := map[string]int{}
	for rows.Next() {
		var v item
		var raw []byte
		if e = rows.Scan(&v.id, &v.kind, &raw, &v.manual, &v.canonical); e != nil {
			rows.Close()
			return e
		}
		if e = json.Unmarshal(raw, &v.ids); e != nil {
			rows.Close()
			return e
		}
		byID[v.id] = len(items)
		items = append(items, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	parent := make([]int, len(items))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}
	compatible := func(a, b int) bool {
		for i := range items {
			if find(i) != a {
				continue
			}
			for j := range items {
				if find(j) != b {
					continue
				}
				for k, v := range items[i].ids {
					if other := items[j].ids[k]; v != "" && other != "" && v != other {
						return false
					}
				}
			}
		}
		return true
	}
	// Restore explicit links first. A manual standalone record is deliberately excluded.
	for i, v := range items {
		if v.manual && v.canonical != "" {
			if j, ok := byID[v.canonical]; ok {
				parent[find(i)] = find(j)
			}
		}
	}
	keys := map[string][]int{}
	for i, v := range items {
		if v.manual {
			continue
		}
		kind := "movie"
		if v.kind == "series" || v.kind == "anime_series" {
			kind = "series"
		}
		for k, id := range v.ids {
			if id == "" {
				continue
			}
			switch k {
			case "imdb", "tmdb", "tvdb", "tvmaze", "anidb":
			default:
				continue
			}
			key := kind + ":" + k + ":" + id
			for _, j := range keys[key] {
				a, b := find(i), find(j)
				if a != b && compatible(a, b) {
					if a > b {
						parent[a] = b
					} else {
						parent[b] = a
					}
				}
			}
			keys[key] = append(keys[key], i)
		}
	}
	for i, v := range items {
		target := ""
		if root := find(i); root != i {
			target = items[root].id
		}
		if _, e = tx.Exec(ctx, `UPDATE library_titles SET canonical_id=NULLIF($2,'')::uuid WHERE id::text=$1`, v.id, target); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}
func (s *Store) SetIdentity(ctx context.Context, id, target string) error {
	canonical, e := s.CanonicalID(ctx, id)
	if e != nil {
		return e
	}
	_ = canonical
	if target != "" {
		target, e = s.CanonicalID(ctx, target)
		if e != nil {
			return e
		}
		if target == id {
			return errors.New("choose another title")
		}
	}
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(726431210)`); e != nil {
		return e
	}
	if target != "" {
		var same bool
		e = tx.QueryRow(ctx, `SELECT (a.category IN('series','anime_series'))=(b.category IN('series','anime_series')) AND NOT EXISTS(SELECT 1 FROM jsonb_each_text(a.provider_ids) ai JOIN jsonb_each_text(b.provider_ids) bi ON ai.key=bi.key WHERE ai.value<>'' AND bi.value<>'' AND ai.value<>bi.value) FROM library_titles a,library_titles b WHERE a.id::text=$1 AND b.id::text=$2`, id, target).Scan(&same)
		if e != nil {
			return e
		}
		if !same {
			return ErrMetadataConflict
		}
	}
	// Detach children before moving a root: no chains or cycles in the projection.
	if _, e = tx.Exec(ctx, `UPDATE library_titles SET canonical_id=NULL,identity_manual=true WHERE canonical_id::text=$1`, id); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `UPDATE library_titles SET canonical_id=NULLIF($2,'')::uuid,identity_manual=true WHERE id::text=$1`, id, target); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

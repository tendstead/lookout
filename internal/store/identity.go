// SPDX-License-Identifier: Elastic-2.0
package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
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
		review    bool
	}
	rows, e := tx.Query(ctx, `SELECT id::text,category,provider_ids,identity_manual,COALESCE(canonical_id::text,''),match_status='review' FROM library_titles ORDER BY created_at,id`)
	if e != nil {
		return e
	}
	items := []item{}
	byID := map[string]int{}
	for rows.Next() {
		var v item
		var raw []byte
		if e = rows.Scan(&v.id, &v.kind, &raw, &v.manual, &v.canonical, &v.review); e != nil {
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
				a, b := find(i), find(j)
				if compatible(a, b) {
					parent[a] = b
				} else {
					items[i].review = true
					items[j].review = true
					if _, e = tx.Exec(ctx, `UPDATE library_titles SET match_status='review' WHERE id::text=ANY($1)`, []string{v.id, items[j].id}); e != nil {
						return e
					}
				}
			}
		}
	}
	keys := map[string][]int{}
	for i, v := range items {
		if v.manual || v.review {
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
	if e = syncCanonicalTracking(ctx, tx); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (s *Store) SetIdentity(ctx context.Context, id, target string) error {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(726431210)`); e != nil {
		return e
	}
	var source string
	if e = tx.QueryRow(ctx, `SELECT canonical_id::text FROM title_identity WHERE id::text=$1`, id).Scan(&source); errors.Is(e, pgx.ErrNoRows) {
		return ErrNotFound
	} else if e != nil {
		return e
	}
	if target != "" {
		var canonical string
		if e = tx.QueryRow(ctx, `SELECT canonical_id::text FROM title_identity WHERE id::text=$1`, target).Scan(&canonical); errors.Is(e, pgx.ErrNoRows) {
			return ErrNotFound
		} else if e != nil {
			return e
		}
		target = canonical
		if target == source {
			return nil
		}
		var conflict bool
		e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM library_titles a JOIN title_identity ai ON ai.id=a.id CROSS JOIN library_titles b JOIN title_identity bi ON bi.id=b.id WHERE ai.canonical_id::text=$1 AND bi.canonical_id::text=$2 AND ((a.category IN('series','anime_series'))<>(b.category IN('series','anime_series')) OR a.match_status='review' OR b.match_status='review' OR EXISTS(SELECT 1 FROM jsonb_each_text(a.provider_ids) ap JOIN jsonb_each_text(b.provider_ids) bp ON ap.key=bp.key WHERE ap.value<>'' AND bp.value<>'' AND ap.value<>bp.value)))`, source, target).Scan(&conflict)
		if e != nil {
			return e
		}
		if conflict {
			return ErrMetadataConflict
		}
		if _, e = tx.Exec(ctx, `UPDATE library_titles SET canonical_id=$2::uuid,identity_manual=true WHERE id IN(SELECT id FROM title_identity WHERE canonical_id::text=$1)`, source, target); e != nil {
			return e
		}
	} else {
		// Splitting the representative keeps the other copies in their own group.
		if source == id {
			var replacement *string
			if e = tx.QueryRow(ctx, `SELECT (SELECT id::text FROM library_titles WHERE canonical_id::text=$1 ORDER BY created_at,id LIMIT 1)`, id).Scan(&replacement); e != nil {
				return e
			}
			if replacement != nil {
				if _, e = tx.Exec(ctx, `UPDATE library_titles SET canonical_id=CASE WHEN id::text=$2 THEN NULL ELSE $2::uuid END,identity_manual=true WHERE canonical_id::text=$1`, id, *replacement); e != nil {
					return e
				}
			}
		}
		if _, e = tx.Exec(ctx, `UPDATE library_titles SET canonical_id=NULL,identity_manual=true WHERE id::text=$1`, id); e != nil {
			return e
		}
	}
	if e = syncCanonicalTracking(ctx, tx); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// A new representative inherits compatible identity, artwork and effective follow
// state. Original source records and event/outbox rows remain available for splits.
func syncCanonicalTracking(ctx context.Context, tx pgx.Tx) error {
	rows, e := tx.Query(ctx, `SELECT i.canonical_id::text FROM title_identity i GROUP BY i.canonical_id HAVING count(*)>1`)
	if e != nil {
		return e
	}
	roots := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		roots = append(roots, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, root := range roots {
		if _, e = tx.Exec(ctx, `UPDATE library_titles SET provider_ids=provider_ids||COALESCE((SELECT jsonb_object_agg(p.key,p.value) FROM library_titles mt JOIN title_identity mi ON mi.id=mt.id CROSS JOIN LATERAL jsonb_each(mt.provider_ids) p WHERE mi.canonical_id::text=$1 AND p.value<>'""'::jsonb),'{}'::jsonb) WHERE id::text=$1`, root); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `UPDATE library_titles target SET artwork_connection=donor.artwork_connection,artwork_path=donor.artwork_path,artwork_url=donor.artwork_url FROM (SELECT mt.artwork_connection,mt.artwork_path,mt.artwork_url FROM library_titles mt JOIN title_identity mi ON mi.id=mt.id WHERE mi.canonical_id::text=$1 AND (mt.artwork_path<>'' OR mt.artwork_url<>'') ORDER BY mt.created_at,mt.id LIMIT 1) donor WHERE target.id::text=$1 AND target.artwork_path='' AND target.artwork_url=''`, root); e != nil {
			return e
		}
		var chosen, provider string
		var enabled, initialized bool
		var baseline, next time.Time
		var checked *time.Time
		var lastError string
		e = tx.QueryRow(ctx, `SELECT f.title_id::text,f.provider_id,(SELECT bool_or(af.enabled) FROM release_follows af JOIN title_identity ai ON ai.id=af.title_id WHERE ai.canonical_id::text=$1),f.initialized,(SELECT min(af.baseline_at) FROM release_follows af JOIN title_identity ai ON ai.id=af.title_id WHERE ai.canonical_id::text=$1 AND af.provider_id=f.provider_id),f.next_refresh_at,f.checked_at,f.last_error FROM release_follows f JOIN title_identity i ON i.id=f.title_id JOIN library_titles root ON root.id=i.canonical_id WHERE i.canonical_id::text=$1 AND f.provider_id=COALESCE(root.provider_ids->>'tvmaze','') ORDER BY f.initialized DESC,f.checked_at DESC NULLS LAST,(f.title_id::text=$1) DESC LIMIT 1`, root).Scan(&chosen, &provider, &enabled, &initialized, &baseline, &next, &checked, &lastError)
		if errors.Is(e, pgx.ErrNoRows) {
			continue
		}
		if e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO release_follows(title_id,enabled,provider_id,initialized,baseline_at,next_refresh_at,checked_at,last_error) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(title_id) DO UPDATE SET enabled=excluded.enabled,provider_id=excluded.provider_id,initialized=excluded.initialized,baseline_at=excluded.baseline_at,next_refresh_at=excluded.next_refresh_at,checked_at=excluded.checked_at,last_error=excluded.last_error`, root, enabled, provider, initialized, baseline, next, checked, lastError); e != nil {
			return e
		}
		if chosen != root {
			if _, e = tx.Exec(ctx, `UPDATE provider_episodes SET active=false WHERE title_id::text=$1 AND show_id=$2`, root, provider); e != nil {
				return e
			}
			if _, e = tx.Exec(ctx, `INSERT INTO provider_episodes(title_id,show_id,episode_id,name,season,number,air_date,air_stamp,scheduled_at,precision,provider_url,active) SELECT $1::uuid,show_id,episode_id,name,season,number,air_date,air_stamp,scheduled_at,precision,provider_url,active FROM provider_episodes WHERE title_id::text=$2 AND show_id=$3 ON CONFLICT(title_id,show_id,episode_id) DO UPDATE SET name=excluded.name,season=excluded.season,number=excluded.number,air_date=excluded.air_date,air_stamp=excluded.air_stamp,scheduled_at=excluded.scheduled_at,precision=excluded.precision,provider_url=excluded.provider_url,active=excluded.active`, root, chosen, provider); e != nil {
				return e
			}
		}
	}
	return nil
}

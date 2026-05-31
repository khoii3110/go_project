package asset

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrAssetNotFound = errors.New("asset not found")

type Repository interface {
	UpsertMetadata(ctx context.Context, assetID string, metadata json.RawMessage) (*Asset, error)
	GetMetadata(ctx context.Context, assetID string) (*Asset, error)
	SetACL(ctx context.Context, assetID string, acl []string) (*Asset, error)
	GetACL(ctx context.Context, assetID string) (*Asset, error)
}

type PGRepository struct { db *pgxpool.Pool }

func NewPGRepository(db *pgxpool.Pool) *PGRepository { return &PGRepository{db: db} }

func (r *PGRepository) UpsertMetadata(ctx context.Context, assetID string, metadata json.RawMessage) (*Asset, error) {
	const query = `
		INSERT INTO assets (asset_id, metadata)
		VALUES ($1, $2)
		ON CONFLICT (asset_id)
		DO UPDATE SET metadata = EXCLUDED.metadata, updated_at = NOW()
		RETURNING asset_id, metadata
	`
	var a Asset
	if err := r.db.QueryRow(ctx, query, assetID, metadata).Scan(&a.AssetID, &a.Metadata); err != nil { return nil, err }
	return &a, nil
}

func (r *PGRepository) GetMetadata(ctx context.Context, assetID string) (*Asset, error) {
	const query = `SELECT asset_id, metadata FROM assets WHERE asset_id = $1`
	var a Asset
	if err := r.db.QueryRow(ctx, query, assetID).Scan(&a.AssetID, &a.Metadata); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return nil, ErrAssetNotFound }
		return nil, err
	}
	return &a, nil
}

func (r *PGRepository) SetACL(ctx context.Context, assetID string, acl []string) (*Asset, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil { return nil, err }
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `INSERT INTO assets (asset_id) VALUES ($1) ON CONFLICT (asset_id) DO NOTHING`, assetID); err != nil { return nil, err }
	if _, err := tx.Exec(ctx, `DELETE FROM asset_acls WHERE asset_id = $1`, assetID); err != nil { return nil, err }
	for _, userID := range acl {
		if _, err := tx.Exec(ctx, `INSERT INTO asset_acls (asset_id, user_id) VALUES ($1, $2)`, assetID, userID); err != nil { return nil, err }
	}
	if err := tx.Commit(ctx); err != nil { return nil, err }
	return r.GetACL(ctx, assetID)
}

func (r *PGRepository) GetACL(ctx context.Context, assetID string) (*Asset, error) {
	const query = `SELECT asset_id FROM assets WHERE asset_id = $1`
	var a Asset
	if err := r.db.QueryRow(ctx, query, assetID).Scan(&a.AssetID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return nil, ErrAssetNotFound }
		return nil, err
	}
	rows, err := r.db.Query(ctx, `SELECT user_id::text FROM asset_acls WHERE asset_id = $1 ORDER BY created_at`, assetID)
	if err != nil { return nil, err }
	defer rows.Close()
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil { return nil, err }
		a.ACL = append(a.ACL, userID)
	}
	if err := rows.Err(); err != nil { return nil, err }
	return &a, nil
}

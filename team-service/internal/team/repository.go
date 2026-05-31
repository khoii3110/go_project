package team

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrTeamNotFound      = errors.New("team not found")
	ErrMemberAlreadySet  = errors.New("member already exists")
	ErrManagerAlreadySet = errors.New("manager already exists")
	ErrMemberNotFound    = errors.New("member not found")
)

type Repository interface {
	CreateTeam(ctx context.Context, teamName, creatorID string) (*Team, error)
	GetTeamByID(ctx context.Context, teamID string) (*Team, error)
	ListMembers(ctx context.Context, teamID string) ([]string, error)
	AddMember(ctx context.Context, teamID, userID string) (*Team, error)
	RemoveMember(ctx context.Context, teamID, userID string) (*Team, error)
	AddManager(ctx context.Context, teamID, userID string) (*Team, error)
}

type PGRepository struct {
	db *pgxpool.Pool
}

func NewPGRepository(db *pgxpool.Pool) *PGRepository {
	return &PGRepository{db: db}
}

func (r *PGRepository) CreateTeam(ctx context.Context, teamName, creatorID string) (*Team, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const teamQuery = `
		INSERT INTO teams (team_name, creator_id)
		VALUES ($1, $2)
		RETURNING team_id, team_name, creator_id, created_at, updated_at
	`

	var team Team
	if err := tx.QueryRow(ctx, teamQuery, teamName, creatorID).Scan(
		&team.TeamID,
		&team.TeamName,
		&team.CreatorID,
		&team.CreatedAt,
		&team.UpdatedAt,
	); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `INSERT INTO team_managers (team_id, user_id) VALUES ($1, $2)`, team.TeamID, creatorID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	team.Managers = []string{creatorID}
	team.Members = []string{}
	return &team, nil
}

func (r *PGRepository) GetTeamByID(ctx context.Context, teamID string) (*Team, error) {
	const teamQuery = `
		SELECT team_id, team_name, creator_id, created_at, updated_at
		FROM teams
		WHERE team_id = $1
	`

	var team Team
	if err := r.db.QueryRow(ctx, teamQuery, teamID).Scan(
		&team.TeamID,
		&team.TeamName,
		&team.CreatorID,
		&team.CreatedAt,
		&team.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTeamNotFound
		}
		return nil, err
	}

	if err := r.db.QueryRow(ctx, `SELECT COALESCE(array_agg(user_id::text ORDER BY created_at), ARRAY[]::text[]) FROM team_managers WHERE team_id = $1`, teamID).Scan(&team.Managers); err != nil {
		return nil, err
	}
	if err := r.db.QueryRow(ctx, `SELECT COALESCE(array_agg(user_id::text ORDER BY created_at), ARRAY[]::text[]) FROM team_members WHERE team_id = $1`, teamID).Scan(&team.Members); err != nil {
		return nil, err
	}

	return &team, nil
}

func (r *PGRepository) ListMembers(ctx context.Context, teamID string) ([]string, error) {
	const query = `
		SELECT COALESCE(array_agg(user_id::text ORDER BY created_at), ARRAY[]::text[])
		FROM team_members
		WHERE team_id = $1
	`

	var members []string
	if err := r.db.QueryRow(ctx, query, teamID).Scan(&members); err != nil {
		return nil, err
	}
	return members, nil
}

func (r *PGRepository) AddMember(ctx context.Context, teamID, userID string) (*Team, error) {
	const query = `
		INSERT INTO team_members (team_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`

	result, err := r.db.Exec(ctx, query, teamID, userID)
	if err != nil {
		return nil, err
	}
	if result.RowsAffected() == 0 {
		return nil, ErrMemberAlreadySet
	}

	return r.GetTeamByID(ctx, teamID)
}

func (r *PGRepository) RemoveMember(ctx context.Context, teamID, userID string) (*Team, error) {
	result, err := r.db.Exec(ctx, `DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`, teamID, userID)
	if err != nil {
		return nil, err
	}
	if result.RowsAffected() == 0 {
		return nil, ErrMemberNotFound
	}

	return r.GetTeamByID(ctx, teamID)
}

func (r *PGRepository) AddManager(ctx context.Context, teamID, userID string) (*Team, error) {
	const query = `
		INSERT INTO team_managers (team_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`

	result, err := r.db.Exec(ctx, query, teamID, userID)
	if err != nil {
		return nil, err
	}
	if result.RowsAffected() == 0 {
		return nil, ErrManagerAlreadySet
	}

	return r.GetTeamByID(ctx, teamID)
}

func mapPgError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return ErrTeamNotFound
	}
	return err
}

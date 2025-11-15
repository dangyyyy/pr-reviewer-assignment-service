package schema

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

var statements = []string{
	`CREATE TABLE IF NOT EXISTS teams (
        team_name TEXT PRIMARY KEY
    )`,
	`CREATE TABLE IF NOT EXISTS users (
        user_id TEXT PRIMARY KEY,
        username TEXT NOT NULL,
        team_name TEXT NOT NULL REFERENCES teams(team_name) ON DELETE RESTRICT,
        is_active BOOLEAN NOT NULL DEFAULT TRUE
    )`,
	`CREATE TABLE IF NOT EXISTS pull_requests (
        pull_request_id TEXT PRIMARY KEY,
        pull_request_name TEXT NOT NULL,
        author_id TEXT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
        status TEXT NOT NULL CHECK (status IN ('OPEN', 'MERGED')),
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        merged_at TIMESTAMPTZ NULL
    )`,
	`CREATE TABLE IF NOT EXISTS pull_request_reviewers (
        pull_request_id TEXT NOT NULL REFERENCES pull_requests(pull_request_id) ON DELETE CASCADE,
        reviewer_id TEXT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
        PRIMARY KEY (pull_request_id, reviewer_id)
    )`,
	`CREATE INDEX IF NOT EXISTS idx_users_team ON users(team_name)`,
	`CREATE INDEX IF NOT EXISTS idx_pull_requests_author ON pull_requests(author_id)`,
	`CREATE INDEX IF NOT EXISTS idx_reviewers_user ON pull_request_reviewers(reviewer_id)`,
}

func Ensure(ctx context.Context, pool *pgxpool.Pool) error {
	for _, stmt := range statements {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

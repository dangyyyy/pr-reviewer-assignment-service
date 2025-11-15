package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dangy/pr-reviewer-assignment-service/internal/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

type querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Close() {
	r.pool.Close()
}

func (r *Repository) withTx(ctx context.Context, fn func(pgx.Tx) error) (err error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	if err = fn(tx); err != nil {
		return err
	}

	if commitErr := tx.Commit(ctx); commitErr != nil {
		return commitErr
	}

	return nil
}

func (r *Repository) CreateTeam(ctx context.Context, team domain.Team) (domain.Team, error) {
	var out domain.Team

	err := r.withTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO teams (team_name) VALUES ($1)`, team.Name)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return domain.ErrTeamExists
			}
			return err
		}

		for _, member := range team.Members {
			_, err = tx.Exec(ctx, `
                INSERT INTO users (user_id, username, team_name, is_active)
                VALUES ($1, $2, $3, $4)
                ON CONFLICT (user_id) DO UPDATE
                SET username = EXCLUDED.username,
                    team_name = EXCLUDED.team_name,
                    is_active = EXCLUDED.is_active
            `, member.ID, member.Username, team.Name, member.IsActive)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return out, err
	}

	return r.GetTeam(ctx, team.Name)
}

func (r *Repository) GetTeam(ctx context.Context, teamName string) (domain.Team, error) {
	var team domain.Team
	team.Name = teamName

	var exists string
	if err := r.pool.QueryRow(ctx, `SELECT team_name FROM teams WHERE team_name = $1`, teamName).Scan(&exists); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return team, domain.ErrTeamNotFound
		}
		return team, err
	}

	rows, err := r.pool.Query(ctx, `
        SELECT user_id, username, is_active
        FROM users
        WHERE team_name = $1
        ORDER BY username ASC
    `, teamName)
	if err != nil {
		return team, err
	}
	defer rows.Close()

	for rows.Next() {
		var member domain.User
		member.TeamName = teamName
		if err := rows.Scan(&member.ID, &member.Username, &member.IsActive); err != nil {
			return team, err
		}
		team.Members = append(team.Members, member)
	}

	if err := rows.Err(); err != nil {
		return team, err
	}

	return team, nil
}

func (r *Repository) SetUserActivity(ctx context.Context, userID string, isActive bool) (domain.User, error) {
	var user domain.User

	err := r.pool.QueryRow(ctx, `
        UPDATE users
        SET is_active = $2
        WHERE user_id = $1
        RETURNING user_id, username, team_name, is_active
    `, userID, isActive).
		Scan(&user.ID, &user.Username, &user.TeamName, &user.IsActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return user, domain.ErrUserNotFound
		}
		return user, err
	}

	return user, nil
}

func (r *Repository) CreatePullRequest(ctx context.Context, id, name, authorID string) (domain.PullRequest, error) {
	var pr domain.PullRequest
	author, err := r.getUser(ctx, r.pool, authorID)
	if err != nil {
		return pr, err
	}
	if author.TeamName == "" {
		return pr, domain.ErrTeamNotFound
	}
	reviewerRows, err := r.pool.Query(ctx, `
        SELECT user_id
        FROM users
        WHERE team_name = $1 AND is_active = TRUE AND user_id <> $2
        ORDER BY random()
        LIMIT 2
    `, author.TeamName, authorID)
	if err != nil {
		return pr, err
	}
	defer reviewerRows.Close()

	var reviewerIDs []string
	for reviewerRows.Next() {
		var reviewerID string
		if err := reviewerRows.Scan(&reviewerID); err != nil {
			return pr, err
		}
		reviewerIDs = append(reviewerIDs, reviewerID)
	}
	if err := reviewerRows.Err(); err != nil {
		return pr, err
	}
	now := time.Now().UTC()
	err = r.withTx(ctx, func(tx pgx.Tx) error {
		_, err = tx.Exec(ctx, `
            INSERT INTO pull_requests (pull_request_id, pull_request_name, author_id, status, created_at)
            VALUES ($1, $2, $3, $4, $5)
        `, id, name, authorID, domain.PullRequestStatusOpen, now)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return domain.ErrPRExists
			}
			return err
		}

		for _, reviewerID := range reviewerIDs {
			_, err = tx.Exec(ctx, `
                INSERT INTO pull_request_reviewers (pull_request_id, reviewer_id)
                VALUES ($1, $2)
            `, id, reviewerID)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return pr, err
	}
	pr.ID = id
	pr.Name = name
	pr.AuthorID = authorID
	pr.Status = domain.PullRequestStatusOpen
	pr.CreatedAt = now
	pr.AssignedReviewers = reviewerIDs

	return pr, nil
}

func (r *Repository) GetPullRequest(ctx context.Context, prID string) (domain.PullRequest, error) {
	return r.loadPullRequest(ctx, r.pool, prID)
}

func (r *Repository) MergePullRequest(ctx context.Context, prID string) (domain.PullRequest, error) {
	var result domain.PullRequest

	err := r.withTx(ctx, func(tx pgx.Tx) error {
		current, err := r.loadPullRequest(ctx, tx, prID)
		if err != nil {
			return err
		}

		if current.Status == domain.PullRequestStatusMerged {
			result = current
			return nil
		}

		now := time.Now().UTC()
		_, err = tx.Exec(ctx, `
            UPDATE pull_requests
            SET status = $2, merged_at = $3
            WHERE pull_request_id = $1
        `, prID, domain.PullRequestStatusMerged, now)
		if err != nil {
			return err
		}

		current.Status = domain.PullRequestStatusMerged
		current.MergedAt = &now
		result = current

		return nil
	})

	if err != nil {
		return result, err
	}

	return result, nil
}

func (r *Repository) ReassignReviewer(ctx context.Context, prID, oldReviewerID string) (domain.PullRequest, string, error) {
	var updated domain.PullRequest
	var replacement string

	pr, err := r.loadPullRequest(ctx, r.pool, prID)
	if err != nil {
		return updated, "", err
	}

	if pr.Status == domain.PullRequestStatusMerged {
		return updated, "", domain.ErrPRMerged
	}

	assigned := false
	assignedSet := make(map[string]struct{}, len(pr.AssignedReviewers))
	for _, id := range pr.AssignedReviewers {
		assignedSet[id] = struct{}{}
		if id == oldReviewerID {
			assigned = true
		}
	}

	if !assigned {
		return updated, "", domain.ErrNotAssigned
	}

	reviewer, err := r.getUser(ctx, r.pool, oldReviewerID)
	if err != nil {
		return updated, "", err
	}

	if reviewer.TeamName == "" {
		return updated, "", domain.ErrTeamNotFound
	}

	candidates, err := r.pool.Query(ctx, `
        SELECT user_id
        FROM users
        WHERE team_name = $1 
          AND is_active = TRUE 
          AND user_id <> $2 
          AND user_id <> $3
        ORDER BY random()
    `, reviewer.TeamName, oldReviewerID, pr.AuthorID)
	if err != nil {
		return updated, "", err
	}
	defer candidates.Close()

	for candidates.Next() {
		var candidate string
		if err := candidates.Scan(&candidate); err != nil {
			return updated, "", err
		}
		if _, exists := assignedSet[candidate]; exists {
			continue
		}
		replacement = candidate
		break
	}

	if err := candidates.Err(); err != nil {
		return updated, "", err
	}

	if replacement == "" {
		return updated, "", domain.ErrNoCandidate
	}

	err = r.withTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			"DELETE FROM pull_request_reviewers WHERE pull_request_id = $1 AND reviewer_id = $2",
			prID, oldReviewerID)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx,
			"INSERT INTO pull_request_reviewers (pull_request_id, reviewer_id) VALUES ($1, $2)",
			prID, replacement)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return updated, "", err
	}

	updated, err = r.loadPullRequest(ctx, r.pool, prID)
	if err != nil {
		return updated, "", err
	}

	return updated, replacement, nil
}

func (r *Repository) ListReviewerPullRequests(ctx context.Context, userID string) ([]domain.PullRequestShort, error) {
	rows, err := r.pool.Query(ctx, `
        SELECT pr.pull_request_id, pr.pull_request_name, pr.author_id, pr.status
        FROM pull_requests pr
        JOIN pull_request_reviewers prr ON pr.pull_request_id = prr.pull_request_id
        WHERE prr.reviewer_id = $1
        ORDER BY pr.created_at DESC
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.PullRequestShort
	for rows.Next() {
		var item domain.PullRequestShort
		if err := rows.Scan(&item.ID, &item.Name, &item.AuthorID, &item.Status); err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *Repository) getUser(ctx context.Context, q querier, userID string) (domain.User, error) {
	var user domain.User
	err := q.QueryRow(ctx, `
        SELECT user_id, username, team_name, is_active
        FROM users
        WHERE user_id = $1
    `, userID).Scan(&user.ID, &user.Username, &user.TeamName, &user.IsActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return user, domain.ErrUserNotFound
		}
		return user, err
	}
	return user, nil
}

func (r *Repository) loadPullRequest(ctx context.Context, q querier, prID string) (domain.PullRequest, error) {
	var pr domain.PullRequest
	var mergedAt *time.Time

	err := q.QueryRow(ctx, `
        SELECT pull_request_id, pull_request_name, author_id, status, created_at, merged_at
        FROM pull_requests
        WHERE pull_request_id = $1
    `, prID).Scan(&pr.ID, &pr.Name, &pr.AuthorID, &pr.Status, &pr.CreatedAt, &mergedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pr, domain.ErrPRNotFound
		}
		return pr, err
	}
	pr.MergedAt = mergedAt

	rows, err := q.Query(ctx, `
        SELECT reviewer_id
        FROM pull_request_reviewers
        WHERE pull_request_id = $1
        ORDER BY reviewer_id
    `, prID)
	if err != nil {
		return pr, err
	}
	defer rows.Close()

	for rows.Next() {
		var reviewerID string
		if err := rows.Scan(&reviewerID); err != nil {
			return pr, err
		}
		pr.AssignedReviewers = append(pr.AssignedReviewers, reviewerID)
	}

	if err := rows.Err(); err != nil {
		return pr, err
	}

	return pr, nil
}

type ReviewerStats struct {
	UserID           string
	Username         string
	TotalAssignments int
}

type PRStats struct {
	TotalPRs            int
	OpenPRs             int
	MergedPRs           int
	PRsWithReviewers    int
	PRsWithoutReviewers int
}

func (r *Repository) GetReviewerStats(ctx context.Context) ([]ReviewerStats, error) {
	rows, err := r.pool.Query(ctx, `
        SELECT 
            u.user_id,
            u.username,
            COUNT(prr.reviewer_id) as total_assignments
        FROM users u
        LEFT JOIN pull_request_reviewers prr ON u.user_id = prr.reviewer_id
        GROUP BY u.user_id, u.username
        ORDER BY total_assignments DESC, u.username ASC
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []ReviewerStats
	for rows.Next() {
		var s ReviewerStats
		if err := rows.Scan(&s.UserID, &s.Username, &s.TotalAssignments); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}

func (r *Repository) GetPRStats(ctx context.Context) (PRStats, error) {
	var stats PRStats

	err := r.pool.QueryRow(ctx, `
        SELECT 
            COUNT(*) as total,
            COUNT(*) FILTER (WHERE status = 'OPEN') as open,
            COUNT(*) FILTER (WHERE status = 'MERGED') as merged,
            COUNT(*) FILTER (WHERE EXISTS (
                SELECT 1 FROM pull_request_reviewers prr 
                WHERE prr.pull_request_id = pr.pull_request_id
            )) as with_reviewers,
            COUNT(*) FILTER (WHERE NOT EXISTS (
                SELECT 1 FROM pull_request_reviewers prr 
                WHERE prr.pull_request_id = pr.pull_request_id
            )) as without_reviewers
        FROM pull_requests pr
    `).Scan(
		&stats.TotalPRs,
		&stats.OpenPRs,
		&stats.MergedPRs,
		&stats.PRsWithReviewers,
		&stats.PRsWithoutReviewers,
	)
	if err != nil {
		return stats, err
	}

	return stats, nil
}

package service

import (
	"context"

	"github.com/dangy/pr-reviewer-assignment-service/internal/domain"
	"github.com/dangy/pr-reviewer-assignment-service/internal/repository"
)

type Service interface {
	CreateTeam(ctx context.Context, team domain.Team) (domain.Team, error)
	GetTeam(ctx context.Context, teamName string) (domain.Team, error)
	SetUserActivity(ctx context.Context, userID string, isActive bool) (domain.User, error)
	CreatePullRequest(ctx context.Context, id, name, authorID string) (domain.PullRequest, error)
	GetPullRequest(ctx context.Context, id string) (domain.PullRequest, error)
	MergePullRequest(ctx context.Context, id string) (domain.PullRequest, error)
	ReassignReviewer(ctx context.Context, prID, oldReviewerID string) (domain.PullRequest, string, error)
	ListReviewerPullRequests(ctx context.Context, userID string) ([]domain.PullRequestShort, error)
	GetReviewerStats(ctx context.Context) ([]repository.ReviewerStats, error)
	GetPRStats(ctx context.Context) (repository.PRStats, error)
}

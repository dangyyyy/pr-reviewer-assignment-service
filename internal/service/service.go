package service

import (
	"context"

	"github.com/dangy/pr-reviewer-assignment-service/internal/domain"
	"github.com/dangy/pr-reviewer-assignment-service/internal/repository"
)

type service struct {
	repo *repository.Repository
}

func New(repo *repository.Repository) Service {
	return &service{repo: repo}
}

func (s *service) CreateTeam(ctx context.Context, team domain.Team) (domain.Team, error) {
	return s.repo.CreateTeam(ctx, team)
}

func (s *service) GetTeam(ctx context.Context, teamName string) (domain.Team, error) {
	return s.repo.GetTeam(ctx, teamName)
}

func (s *service) SetUserActivity(ctx context.Context, userID string, isActive bool) (domain.User, error) {
	return s.repo.SetUserActivity(ctx, userID, isActive)
}

func (s *service) CreatePullRequest(ctx context.Context, id, name, authorID string) (domain.PullRequest, error) {
	return s.repo.CreatePullRequest(ctx, id, name, authorID)
}

func (s *service) GetPullRequest(ctx context.Context, id string) (domain.PullRequest, error) {
	return s.repo.GetPullRequest(ctx, id)
}

func (s *service) MergePullRequest(ctx context.Context, id string) (domain.PullRequest, error) {
	return s.repo.MergePullRequest(ctx, id)
}

func (s *service) ReassignReviewer(ctx context.Context, prID, oldReviewerID string) (domain.PullRequest, string, error) {
	return s.repo.ReassignReviewer(ctx, prID, oldReviewerID)
}

func (s *service) ListReviewerPullRequests(ctx context.Context, userID string) ([]domain.PullRequestShort, error) {
	return s.repo.ListReviewerPullRequests(ctx, userID)
}

func (s *service) GetReviewerStats(ctx context.Context) ([]repository.ReviewerStats, error) {
	return s.repo.GetReviewerStats(ctx)
}

func (s *service) GetPRStats(ctx context.Context) (repository.PRStats, error) {
	return s.repo.GetPRStats(ctx)
}

package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

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
	if len(team.Members) == 0 {
		log.Printf("[Service] CreateTeam: validation error - team %q has no members\"", team.Name)
		return domain.Team{}, errors.New("team must have at least one member")
	}
	if strings.TrimSpace(team.Name) == "" {
		log.Printf("[Service] CreateTeam: validation error - team name is required")
		return domain.Team{}, errors.New("team name is required")
	}
	created, err := s.repo.CreateTeam(ctx, team)
	if err != nil {
		log.Printf("[Service] CreateTeam: failed to create team %q: %v", team.Name, err)
		return domain.Team{}, fmt.Errorf("failed to create team: %w", err)
	}
	log.Printf("[Service] CreateTeam: successfully created team %q with %d members", created.Name, len(created.Members))
	return created, nil
}

func (s *service) GetTeam(ctx context.Context, teamName string) (domain.Team, error) {
	if strings.TrimSpace(teamName) == "" {
		log.Printf("[Service] GetTeam: validation error - team name is required")
		return domain.Team{}, errors.New("team name is required")
	}
	team, err := s.repo.GetTeam(ctx, teamName)
	if err != nil {
		log.Printf("[Service] GetTeam: failed to get team %q: %v", teamName, err)
		return domain.Team{}, fmt.Errorf("failed to get team: %w", err)
	}
	log.Printf("[Service] GetTeam: successfully retrieved team %q with %d members", team.Name, len(team.Members))
	return team, nil
}

func (s *service) SetUserActivity(ctx context.Context, userID string, isActive bool) (domain.User, error) {
	if strings.TrimSpace(userID) == "" {
		log.Printf("[Service] SetUserActivity: validation error - user ID is required")
		return domain.User{}, errors.New("user ID is required")
	}
	user, err := s.repo.SetUserActivity(ctx, userID, isActive)
	if err != nil {
		log.Printf("[Service] SetUserActivity: failed to set user %q activity: %v", userID, err)
		return domain.User{}, fmt.Errorf("failed to set user activity: %w", err)
	}
	log.Printf("[Service] SetUserActivity: successfully set user %q activity to %v", user.Username, isActive)
	return user, nil
}

func (s *service) CreatePullRequest(ctx context.Context, id, name, authorID string) (domain.PullRequest, error) {
	if strings.TrimSpace(id) == "" {
		return domain.PullRequest{}, errors.New("pull request ID is required")
	}
	if strings.TrimSpace(name) == "" {
		return domain.PullRequest{}, errors.New("pull request name is required")
	}
	if strings.TrimSpace(authorID) == "" {
		return domain.PullRequest{}, errors.New("author ID is required")
	}
	pr, err := s.repo.CreatePullRequest(ctx, id, name, authorID)
	if err != nil {
		log.Printf("[Service] CreatePullRequest: error creating PR %q by author %q: %v", id, authorID, err)
		return domain.PullRequest{}, fmt.Errorf("failed to create pull request: %w", err)
	}

	log.Printf("[Service] CreatePullRequest: created PR %q with %d reviewers", pr.ID, len(pr.AssignedReviewers))
	return pr, nil
}

func (s *service) GetPullRequest(ctx context.Context, id string) (domain.PullRequest, error) {
	if strings.TrimSpace(id) == "" {
		return domain.PullRequest{}, errors.New("pull request ID is required")
	}
	pr, err := s.repo.GetPullRequest(ctx, id)
	if err != nil {
		log.Printf("[Service] GetPullRequest: error getting PR %q: %v", id, err)
		return domain.PullRequest{}, fmt.Errorf("failed to get pull request: %w", err)
	}
	return pr, nil
}

func (s *service) MergePullRequest(ctx context.Context, id string) (domain.PullRequest, error) {
	if strings.TrimSpace(id) == "" {
		return domain.PullRequest{}, errors.New("pull request ID is required")
	}
	pr, err := s.repo.MergePullRequest(ctx, id)
	if err != nil {
		log.Printf("[Service] MergePullRequest: error merging PR %q: %v", id, err)
		return domain.PullRequest{}, fmt.Errorf("failed to merge pull request: %w", err)
	}
	if pr.Status == domain.PullRequestStatusMerged {
		log.Printf("[Service] MergePullRequest: PR %q already merged, returning current state", id)
		return pr, nil
	}
	mergedPR, err := s.repo.MergePullRequest(ctx, id)
	if err != nil {
		log.Printf("[Service] MergePullRequest: error merging PR %q: %v", id, err)
		return domain.PullRequest{}, fmt.Errorf("failed to merge pull request: %w", err)
	}
	log.Printf("[Service] MergePullRequest: successfully merged PR %q", id)
	return mergedPR, nil
}

func (s *service) ReassignReviewer(ctx context.Context, prID, oldReviewerID string) (domain.PullRequest, string, error) {
	if strings.TrimSpace(prID) == "" {
		return domain.PullRequest{}, "", errors.New("pull request ID is required")
	}
	if strings.TrimSpace(oldReviewerID) == "" {
		return domain.PullRequest{}, "", errors.New("old reviewer ID is required")
	}
	pr, err := s.repo.GetPullRequest(ctx, prID)
	if err != nil {
		log.Printf("[Service] ReassignReviewer: error fetching PR %q: %v", prID, err)
		return domain.PullRequest{}, "", fmt.Errorf("failed to get pull request: %w", err)
	}
	if pr.Status == domain.PullRequestStatusMerged {
		log.Printf("[Service] ReassignReviewer: cannot reassign on merged PR %q", prID)
		return pr, "", domain.ErrPRMerged
	}
	updatedPR, replacement, err := s.repo.ReassignReviewer(ctx, prID, oldReviewerID)
	if err != nil {
		log.Printf("[Service] ReassignReviewer: error reassigning reviewer in PR %q: %v", prID, err)
		return domain.PullRequest{}, "", fmt.Errorf("failed to reassign reviewer: %w", err)
	}

	log.Printf("[Service] ReassignReviewer: replaced %q with %q in PR %q", oldReviewerID, replacement, prID)
	return updatedPR, replacement, nil
}

func (s *service) ListReviewerPullRequests(ctx context.Context, userID string) ([]domain.PullRequestShort, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("user ID is required")
	}
	prs, err := s.repo.ListReviewerPullRequests(ctx, userID)
	if err != nil {
		log.Printf("[Service] ListReviewerPullRequests: error fetching PRs for user %q: %v", userID, err)
		return nil, fmt.Errorf("failed to list reviewer pull requests: %w", err)
	}

	log.Printf("[Service] ListReviewerPullRequests: found %d PRs for user %q", len(prs), userID)
	return prs, nil
}

func (s *service) GetReviewerStats(ctx context.Context) ([]repository.ReviewerStats, error) {
	stats, err := s.repo.GetReviewerStats(ctx)
	if err != nil {
		log.Printf("[Service] GetReviewerStats: error fetching reviewer stats: %v", err)
		return nil, fmt.Errorf("failed to get reviewer stats: %w", err)
	}
	log.Printf("[Service] GetReviewerStats: fetched stats for %d reviewers", len(stats))
	return stats, nil
}

func (s *service) GetPRStats(ctx context.Context) (repository.PRStats, error) {
	stats, err := s.repo.GetPRStats(ctx)
	if err != nil {
		log.Printf("[Service] GetPRStats: error fetching PR stats: %v", err)
		return repository.PRStats{}, fmt.Errorf("failed to get PR stats: %w", err)
	}
	log.Printf("[Service] GetPRStats: total=%d open=%d merged=%d", stats.TotalPRs, stats.OpenPRs, stats.MergedPRs)
	return stats, nil
}

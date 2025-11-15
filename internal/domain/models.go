package domain

import "time"

type PullRequestStatus string

const (
	PullRequestStatusOpen   PullRequestStatus = "OPEN"
	PullRequestStatusMerged PullRequestStatus = "MERGED"
)

type Team struct {
	Name    string
	Members []User
}

type User struct {
	ID       string
	Username string
	TeamName string
	IsActive bool
}

type PullRequest struct {
	ID                string
	Name              string
	AuthorID          string
	Status            PullRequestStatus
	AssignedReviewers []string
	CreatedAt         time.Time
	MergedAt          *time.Time
}

func (pr PullRequest) NeedMoreReviewers() bool {
	return len(pr.AssignedReviewers) < 2
}

type PullRequestShort struct {
	ID       string
	Name     string
	AuthorID string
	Status   PullRequestStatus
}

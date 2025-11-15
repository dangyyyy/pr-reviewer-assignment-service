package domain

import "errors"

var (
	ErrTeamExists   = errors.New("team already exists")
	ErrPRExists     = errors.New("pull request already exists")
	ErrUserNotFound = errors.New("user not found")
	ErrTeamNotFound = errors.New("team not found")
	ErrPRNotFound   = errors.New("pull request not found")
	ErrPRMerged     = errors.New("pull request already merged")
	ErrNotAssigned  = errors.New("user is not assigned to pull request")
	ErrNoCandidate  = errors.New("no active candidates available")
)

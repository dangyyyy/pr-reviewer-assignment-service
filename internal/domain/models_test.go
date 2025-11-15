package domain

import "testing"

func TestPullRequest_NeedMoreReviewers(t *testing.T) {
	tests := []struct {
		name           string
		reviewersCount int
		want           bool
	}{
		{
			name:           "no reviewers",
			reviewersCount: 0,
			want:           true,
		},
		{
			name:           "one reviewer",
			reviewersCount: 1,
			want:           true,
		},
		{
			name:           "two reviewers",
			reviewersCount: 2,
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := PullRequest{
				AssignedReviewers: make([]string, tt.reviewersCount),
			}
			if got := pr.NeedMoreReviewers(); got != tt.want {
				t.Errorf("NeedMoreReviewers() = %v, want %v", got, tt.want)
			}
		})
	}
}

package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/dangy/pr-reviewer-assignment-service/internal/domain"
	"github.com/dangy/pr-reviewer-assignment-service/internal/service"
)

type Handler struct {
	svc        *service.Service
	adminToken string
	userToken  string
}

type errorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func New(svc *service.Service, adminToken, userToken string) *Handler {
	return &Handler{svc: svc, adminToken: adminToken, userToken: userToken}
}

func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RealIP)

	r.Get("/health", h.health)

	r.Post("/team/add", h.requireAdmin(h.createTeam))
	r.Get("/team/get", h.requireUserOrAdmin(h.getTeam))

	r.Post("/users/setIsActive", h.requireAdmin(h.setUserActive))
	r.Get("/users/getReview", h.requireUserOrAdmin(h.getUserReviewAssignments))

	r.Post("/pullRequest/create", h.requireAdmin(h.createPullRequest))
	r.Post("/pullRequest/merge", h.requireAdmin(h.mergePullRequest))
	r.Post("/pullRequest/reassign", h.requireAdmin(h.reassignReviewer))

	r.Get("/stats/reviewers", h.requireUserOrAdmin(h.getReviewerStats))
	r.Get("/stats/pullRequests", h.requireUserOrAdmin(h.getPRStats))

	return r
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) createTeam(w http.ResponseWriter, r *http.Request) {
	var req createTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "NOT_FOUND", "invalid JSON payload")
		return
	}
	if err := req.validate(); err != nil {
		writeError(w, http.StatusBadRequest, "NOT_FOUND", err.Error())
		return
	}

	team := domain.Team{Name: req.TeamName}
	for _, m := range req.Members {
		team.Members = append(team.Members, domain.User{
			ID:       m.UserID,
			Username: m.Username,
			TeamName: req.TeamName,
			IsActive: m.IsActive,
		})
	}

	created, err := h.svc.CreateTeam(r.Context(), team)
	if err != nil {
		status, code, message := mapDomainError(err)
		writeError(w, status, code, message)
		return
	}

	respondJSONWithStatus(w, http.StatusCreated, map[string]any{
		"team": mapTeam(created),
	})
}

func (h *Handler) getTeam(w http.ResponseWriter, r *http.Request) {
	teamName := strings.TrimSpace(r.URL.Query().Get("team_name"))
	if teamName == "" {
		writeError(w, http.StatusBadRequest, "NOT_FOUND", "team_name is required")
		return
	}

	team, err := h.svc.GetTeam(r.Context(), teamName)
	if err != nil {
		status, code, message := mapDomainError(err)
		writeError(w, status, code, message)
		return
	}

	respondJSON(w, http.StatusOK, mapTeam(team))
}

func (h *Handler) setUserActive(w http.ResponseWriter, r *http.Request) {
	var req setUserActiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "NOT_FOUND", "invalid JSON payload")
		return
	}
	if err := req.validate(); err != nil {
		writeError(w, http.StatusBadRequest, "NOT_FOUND", err.Error())
		return
	}

	user, err := h.svc.SetUserActivity(r.Context(), req.UserID, req.IsActive)
	if err != nil {
		status, code, message := mapDomainError(err)
		writeError(w, status, code, message)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"user": mapUser(user),
	})
}

func (h *Handler) createPullRequest(w http.ResponseWriter, r *http.Request) {
	var req createPullRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "NOT_FOUND", "invalid JSON payload")
		return
	}
	if err := req.validate(); err != nil {
		writeError(w, http.StatusBadRequest, "NOT_FOUND", err.Error())
		return
	}

	pr, err := h.svc.CreatePullRequest(r.Context(), req.PullRequestID, req.PullRequestName, req.AuthorID)
	if err != nil {
		status, code, message := mapDomainError(err)
		writeError(w, status, code, message)
		return
	}

	respondJSONWithStatus(w, http.StatusCreated, map[string]any{
		"pr": mapPullRequest(pr),
	})
}

func (h *Handler) mergePullRequest(w http.ResponseWriter, r *http.Request) {
	var req mergePullRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "NOT_FOUND", "invalid JSON payload")
		return
	}
	if err := req.validate(); err != nil {
		writeError(w, http.StatusBadRequest, "NOT_FOUND", err.Error())
		return
	}

	pr, err := h.svc.MergePullRequest(r.Context(), req.PullRequestID)
	if err != nil {
		status, code, message := mapDomainError(err)
		writeError(w, status, code, message)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"pr": mapPullRequest(pr),
	})
}

func (h *Handler) reassignReviewer(w http.ResponseWriter, r *http.Request) {
	var req reassignReviewerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "NOT_FOUND", "invalid JSON payload")
		return
	}
	if err := req.validate(); err != nil {
		writeError(w, http.StatusBadRequest, "NOT_FOUND", err.Error())
		return
	}

	pr, replacement, err := h.svc.ReassignReviewer(r.Context(), req.PullRequestID, req.OldUserID)
	if err != nil {
		status, code, message := mapDomainError(err)
		writeError(w, status, code, message)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"pr":          mapPullRequest(pr),
		"replaced_by": replacement,
	})
}

func (h *Handler) getUserReviewAssignments(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		writeError(w, http.StatusBadRequest, "NOT_FOUND", "user_id is required")
		return
	}

	prs, err := h.svc.ListReviewerPullRequests(r.Context(), userID)
	if err != nil {
		status, code, message := mapDomainError(err)
		writeError(w, status, code, message)
		return
	}

	var response []map[string]any
	for _, pr := range prs {
		response = append(response, map[string]any{
			"pull_request_id":   pr.ID,
			"pull_request_name": pr.Name,
			"author_id":         pr.AuthorID,
			"status":            string(pr.Status),
		})
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"user_id":       userID,
		"pull_requests": response,
	})
}

func (h *Handler) getReviewerStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.GetReviewerStats(r.Context())
	if err != nil {
		status, code, message := mapDomainError(err)
		writeError(w, status, code, message)
		return
	}

	var response []map[string]any
	for _, s := range stats {
		response = append(response, map[string]any{
			"user_id":           s.UserID,
			"username":          s.Username,
			"total_assignments": s.TotalAssignments,
		})
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"reviewers": response,
	})
}

func (h *Handler) getPRStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.GetPRStats(r.Context())
	if err != nil {
		status, code, message := mapDomainError(err)
		writeError(w, status, code, message)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"total_prs":             stats.TotalPRs,
		"open_prs":              stats.OpenPRs,
		"merged_prs":            stats.MergedPRs,
		"prs_with_reviewers":    stats.PRsWithReviewers,
		"prs_without_reviewers": stats.PRsWithoutReviewers,
	})
}

func (h *Handler) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.authorize(r, h.adminToken) {
			writeError(w, http.StatusUnauthorized, "NOT_FOUND", "unauthorized")
			return
		}
		next(w, r)
	}
}

func (h *Handler) requireUserOrAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.authorize(r, h.adminToken, h.userToken) {
			writeError(w, http.StatusUnauthorized, "NOT_FOUND", "unauthorized")
			return
		}
		next(w, r)
	}
}

func (h *Handler) authorize(r *http.Request, tokens ...string) bool {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader == "" {
		return false
	}

	token := authHeader
	if parts := strings.SplitN(authHeader, " ", 2); len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		token = parts[1]
	}

	for _, allowed := range tokens {
		if allowed != "" && token == allowed {
			return true
		}
	}

	return false
}

func mapTeam(team domain.Team) map[string]any {
	members := make([]map[string]any, 0, len(team.Members))
	for _, member := range team.Members {
		members = append(members, map[string]any{
			"user_id":   member.ID,
			"username":  member.Username,
			"is_active": member.IsActive,
		})
	}

	return map[string]any{
		"team_name": team.Name,
		"members":   members,
	}
}

func mapUser(user domain.User) map[string]any {
	return map[string]any{
		"user_id":   user.ID,
		"username":  user.Username,
		"team_name": user.TeamName,
		"is_active": user.IsActive,
	}
}

func mapPullRequest(pr domain.PullRequest) map[string]any {
	payload := map[string]any{
		"pull_request_id":    pr.ID,
		"pull_request_name":  pr.Name,
		"author_id":          pr.AuthorID,
		"status":             string(pr.Status),
		"assigned_reviewers": pr.AssignedReviewers,
	}

	if !pr.CreatedAt.IsZero() {
		created := pr.CreatedAt.UTC()
		payload["createdAt"] = created
	}
	if pr.MergedAt != nil {
		merged := pr.MergedAt.UTC()
		payload["mergedAt"] = merged
	}

	return payload
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	respondJSONWithStatus(w, status, errorBody{
		Error: apiError{Code: code, Message: message},
	})
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	respondJSONWithStatus(w, status, payload)
}

func respondJSONWithStatus(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

type createTeamRequest struct {
	TeamName string              `json:"team_name"`
	Members  []teamMemberRequest `json:"members"`
}

type teamMemberRequest struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

type setUserActiveRequest struct {
	UserID   string `json:"user_id"`
	IsActive bool   `json:"is_active"`
}

type createPullRequestRequest struct {
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	AuthorID        string `json:"author_id"`
}

type mergePullRequestRequest struct {
	PullRequestID string `json:"pull_request_id"`
}

type reassignReviewerRequest struct {
	PullRequestID string `json:"pull_request_id"`
	OldUserID     string `json:"old_user_id"`
}

func mapDomainError(err error) (int, string, string) {
	switch {
	case errors.Is(err, domain.ErrTeamExists):
		return http.StatusBadRequest, "TEAM_EXISTS", err.Error()
	case errors.Is(err, domain.ErrPRExists):
		return http.StatusConflict, "PR_EXISTS", err.Error()
	case errors.Is(err, domain.ErrPRMerged):
		return http.StatusConflict, "PR_MERGED", err.Error()
	case errors.Is(err, domain.ErrNotAssigned):
		return http.StatusConflict, "NOT_ASSIGNED", err.Error()
	case errors.Is(err, domain.ErrNoCandidate):
		return http.StatusConflict, "NO_CANDIDATE", err.Error()
	case errors.Is(err, domain.ErrUserNotFound), errors.Is(err, domain.ErrTeamNotFound), errors.Is(err, domain.ErrPRNotFound):
		return http.StatusNotFound, "NOT_FOUND", err.Error()
	default:
		return http.StatusInternalServerError, "NOT_FOUND", "internal error"
	}
}

func (r *createTeamRequest) validate() error {
	if strings.TrimSpace(r.TeamName) == "" {
		return errors.New("team_name is required")
	}
	for idx, member := range r.Members {
		if strings.TrimSpace(member.UserID) == "" {
			return errors.New("members[" + strconv.Itoa(idx) + "].user_id is required")
		}
		if strings.TrimSpace(member.Username) == "" {
			return errors.New("members[" + strconv.Itoa(idx) + "].username is required")
		}
	}
	return nil
}

func (r *setUserActiveRequest) validate() error {
	if strings.TrimSpace(r.UserID) == "" {
		return errors.New("user_id is required")
	}
	return nil
}

func (r *createPullRequestRequest) validate() error {
	if strings.TrimSpace(r.PullRequestID) == "" {
		return errors.New("pull_request_id is required")
	}
	if strings.TrimSpace(r.PullRequestName) == "" {
		return errors.New("pull_request_name is required")
	}
	if strings.TrimSpace(r.AuthorID) == "" {
		return errors.New("author_id is required")
	}
	return nil
}

func (r *mergePullRequestRequest) validate() error {
	if strings.TrimSpace(r.PullRequestID) == "" {
		return errors.New("pull_request_id is required")
	}
	return nil
}

func (r *reassignReviewerRequest) validate() error {
	if strings.TrimSpace(r.PullRequestID) == "" {
		return errors.New("pull_request_id is required")
	}
	if strings.TrimSpace(r.OldUserID) == "" {
		return errors.New("old_user_id is required")
	}
	return nil
}

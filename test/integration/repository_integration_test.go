package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dangy/pr-reviewer-assignment-service/internal/domain"
	"github.com/dangy/pr-reviewer-assignment-service/internal/repository"
	"github.com/dangy/pr-reviewer-assignment-service/internal/storage/schema"
)

var testDBPool *pgxpool.Pool

// TestMain выполняется один раз перед всеми тестами
func TestMain(m *testing.M) {
	ctx := context.Background()

	// Получить URL базы данных из переменной окружения
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/pr_service?sslmode=disable"
	}

	var err error
	testDBPool, err = pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	// Проверить соединение
	if err := testDBPool.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to ping database: %v\n", err)
		testDBPool.Close()
		os.Exit(1)
	}

	// Создать схему
	if err := schema.Ensure(ctx, testDBPool); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create schema: %v\n", err)
		testDBPool.Close()
		os.Exit(1)
	}

	// Запустить тесты
	code := m.Run()

	// Очистка
	testDBPool.Close()
	os.Exit(code)
}

// setupTest создает новый репозиторий и очищает таблицы перед каждым тестом
func setupTest(t *testing.T) (*repository.Repository, func()) {
	ctx := context.Background()

	// Очистить таблицы в правильном порядке (из-за foreign keys)
	_, err := testDBPool.Exec(ctx, `
		TRUNCATE TABLE pull_request_reviewers CASCADE;
		TRUNCATE TABLE pull_requests CASCADE;
		TRUNCATE TABLE users CASCADE;
		TRUNCATE TABLE teams CASCADE;
	`)
	require.NoError(t, err, "Failed to truncate tables")

	repo := repository.New(testDBPool)

	cleanup := func() {
		// Дополнительная очистка если нужна
	}

	return repo, cleanup
}

func TestCreateTeam(t *testing.T) {
	repo, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("успешное создание команды с участниками", func(t *testing.T) {
		team := domain.Team{
			Name: "backend",
			Members: []domain.User{
				{ID: "u1", Username: "Alice", IsActive: true},
				{ID: "u2", Username: "Bob", IsActive: true},
			},
		}

		created, err := repo.CreateTeam(ctx, team)
		require.NoError(t, err)
		assert.Equal(t, "backend", created.Name)
		assert.Len(t, created.Members, 2)

		// Проверить что участники созданы корректно
		assert.Equal(t, "Alice", created.Members[0].Username)
		assert.Equal(t, "backend", created.Members[0].TeamName)
		assert.True(t, created.Members[0].IsActive)
	})

	t.Run("попытка создать дубликат команды", func(t *testing.T) {
		team := domain.Team{
			Name: "frontend",
			Members: []domain.User{
				{ID: "u3", Username: "Charlie", IsActive: true},
			},
		}

		_, err := repo.CreateTeam(ctx, team)
		require.NoError(t, err)

		// Повторная попытка создания должна вернуть ошибку
		_, err = repo.CreateTeam(ctx, team)
		assert.ErrorIs(t, err, domain.ErrTeamExists)
	})

	t.Run("обновление пользователя при добавлении в новую команду", func(t *testing.T) {
		// Создать первую команду с пользователем
		team1 := domain.Team{
			Name: "team1",
			Members: []domain.User{
				{ID: "u4", Username: "David", IsActive: true},
			},
		}
		_, err := repo.CreateTeam(ctx, team1)
		require.NoError(t, err)

		// Создать вторую команду с тем же пользователем (обновление)
		team2 := domain.Team{
			Name: "team2",
			Members: []domain.User{
				{ID: "u4", Username: "David Updated", IsActive: false},
			},
		}
		created, err := repo.CreateTeam(ctx, team2)
		require.NoError(t, err)

		// Проверить что пользователь обновился
		assert.Equal(t, "David Updated", created.Members[0].Username)
		assert.Equal(t, "team2", created.Members[0].TeamName)
		assert.False(t, created.Members[0].IsActive)
	})
}

func TestGetTeam(t *testing.T) {
	repo, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("получение существующей команды", func(t *testing.T) {
		// Создать команду
		team := domain.Team{
			Name: "backend",
			Members: []domain.User{
				{ID: "u1", Username: "Alice", IsActive: true},
				{ID: "u2", Username: "Bob", IsActive: false},
			},
		}
		_, err := repo.CreateTeam(ctx, team)
		require.NoError(t, err)

		// Получить команду
		found, err := repo.GetTeam(ctx, "backend")
		require.NoError(t, err)
		assert.Equal(t, "backend", found.Name)
		assert.Len(t, found.Members, 2)

		// Проверить сортировку по username (ASC)
		assert.Equal(t, "Alice", found.Members[0].Username)
		assert.Equal(t, "Bob", found.Members[1].Username)
	})

	t.Run("несуществующая команда возвращает ошибку", func(t *testing.T) {
		_, err := repo.GetTeam(ctx, "nonexistent")
		assert.ErrorIs(t, err, domain.ErrTeamNotFound)
	})
}

func TestSetUserActivity(t *testing.T) {
	repo, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	// Подготовка: создать команду с пользователем
	team := domain.Team{
		Name: "backend",
		Members: []domain.User{
			{ID: "u1", Username: "Alice", IsActive: true},
		},
	}
	_, err := repo.CreateTeam(ctx, team)
	require.NoError(t, err)

	t.Run("деактивация активного пользователя", func(t *testing.T) {
		user, err := repo.SetUserActivity(ctx, "u1", false)
		require.NoError(t, err)
		assert.False(t, user.IsActive)
		assert.Equal(t, "Alice", user.Username)
		assert.Equal(t, "backend", user.TeamName)
	})

	t.Run("активация неактивного пользователя", func(t *testing.T) {
		user, err := repo.SetUserActivity(ctx, "u1", true)
		require.NoError(t, err)
		assert.True(t, user.IsActive)
	})

	t.Run("несуществующий пользователь возвращает ошибку", func(t *testing.T) {
		_, err := repo.SetUserActivity(ctx, "nonexistent", true)
		assert.ErrorIs(t, err, domain.ErrUserNotFound)
	})
}

func TestCreatePullRequest(t *testing.T) {
	repo, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	// Подготовка: создать команду из 4 человек
	team := domain.Team{
		Name: "backend",
		Members: []domain.User{
			{ID: "u1", Username: "Alice", IsActive: true},
			{ID: "u2", Username: "Bob", IsActive: true},
			{ID: "u3", Username: "Charlie", IsActive: true},
			{ID: "u4", Username: "David", IsActive: false}, // Неактивный
		},
	}
	_, err := repo.CreateTeam(ctx, team)
	require.NoError(t, err)

	t.Run("успешное создание PR с назначением ревьюеров", func(t *testing.T) {
		pr, err := repo.CreatePullRequest(ctx, "pr1", "Add feature", "u1")
		require.NoError(t, err)

		assert.Equal(t, "pr1", pr.ID)
		assert.Equal(t, "Add feature", pr.Name)
		assert.Equal(t, "u1", pr.AuthorID)
		assert.Equal(t, domain.PullRequestStatusOpen, pr.Status)
		assert.NotZero(t, pr.CreatedAt)
		assert.Nil(t, pr.MergedAt)

		// Должно быть назначено 1-2 ревьюера (из u2, u3)
		assert.GreaterOrEqual(t, len(pr.AssignedReviewers), 1)
		assert.LessOrEqual(t, len(pr.AssignedReviewers), 2)

		// Автор не должен быть назначен сам себе
		for _, reviewerID := range pr.AssignedReviewers {
			assert.NotEqual(t, "u1", reviewerID, "Author should not be assigned as reviewer")
		}
	})

	t.Run("неактивные пользователи не назначаются", func(t *testing.T) {
		pr, err := repo.CreatePullRequest(ctx, "pr2", "Fix bug", "u1")
		require.NoError(t, err)

		// u4 не должен быть назначен (is_active = false)
		for _, reviewerID := range pr.AssignedReviewers {
			assert.NotEqual(t, "u4", reviewerID, "Inactive user should not be assigned")
		}
	})

	t.Run("создание PR в команде из одного человека", func(t *testing.T) {
		// Создать команду с одним участником
		soloTeam := domain.Team{
			Name: "solo",
			Members: []domain.User{
				{ID: "u5", Username: "Solo", IsActive: true},
			},
		}
		_, err := repo.CreateTeam(ctx, soloTeam)
		require.NoError(t, err)

		pr, err := repo.CreatePullRequest(ctx, "pr3", "Solo PR", "u5")
		require.NoError(t, err)

		// Ревьюеров не должно быть (некого назначить)
		assert.Len(t, pr.AssignedReviewers, 0)
	})

	t.Run("попытка создать дубликат PR", func(t *testing.T) {
		_, err := repo.CreatePullRequest(ctx, "pr1", "Duplicate", "u1")
		assert.ErrorIs(t, err, domain.ErrPRExists)
	})

	t.Run("несуществующий автор возвращает ошибку", func(t *testing.T) {
		_, err := repo.CreatePullRequest(ctx, "pr4", "Invalid", "nonexistent")
		assert.ErrorIs(t, err, domain.ErrUserNotFound)
	})
}

func TestGetPullRequest(t *testing.T) {
	repo, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	// Подготовка
	team := domain.Team{
		Name: "backend",
		Members: []domain.User{
			{ID: "u1", Username: "Alice", IsActive: true},
			{ID: "u2", Username: "Bob", IsActive: true},
		},
	}
	_, err := repo.CreateTeam(ctx, team)
	require.NoError(t, err)

	pr, err := repo.CreatePullRequest(ctx, "pr1", "Test PR", "u1")
	require.NoError(t, err)

	t.Run("получение существующего PR", func(t *testing.T) {
		found, err := repo.GetPullRequest(ctx, "pr1")
		require.NoError(t, err)
		assert.Equal(t, pr.ID, found.ID)
		assert.Equal(t, pr.Name, found.Name)
		assert.Equal(t, pr.Status, found.Status)
	})

	t.Run("несуществующий PR возвращает ошибку", func(t *testing.T) {
		_, err := repo.GetPullRequest(ctx, "nonexistent")
		assert.ErrorIs(t, err, domain.ErrPRNotFound)
	})
}

func TestMergePullRequest(t *testing.T) {
	repo, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	// Подготовка
	team := domain.Team{
		Name: "backend",
		Members: []domain.User{
			{ID: "u1", Username: "Alice", IsActive: true},
			{ID: "u2", Username: "Bob", IsActive: true},
		},
	}
	_, err := repo.CreateTeam(ctx, team)
	require.NoError(t, err)

	pr, err := repo.CreatePullRequest(ctx, "pr1", "Test PR", "u1")
	require.NoError(t, err)
	require.Equal(t, domain.PullRequestStatusOpen, pr.Status)

	t.Run("успешный merge PR", func(t *testing.T) {
		merged, err := repo.MergePullRequest(ctx, "pr1")
		require.NoError(t, err)

		assert.Equal(t, domain.PullRequestStatusMerged, merged.Status)
		assert.NotNil(t, merged.MergedAt)
		assert.WithinDuration(t, time.Now(), *merged.MergedAt, 2*time.Second)
	})

	t.Run("повторный merge возвращает тот же PR (идемпотентность)", func(t *testing.T) {
		firstMerge, err := repo.MergePullRequest(ctx, "pr1")
		require.NoError(t, err)
		firstTime := firstMerge.MergedAt

		secondMerge, err := repo.MergePullRequest(ctx, "pr1")
		require.NoError(t, err)

		assert.Equal(t, domain.PullRequestStatusMerged, secondMerge.Status)
		// Время merge не должно измениться
		assert.Equal(t, firstTime, secondMerge.MergedAt)
	})

	t.Run("несуществующий PR возвращает ошибку", func(t *testing.T) {
		_, err := repo.MergePullRequest(ctx, "nonexistent")
		assert.ErrorIs(t, err, domain.ErrPRNotFound)
	})
}

func TestReassignReviewer(t *testing.T) {
	repo, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	// Подготовка: команда из 4 человек
	team := domain.Team{
		Name: "backend",
		Members: []domain.User{
			{ID: "u1", Username: "Alice", IsActive: true},
			{ID: "u2", Username: "Bob", IsActive: true},
			{ID: "u3", Username: "Charlie", IsActive: true},
			{ID: "u4", Username: "David", IsActive: true},
		},
	}
	_, err := repo.CreateTeam(ctx, team)
	require.NoError(t, err)

	pr, err := repo.CreatePullRequest(ctx, "pr1", "Test PR", "u1")
	require.NoError(t, err)
	require.Greater(t, len(pr.AssignedReviewers), 0, "PR should have reviewers")

	t.Run("успешное переназначение ревьюера", func(t *testing.T) {
		oldReviewer := pr.AssignedReviewers[0]

		updated, newReviewer, err := repo.ReassignReviewer(ctx, "pr1", oldReviewer)
		require.NoError(t, err)
		assert.NotEmpty(t, newReviewer)

		// Старого ревьюера не должно быть в списке
		for _, id := range updated.AssignedReviewers {
			assert.NotEqual(t, oldReviewer, id)
		}

		// Новый ревьюер должен быть в списке
		assert.Contains(t, updated.AssignedReviewers, newReviewer)

		// Автор не должен быть назначен
		for _, id := range updated.AssignedReviewers {
			assert.NotEqual(t, "u1", id)
		}
	})

	t.Run("попытка переназначить не назначенного ревьюера", func(t *testing.T) {
		// Создать команду где точно известны назначенные ревьюеры
		testTeam := domain.Team{
			Name: "test-not-assigned",
			Members: []domain.User{
				{ID: "test-author", Username: "TestAuthor", IsActive: true},
				{ID: "test-reviewer-active", Username: "TestReviewerActive", IsActive: true},
				{ID: "test-reviewer-inactive", Username: "TestReviewerInactive", IsActive: false},
			},
		}
		_, err := repo.CreateTeam(ctx, testTeam)
		require.NoError(t, err)

		// Создать PR - будет назначен только test-reviewer-active
		testPR, err := repo.CreatePullRequest(ctx, "test-pr-not-assigned", "Test", "test-author")
		require.NoError(t, err)

		// Убедиться что назначен только активный ревьюер
		require.Len(t, testPR.AssignedReviewers, 1)
		require.Equal(t, "test-reviewer-active", testPR.AssignedReviewers[0])

		// Попытка переназначить неактивного пользователя (не назначен)
		_, _, err = repo.ReassignReviewer(ctx, "test-pr-not-assigned", "test-reviewer-inactive")
		assert.ErrorIs(t, err, domain.ErrNotAssigned)
	})

	t.Run("переназначение на смерженном PR", func(t *testing.T) {
		// Создать и смержить PR
		pr2, err := repo.CreatePullRequest(ctx, "pr2", "Another PR", "u1")
		require.NoError(t, err)

		_, err = repo.MergePullRequest(ctx, "pr2")
		require.NoError(t, err)

		if len(pr2.AssignedReviewers) > 0 {
			_, _, err = repo.ReassignReviewer(ctx, "pr2", pr2.AssignedReviewers[0])
			assert.ErrorIs(t, err, domain.ErrPRMerged)
		}
	})

	t.Run("нет доступных кандидатов для замены", func(t *testing.T) {
		// Создать команду из 2 активных человек
		smallTeam := domain.Team{
			Name: "small",
			Members: []domain.User{
				{ID: "u5", Username: "Eve", IsActive: true},
				{ID: "u6", Username: "Frank", IsActive: true},
			},
		}
		_, err := repo.CreateTeam(ctx, smallTeam)
		require.NoError(t, err)

		pr3, err := repo.CreatePullRequest(ctx, "pr3", "Small team PR", "u5")
		require.NoError(t, err)

		// Если u6 назначен, попытка переназначить должна вернуть NO_CANDIDATE
		if len(pr3.AssignedReviewers) > 0 && pr3.AssignedReviewers[0] == "u6" {
			_, _, err = repo.ReassignReviewer(ctx, "pr3", "u6")
			assert.ErrorIs(t, err, domain.ErrNoCandidate)
		}
	})

	t.Run("несуществующий PR возвращает ошибку", func(t *testing.T) {
		_, _, err := repo.ReassignReviewer(ctx, "nonexistent", "u2")
		assert.ErrorIs(t, err, domain.ErrPRNotFound)
	})
}

func TestListReviewerPullRequests(t *testing.T) {
	repo, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	// Подготовка
	team := domain.Team{
		Name: "backend",
		Members: []domain.User{
			{ID: "u1", Username: "Alice", IsActive: true},
			{ID: "u2", Username: "Bob", IsActive: true},
			{ID: "u3", Username: "Charlie", IsActive: true},
		},
	}
	_, err := repo.CreateTeam(ctx, team)
	require.NoError(t, err)

	// Создать несколько PR от u1
	_, err = repo.CreatePullRequest(ctx, "pr1", "PR 1", "u1")
	require.NoError(t, err)
	_, err = repo.CreatePullRequest(ctx, "pr2", "PR 2", "u1")
	require.NoError(t, err)
	_, err = repo.CreatePullRequest(ctx, "pr3", "PR 3", "u1")
	require.NoError(t, err)

	t.Run("получение списка PR для ревьюера", func(t *testing.T) {
		prs, err := repo.ListReviewerPullRequests(ctx, "u2")
		require.NoError(t, err)

		// u2 должен быть назначен хотя бы на один PR (random selection)
		assert.NotNil(t, prs)

		for _, pr := range prs {
			assert.NotEmpty(t, pr.ID)
			assert.NotEmpty(t, pr.Name)
			assert.NotEmpty(t, pr.Status)
			assert.Equal(t, "u1", pr.AuthorID)
		}
	})

	t.Run("пользователь без назначений возвращает пустой список", func(t *testing.T) {
		// Создать нового пользователя который не участвовал в ревью
		newTeam := domain.Team{
			Name: "other",
			Members: []domain.User{
				{ID: "u10", Username: "NewUser", IsActive: true},
			},
		}
		_, err := repo.CreateTeam(ctx, newTeam)
		require.NoError(t, err)

		prs, err := repo.ListReviewerPullRequests(ctx, "u10")
		require.NoError(t, err)
		assert.Empty(t, prs)
	})
}

func TestGetReviewerStats(t *testing.T) {
	repo, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	// Подготовка
	team := domain.Team{
		Name: "backend",
		Members: []domain.User{
			{ID: "u1", Username: "Alice", IsActive: true},
			{ID: "u2", Username: "Bob", IsActive: true},
			{ID: "u3", Username: "Charlie", IsActive: true},
		},
	}
	_, err := repo.CreateTeam(ctx, team)
	require.NoError(t, err)

	// Создать несколько PR для статистики
	for i := 1; i <= 5; i++ {
		_, err = repo.CreatePullRequest(ctx, fmt.Sprintf("pr%d", i), fmt.Sprintf("PR %d", i), "u1")
		require.NoError(t, err)
	}

	t.Run("получение статистики по ревьюерам", func(t *testing.T) {
		stats, err := repo.GetReviewerStats(ctx)
		require.NoError(t, err)

		assert.Len(t, stats, 3, "Should have stats for all 3 users")

		// Проверить что все пользователи есть
		userIDs := make(map[string]bool)
		for _, s := range stats {
			userIDs[s.UserID] = true
			assert.NotEmpty(t, s.Username)
			assert.GreaterOrEqual(t, s.TotalAssignments, 0)
		}
		assert.True(t, userIDs["u1"])
		assert.True(t, userIDs["u2"])
		assert.True(t, userIDs["u3"])

		// Сумма назначений должна быть около 10 (5 PR * до 2 ревьюеров)
		totalAssignments := 0
		for _, s := range stats {
			totalAssignments += s.TotalAssignments
		}
		assert.GreaterOrEqual(t, totalAssignments, 5)
		assert.LessOrEqual(t, totalAssignments, 10)
	})
}

func TestGetPRStats(t *testing.T) {
	repo, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	// Подготовка
	team := domain.Team{
		Name: "backend",
		Members: []domain.User{
			{ID: "u1", Username: "Alice", IsActive: true},
			{ID: "u2", Username: "Bob", IsActive: true},
		},
	}
	_, err := repo.CreateTeam(ctx, team)
	require.NoError(t, err)

	// Создать несколько PR
	pr1, err := repo.CreatePullRequest(ctx, "pr1", "PR 1", "u1")
	require.NoError(t, err)
	_, err = repo.CreatePullRequest(ctx, "pr2", "PR 2", "u1")
	require.NoError(t, err)
	_, err = repo.CreatePullRequest(ctx, "pr3", "PR 3", "u1")
	require.NoError(t, err)

	// Смержить один PR
	_, err = repo.MergePullRequest(ctx, pr1.ID)
	require.NoError(t, err)

	t.Run("получение статистики по PR", func(t *testing.T) {
		stats, err := repo.GetPRStats(ctx)
		require.NoError(t, err)

		assert.Equal(t, 3, stats.TotalPRs)
		assert.Equal(t, 2, stats.OpenPRs)
		assert.Equal(t, 1, stats.MergedPRs)

		// Проверить что PR с ревьюерами учтены
		assert.GreaterOrEqual(t, stats.PRsWithReviewers, 0)
		assert.Equal(t, stats.TotalPRs, stats.PRsWithReviewers+stats.PRsWithoutReviewers)
	})

	t.Run("статистика для пустой базы", func(t *testing.T) {
		// Очистить все
		_, err := testDBPool.Exec(ctx, "TRUNCATE TABLE pull_request_reviewers, pull_requests, users, teams CASCADE")
		require.NoError(t, err)

		stats, err := repo.GetPRStats(ctx)
		require.NoError(t, err)

		assert.Equal(t, 0, stats.TotalPRs)
		assert.Equal(t, 0, stats.OpenPRs)
		assert.Equal(t, 0, stats.MergedPRs)
		assert.Equal(t, 0, stats.PRsWithReviewers)
		assert.Equal(t, 0, stats.PRsWithoutReviewers)
	})
}

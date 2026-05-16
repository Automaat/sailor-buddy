package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func trainingTestAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterTrainingRoutes(api, m)
	return api
}

func TestTrainingHandler_List(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			listTrainingsFn: func(context.Context, int64) ([]sqlcdb.Training, error) {
				return []sqlcdb.Training{{ID: 1, Name: "RYA Day Skipper"}}, nil
			},
			countTrainingsFn: func(context.Context, int64) (int64, error) { return 1, nil },
		}
		resp := trainingTestAPI(t, m).GetCtx(userCtx(context.Background()), "/trainings")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			listTrainingsFn: func(context.Context, int64) ([]sqlcdb.Training, error) {
				return nil, errors.New("fail")
			},
		}
		resp := trainingTestAPI(t, m).GetCtx(userCtx(context.Background()), "/trainings")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestTrainingHandler_Get(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getTrainingFn: func(_ context.Context, arg sqlcdb.GetTrainingParams) (sqlcdb.Training, error) {
				return sqlcdb.Training{ID: arg.ID, Name: "RYA"}, nil
			},
		}
		resp := trainingTestAPI(t, m).GetCtx(userCtx(context.Background()), "/trainings/1")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("not found", func(t *testing.T) {
		m := &mockQuerier{
			getTrainingFn: func(context.Context, sqlcdb.GetTrainingParams) (sqlcdb.Training, error) {
				return sqlcdb.Training{}, sql.ErrNoRows
			},
		}
		resp := trainingTestAPI(t, m).GetCtx(userCtx(context.Background()), "/trainings/1")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			getTrainingFn: func(context.Context, sqlcdb.GetTrainingParams) (sqlcdb.Training, error) {
				return sqlcdb.Training{}, errors.New("fail")
			},
		}
		resp := trainingTestAPI(t, m).GetCtx(userCtx(context.Background()), "/trainings/1")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestTrainingHandler_Create(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			createTrainingFn: func(_ context.Context, arg sqlcdb.CreateTrainingParams) (sqlcdb.Training, error) {
				return sqlcdb.Training{ID: 1, Name: arg.Name}, nil
			},
		}
		resp := trainingTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trainings", map[string]any{"name": "RYA"})
		if resp.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		resp := trainingTestAPI(t, &mockQuerier{}).PostCtx(userCtx(context.Background()), "/trainings", map[string]any{})
		if resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got %d, want 422", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			createTrainingFn: func(context.Context, sqlcdb.CreateTrainingParams) (sqlcdb.Training, error) {
				return sqlcdb.Training{}, errors.New("fail")
			},
		}
		resp := trainingTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trainings", map[string]any{"name": "X"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestTrainingHandler_Update(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			updateTrainingFn: func(context.Context, sqlcdb.UpdateTrainingParams) error { return nil },
		}
		resp := trainingTestAPI(t, m).PutCtx(userCtx(context.Background()), "/trainings/1", map[string]any{"name": "Updated"})
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		resp := trainingTestAPI(t, &mockQuerier{}).PutCtx(userCtx(context.Background()), "/trainings/1", map[string]any{})
		if resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got %d, want 422", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			updateTrainingFn: func(context.Context, sqlcdb.UpdateTrainingParams) error { return errors.New("fail") },
		}
		resp := trainingTestAPI(t, m).PutCtx(userCtx(context.Background()), "/trainings/1", map[string]any{"name": "X"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestTrainingHandler_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			deleteTrainingFn: func(context.Context, sqlcdb.DeleteTrainingParams) error { return nil },
		}
		resp := trainingTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/trainings/1")
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			deleteTrainingFn: func(context.Context, sqlcdb.DeleteTrainingParams) error { return errors.New("fail") },
		}
		resp := trainingTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/trainings/1")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

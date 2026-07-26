package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"golf/internal/auth"
	"golf/internal/database"
	"golf/internal/handicap"
)

type API struct {
	store *database.Store
}

func New(store *database.Store, authentication *auth.Manager) http.Handler {
	api := &API{store: store}
	mux := http.NewServeMux()
	authentication.Register(mux)
	mux.HandleFunc("GET /api/health", api.health)
	mux.HandleFunc("GET /api/players", api.players)
	mux.HandleFunc("POST /api/players", api.createPlayer)
	mux.HandleFunc("GET /api/players/{id}", api.player)
	mux.HandleFunc("PUT /api/players/{id}", api.updatePlayer)
	mux.HandleFunc("DELETE /api/players/{id}", api.deletePlayer)
	mux.HandleFunc("GET /api/courses", api.courses)
	mux.HandleFunc("POST /api/courses", api.createTee)
	mux.HandleFunc("PUT /api/courses/{id}", api.updateCourse)
	mux.HandleFunc("PUT /api/tees/{id}", api.updateTee)
	mux.HandleFunc("DELETE /api/tees/{id}", api.deleteTee)
	mux.HandleFunc("GET /api/rounds", api.rounds)
	mux.HandleFunc("POST /api/rounds", api.createRound)
	mux.HandleFunc("GET /api/rounds/{id}", api.round)
	mux.HandleFunc("PUT /api/rounds/{id}", api.updateRound)
	mux.HandleFunc("DELETE /api/rounds/{id}", api.deleteRound)
	return recoverMiddleware(authentication.ProtectWrites(mux))
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) players(w http.ResponseWriter, r *http.Request) {
	players, err := a.store.Players(r.Context())
	respond(w, players, err)
}

func (a *API) player(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	player, rounds, err := a.store.Player(r.Context(), id)
	if err != nil {
		respond(w, nil, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"player": player, "rounds": rounds})
}

func (a *API) createPlayer(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name                   string `json:"name"`
		StartingCourseHandicap *int   `json:"startingCourseHandicap"`
	}
	if !decode(w, r, &input) {
		return
	}
	player, err := a.store.CreatePlayer(r.Context(), input.Name, input.StartingCourseHandicap)
	if err != nil {
		respond(w, nil, err)
		return
	}
	writeJSON(w, http.StatusCreated, player)
}

func (a *API) updatePlayer(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var input struct {
		Name                   string   `json:"name"`
		StartingCourseHandicap *int     `json:"startingCourseHandicap"`
		OfficialHandicapIndex  *float64 `json:"officialHandicapIndex"`
		OfficialHandicapDate   *string  `json:"officialHandicapDate"`
	}
	if !decode(w, r, &input) {
		return
	}
	player, err := a.store.UpdatePlayer(r.Context(), id, input.Name, input.StartingCourseHandicap,
		input.OfficialHandicapIndex, input.OfficialHandicapDate)
	respond(w, player, err)
}

func (a *API) deletePlayer(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err == nil {
		err = a.store.DeletePlayer(r.Context(), id)
	}
	if err != nil {
		respond(w, nil, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) courses(w http.ResponseWriter, r *http.Request) {
	courses, err := a.store.Courses(r.Context())
	respond(w, courses, err)
}

func (a *API) createTee(w http.ResponseWriter, r *http.Request) {
	var course handicap.Course
	if !decode(w, r, &course) {
		return
	}
	tee, err := a.store.CreateTee(r.Context(), course)
	if err != nil {
		respond(w, nil, err)
		return
	}
	writeJSON(w, http.StatusCreated, tee)
}

func (a *API) updateCourse(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &input) {
		return
	}
	course, err := a.store.UpdateCourse(r.Context(), id, input.Name)
	respond(w, course, err)
}

func (a *API) updateTee(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var input handicap.Course
	if !decode(w, r, &input) {
		return
	}
	tee, err := a.store.UpdateTee(r.Context(), id, input)
	respond(w, tee, err)
}

func (a *API) deleteTee(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err == nil {
		err = a.store.DeleteTee(r.Context(), id)
	}
	if err != nil {
		respond(w, nil, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) rounds(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}
	rounds, err := a.store.Rounds(r.Context(), limit)
	respond(w, rounds, err)
}

func (a *API) round(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	round, err := a.store.Round(r.Context(), id)
	respond(w, round, err)
}

func (a *API) createRound(w http.ResponseWriter, r *http.Request) {
	var input database.CreateRoundInput
	if !decode(w, r, &input) {
		return
	}
	round, err := a.store.CreateRound(r.Context(), input)
	if err != nil {
		respond(w, nil, err)
		return
	}
	writeJSON(w, http.StatusCreated, round)
}

func (a *API) updateRound(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var input database.CreateRoundInput
	if !decode(w, r, &input) {
		return
	}
	round, err := a.store.UpdateRound(r.Context(), id, input)
	respond(w, round, err)
}

func (a *API) deleteRound(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err == nil {
		err = a.store.DeleteRound(r.Context(), id)
	}
	if err != nil {
		respond(w, nil, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid id")
	}
	return id, nil
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request: %w", err))
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, fmt.Errorf("request body must contain one JSON object"))
		return false
	}
	return true
}

func respond(w http.ResponseWriter, data any, err error) {
	if err == nil {
		writeJSON(w, http.StatusOK, data)
		return
	}
	status := http.StatusBadRequest
	if errors.Is(err, sql.ErrNoRows) {
		status = http.StatusNotFound
	} else if !isUserError(err) {
		status = http.StatusInternalServerError
	}
	writeError(w, status, err)
}

func isUserError(err error) bool {
	message := strings.ToLower(err.Error())
	for _, marker := range []string{"required", "already", "cannot", "must", "between", "not found", "does not belong", "twice"} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				writeError(w, http.StatusInternalServerError, fmt.Errorf("internal server error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

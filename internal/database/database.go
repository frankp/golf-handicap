package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golf/internal/handicap"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Player struct {
	ID                     int64    `json:"id"`
	Name                   string   `json:"name"`
	StartingCourseHandicap *int     `json:"startingCourseHandicap"`
	OfficialHandicapIndex  *float64 `json:"officialHandicapIndex"`
	OfficialHandicapDate   *string  `json:"officialHandicapDate"`
	GroupHandicapIndex     *float64 `json:"groupHandicapIndex"`
	RoundCount             int      `json:"roundCount"`
}

type Tee struct {
	ID          int64   `json:"id"`
	CourseID    int64   `json:"courseId"`
	CourseName  string  `json:"courseName"`
	Name        string  `json:"name"`
	Rating      float64 `json:"rating"`
	Slope       int     `json:"slope"`
	Par         [18]int `json:"par"`
	StrokeIndex [18]int `json:"strokeIndex"`
	TotalPar    int     `json:"totalPar"`
}

type Course struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Tees []Tee  `json:"tees"`
}

type RoundPlayer struct {
	ID                    int64    `json:"id"`
	PlayerID              int64    `json:"playerId"`
	PlayerName            string   `json:"playerName"`
	TeeID                 int64    `json:"teeId"`
	TeeName               string   `json:"teeName"`
	Scores                [18]int  `json:"scores"`
	Gross                 int      `json:"gross"`
	HandicapUsed          *float64 `json:"handicapUsed"`
	NetScore              *float64 `json:"netScore"`
	NetScores             *[18]int `json:"netScores"`
	CourseHandicap        int      `json:"courseHandicap"`
	AdjustedGross         int      `json:"adjustedGross"`
	ScoreDifferential     float64  `json:"scoreDifferential"`
	HandicapIndexAfter    *float64 `json:"handicapIndexAfter"`
	StartingHandicapUsed  bool     `json:"startingHandicapUsed"`
	InitialParFiveCapUsed bool     `json:"initialParFiveCapUsed"`
	Counting              bool     `json:"counting"`
}

type Round struct {
	ID           int64         `json:"id"`
	PlayedOn     string        `json:"playedOn"`
	CourseID     int64         `json:"courseId"`
	CourseName   string        `json:"courseName"`
	Notes        string        `json:"notes"`
	Participants []RoundPlayer `json:"participants"`
}

type RoundEntry struct {
	PlayerID     int64    `json:"playerId"`
	TeeID        int64    `json:"teeId"`
	Scores       [18]int  `json:"scores"`
	HandicapUsed *float64 `json:"handicapUsed"`
}

type CreateRoundInput struct {
	PlayedOn     string       `json:"playedOn"`
	CourseID     int64        `json:"courseId"`
	Notes        string       `json:"notes"`
	Participants []RoundEntry `json:"participants"`
}

const schema = `
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS players (
	id INTEGER PRIMARY KEY,
	name TEXT NOT NULL COLLATE NOCASE UNIQUE,
	starting_course_handicap INTEGER,
	official_handicap_index REAL,
	official_handicap_date TEXT,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS courses (
	id INTEGER PRIMARY KEY,
	name TEXT NOT NULL COLLATE NOCASE UNIQUE,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tees (
	id INTEGER PRIMARY KEY,
	course_id INTEGER NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
	name TEXT NOT NULL COLLATE NOCASE,
	rating REAL NOT NULL,
	slope INTEGER NOT NULL,
	par_json TEXT NOT NULL,
	stroke_index_json TEXT NOT NULL,
	UNIQUE(course_id, name)
);

CREATE TABLE IF NOT EXISTS rounds (
	id INTEGER PRIMARY KEY,
	played_on TEXT NOT NULL,
	course_id INTEGER NOT NULL REFERENCES courses(id),
	notes TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS round_players (
	id INTEGER PRIMARY KEY,
	round_id INTEGER NOT NULL REFERENCES rounds(id) ON DELETE CASCADE,
	player_id INTEGER NOT NULL REFERENCES players(id),
	tee_id INTEGER NOT NULL REFERENCES tees(id),
	handicap_used REAL,
	course_handicap INTEGER NOT NULL DEFAULT 0,
	adjusted_gross INTEGER NOT NULL DEFAULT 0,
	score_differential REAL NOT NULL DEFAULT 0,
	handicap_index_after REAL,
	starting_handicap_used INTEGER NOT NULL DEFAULT 0,
	initial_par_five_cap_used INTEGER NOT NULL DEFAULT 0,
	UNIQUE(round_id, player_id)
);

CREATE TABLE IF NOT EXISTS hole_scores (
	round_player_id INTEGER NOT NULL REFERENCES round_players(id) ON DELETE CASCADE,
	hole_number INTEGER NOT NULL CHECK(hole_number BETWEEN 1 AND 18),
	strokes INTEGER NOT NULL CHECK(strokes BETWEEN 0 AND 30),
	PRIMARY KEY(round_player_id, hole_number)
);

CREATE INDEX IF NOT EXISTS idx_rounds_played_on ON rounds(played_on DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_round_players_player ON round_players(player_id);
`

func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize database: %w", err)
	}
	if err := ensureColumn(db, "round_players", "handicap_used", "REAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func ensureColumn(db *sql.DB, table, column, definition string) error {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		if name == column {
			found = true
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	_, err = db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + definition)
	return err
}

func (s *Store) Empty(ctx context.Context) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM players) + (SELECT COUNT(*) FROM courses) + (SELECT COUNT(*) FROM rounds)`).Scan(&count)
	return count == 0, err
}

func (s *Store) Players(ctx context.Context) ([]Player, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.name, p.starting_course_handicap, p.official_handicap_index,
		       p.official_handicap_date, COUNT(rp.id),
		       (SELECT rp2.handicap_index_after
		          FROM round_players rp2 JOIN rounds r2 ON r2.id = rp2.round_id
		         WHERE rp2.player_id = p.id AND rp2.handicap_index_after IS NOT NULL
		         ORDER BY r2.played_on DESC, r2.id DESC LIMIT 1)
		  FROM players p
		  LEFT JOIN round_players rp ON rp.player_id = p.id
		 GROUP BY p.id
		 ORDER BY p.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	players := []Player{}
	for rows.Next() {
		var p Player
		var starting sql.NullInt64
		var official, group sql.NullFloat64
		var officialDate sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &starting, &official, &officialDate, &p.RoundCount, &group); err != nil {
			return nil, err
		}
		p.StartingCourseHandicap = intPtr(starting)
		p.OfficialHandicapIndex = floatPtr(official)
		p.OfficialHandicapDate = stringPtr(officialDate)
		p.GroupHandicapIndex = floatPtr(group)
		players = append(players, p)
	}
	return players, rows.Err()
}

func (s *Store) Player(ctx context.Context, id int64) (Player, []Round, error) {
	players, err := s.Players(ctx)
	if err != nil {
		return Player{}, nil, err
	}
	var found *Player
	for i := range players {
		if players[i].ID == id {
			found = &players[i]
			break
		}
	}
	if found == nil {
		return Player{}, nil, sql.ErrNoRows
	}
	rounds, err := s.rounds(ctx, &id, 100)
	return *found, rounds, err
}

func (s *Store) CreatePlayer(ctx context.Context, name string, starting *int) (Player, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Player{}, fmt.Errorf("player name is required")
	}
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO players(name, starting_course_handicap) VALUES(?, ?)`, name, starting)
	if err != nil {
		return Player{}, friendlyConstraint(err, "a player with that name already exists")
	}
	id, _ := result.LastInsertId()
	player, _, err := s.Player(ctx, id)
	return player, err
}

func (s *Store) UpdatePlayer(ctx context.Context, id int64, name string, starting *int, official *float64, officialDate *string) (Player, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Player{}, fmt.Errorf("player name is required")
	}
	if official != nil && (*official < -10 || *official > 54) {
		return Player{}, fmt.Errorf("official Handicap Index must be between -10 and 54")
	}
	if officialDate != nil {
		if _, err := time.Parse("2006-01-02", *officialDate); err != nil {
			return Player{}, fmt.Errorf("official handicap date must be YYYY-MM-DD")
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Player{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE players
		SET name = ?, starting_course_handicap = ?, official_handicap_index = ?, official_handicap_date = ?
		WHERE id = ?`, name, starting, official, officialDate, id)
	if err != nil {
		return Player{}, friendlyConstraint(err, "a player with that name already exists")
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return Player{}, sql.ErrNoRows
	}
	if err := s.recalculatePlayerTx(ctx, tx, id); err != nil {
		return Player{}, err
	}
	if err := tx.Commit(); err != nil {
		return Player{}, err
	}
	player, _, err := s.Player(ctx, id)
	return player, err
}

func (s *Store) DeletePlayer(ctx context.Context, id int64) error {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM round_players WHERE player_id = ?`, id).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("player has recorded rounds and cannot be deleted")
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM players WHERE id = ?`, id)
	if err == nil {
		if n, _ := result.RowsAffected(); n == 0 {
			return sql.ErrNoRows
		}
	}
	return err
}

func (s *Store) Courses(ctx context.Context) ([]Course, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.name, t.id, t.name, t.rating, t.slope, t.par_json, t.stroke_index_json
		  FROM courses c LEFT JOIN tees t ON t.course_id = c.id
		 ORDER BY c.name, t.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[int64]*Course{}
	var order []int64
	for rows.Next() {
		var courseID int64
		var courseName string
		var teeID sql.NullInt64
		var teeName, parJSON, strokeJSON sql.NullString
		var rating sql.NullFloat64
		var slope sql.NullInt64
		if err := rows.Scan(&courseID, &courseName, &teeID, &teeName, &rating, &slope, &parJSON, &strokeJSON); err != nil {
			return nil, err
		}
		course := byID[courseID]
		if course == nil {
			course = &Course{ID: courseID, Name: courseName, Tees: []Tee{}}
			byID[courseID] = course
			order = append(order, courseID)
		}
		if teeID.Valid {
			tee := Tee{ID: teeID.Int64, CourseID: courseID, CourseName: courseName, Name: teeName.String, Rating: rating.Float64, Slope: int(slope.Int64)}
			if err := json.Unmarshal([]byte(parJSON.String), &tee.Par); err != nil {
				return nil, err
			}
			if err := json.Unmarshal([]byte(strokeJSON.String), &tee.StrokeIndex); err != nil {
				return nil, err
			}
			tee.TotalPar = handicap.TotalPar(tee.Par)
			course.Tees = append(course.Tees, tee)
		}
	}
	courses := make([]Course, 0, len(order))
	for _, id := range order {
		courses = append(courses, *byID[id])
	}
	return courses, rows.Err()
}

func (s *Store) CreateTee(ctx context.Context, course handicap.Course) (Tee, error) {
	course.Name = strings.TrimSpace(course.Name)
	course.Tee = strings.TrimSpace(course.Tee)
	if err := course.Validate(); err != nil {
		return Tee{}, err
	}
	parJSON, _ := json.Marshal(course.Par)
	strokeJSON, _ := json.Marshal(course.StrokeIndex)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Tee{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO courses(name) VALUES(?)
		ON CONFLICT(name) DO NOTHING`, strings.TrimSpace(course.Name)); err != nil {
		return Tee{}, err
	}
	var courseID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM courses WHERE name = ?`, course.Name).Scan(&courseID); err != nil {
		return Tee{}, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO tees(course_id, name, rating, slope, par_json, stroke_index_json)
		VALUES(?, ?, ?, ?, ?, ?)`, courseID, course.Tee, course.Rating, course.Slope, parJSON, strokeJSON)
	if err != nil {
		return Tee{}, friendlyConstraint(err, "that tee already exists for this course")
	}
	teeID, _ := result.LastInsertId()
	if err := tx.Commit(); err != nil {
		return Tee{}, err
	}
	return Tee{ID: teeID, CourseID: courseID, CourseName: course.Name, Name: course.Tee, Rating: course.Rating,
		Slope: course.Slope, Par: course.Par, StrokeIndex: course.StrokeIndex, TotalPar: handicap.TotalPar(course.Par)}, nil
}

func (s *Store) UpdateCourse(ctx context.Context, id int64, name string) (Course, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Course{}, fmt.Errorf("course name is required")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE courses SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		return Course{}, friendlyConstraint(err, "a course with that name already exists")
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return Course{}, sql.ErrNoRows
	}
	courses, err := s.Courses(ctx)
	if err != nil {
		return Course{}, err
	}
	for _, course := range courses {
		if course.ID == id {
			return course, nil
		}
	}
	return Course{}, sql.ErrNoRows
}

func (s *Store) UpdateTee(ctx context.Context, id int64, input handicap.Course) (Tee, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Tee{}, err
	}
	defer tx.Rollback()

	var courseID int64
	if err := tx.QueryRowContext(ctx, `SELECT course_id FROM tees WHERE id = ?`, id).Scan(&courseID); err != nil {
		return Tee{}, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT name FROM courses WHERE id = ?`, courseID).Scan(&input.Name); err != nil {
		return Tee{}, err
	}
	input.Tee = strings.TrimSpace(input.Tee)
	if err := input.Validate(); err != nil {
		return Tee{}, err
	}
	parJSON, _ := json.Marshal(input.Par)
	strokeJSON, _ := json.Marshal(input.StrokeIndex)

	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT player_id FROM round_players WHERE tee_id = ?`, id)
	if err != nil {
		return Tee{}, err
	}
	var playerIDs []int64
	for rows.Next() {
		var playerID int64
		if err := rows.Scan(&playerID); err != nil {
			rows.Close()
			return Tee{}, err
		}
		playerIDs = append(playerIDs, playerID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Tee{}, err
	}
	if err := rows.Close(); err != nil {
		return Tee{}, err
	}

	_, err = tx.ExecContext(ctx, `UPDATE tees
		SET name = ?, rating = ?, slope = ?, par_json = ?, stroke_index_json = ?
		WHERE id = ?`, input.Tee, input.Rating, input.Slope, parJSON, strokeJSON, id)
	if err != nil {
		return Tee{}, friendlyConstraint(err, "that tee already exists for this course")
	}
	for _, playerID := range playerIDs {
		if err := s.recalculatePlayerTx(ctx, tx, playerID); err != nil {
			return Tee{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Tee{}, err
	}

	courses, err := s.Courses(ctx)
	if err != nil {
		return Tee{}, err
	}
	for _, course := range courses {
		for _, tee := range course.Tees {
			if tee.ID == id {
				return tee, nil
			}
		}
	}
	return Tee{}, sql.ErrNoRows
}

func (s *Store) DeleteTee(ctx context.Context, id int64) error {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM round_players WHERE tee_id = ?`, id).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("tee has recorded rounds and cannot be deleted")
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM tees WHERE id = ?`, id)
	if err == nil {
		if n, _ := result.RowsAffected(); n == 0 {
			return sql.ErrNoRows
		}
	}
	return err
}

func (s *Store) Rounds(ctx context.Context, limit int) ([]Round, error) {
	return s.rounds(ctx, nil, limit)
}

func (s *Store) Round(ctx context.Context, id int64) (Round, error) {
	rounds, err := s.loadRounds(ctx, `WHERE r.id = ?`, id)
	if err != nil {
		return Round{}, err
	}
	if len(rounds) == 0 {
		return Round{}, sql.ErrNoRows
	}
	return rounds[0], nil
}

func (s *Store) rounds(ctx context.Context, playerID *int64, limit int) ([]Round, error) {
	if limit <= 0 {
		limit = 50
	}
	if playerID != nil {
		return s.loadRounds(ctx, `WHERE r.id IN (
			SELECT r0.id FROM rounds r0
			JOIN round_players filter_rp ON filter_rp.round_id = r0.id
			WHERE filter_rp.player_id = ?
			ORDER BY r0.played_on DESC, r0.id DESC LIMIT ?
		) ORDER BY r.played_on DESC, r.id DESC`, *playerID, limit)
	}
	return s.loadRounds(ctx, `WHERE r.id IN (
		SELECT id FROM rounds ORDER BY played_on DESC, id DESC LIMIT ?
	) ORDER BY r.played_on DESC, r.id DESC`, limit)
}

func (s *Store) loadRounds(ctx context.Context, clause string, args ...any) ([]Round, error) {
	query := `SELECT r.id, r.played_on, r.course_id, c.name, r.notes,
		rp.id, p.id, p.name, t.id, t.name, rp.handicap_used, rp.course_handicap, rp.adjusted_gross,
		rp.score_differential, rp.handicap_index_after, rp.starting_handicap_used,
		rp.initial_par_five_cap_used, t.stroke_index_json
		FROM rounds r
		JOIN courses c ON c.id = r.course_id
		LEFT JOIN round_players rp ON rp.round_id = r.id
		LEFT JOIN players p ON p.id = rp.player_id
		LEFT JOIN tees t ON t.id = rp.tee_id ` + clause
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	roundMap := map[int64]*Round{}
	var order []int64
	var participantIDs []int64
	type participantPosition struct {
		roundID int64
		index   int
	}
	participantPositions := map[int64]participantPosition{}
	participantStrokeIndexes := map[int64][18]int{}
	for rows.Next() {
		var r Round
		var rpID, playerID, teeID sql.NullInt64
		var playerName, teeName, strokeIndexJSON sql.NullString
		var ch, adjusted sql.NullInt64
		var handicapUsed, differential sql.NullFloat64
		var index sql.NullFloat64
		var startingUsed, initialUsed sql.NullBool
		if err := rows.Scan(&r.ID, &r.PlayedOn, &r.CourseID, &r.CourseName, &r.Notes,
			&rpID, &playerID, &playerName, &teeID, &teeName, &handicapUsed, &ch, &adjusted,
			&differential, &index, &startingUsed, &initialUsed, &strokeIndexJSON); err != nil {
			return nil, err
		}
		current := roundMap[r.ID]
		if current == nil {
			r.Participants = []RoundPlayer{}
			current = &r
			roundMap[r.ID] = current
			order = append(order, r.ID)
		}
		if rpID.Valid {
			rp := RoundPlayer{ID: rpID.Int64, PlayerID: playerID.Int64, PlayerName: playerName.String,
				TeeID: teeID.Int64, TeeName: teeName.String, HandicapUsed: floatPtr(handicapUsed),
				CourseHandicap: int(ch.Int64),
				AdjustedGross:  int(adjusted.Int64), ScoreDifferential: differential.Float64,
				HandicapIndexAfter: floatPtr(index), StartingHandicapUsed: startingUsed.Bool,
				InitialParFiveCapUsed: initialUsed.Bool}
			current.Participants = append(current.Participants, rp)
			participantPositions[rp.ID] = participantPosition{roundID: r.ID, index: len(current.Participants) - 1}
			participantIDs = append(participantIDs, rp.ID)
			var strokeIndexes [18]int
			if err := json.Unmarshal([]byte(strokeIndexJSON.String), &strokeIndexes); err != nil {
				return nil, err
			}
			participantStrokeIndexes[rp.ID] = strokeIndexes
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range participantIDs {
		position := participantPositions[id]
		rp := &roundMap[position.roundID].Participants[position.index]
		scoreRows, err := s.db.QueryContext(ctx, `SELECT hole_number, strokes FROM hole_scores WHERE round_player_id = ? ORDER BY hole_number`, id)
		if err != nil {
			return nil, err
		}
		for scoreRows.Next() {
			var hole, strokes int
			if err := scoreRows.Scan(&hole, &strokes); err != nil {
				scoreRows.Close()
				return nil, err
			}
			rp.Scores[hole-1] = strokes
			rp.Gross += strokes
		}
		scoreRows.Close()
		if rp.HandicapUsed != nil {
			net := float64(rp.Gross) - *rp.HandicapUsed
			rp.NetScore = &net
			netScores := handicap.NetScores(rp.Scores, participantStrokeIndexes[id], *rp.HandicapUsed)
			rp.NetScores = &netScores
		}
	}
	rounds := make([]Round, 0, len(order))
	for _, id := range order {
		rounds = append(rounds, *roundMap[id])
	}
	if err := s.markCounting(ctx, rounds); err != nil {
		return nil, err
	}
	return rounds, nil
}

func (s *Store) markCounting(ctx context.Context, rounds []Round) error {
	playerIDs := map[int64]bool{}
	for _, round := range rounds {
		for _, participant := range round.Participants {
			playerIDs[participant.PlayerID] = true
		}
	}

	countingIDs := map[int64]bool{}
	for playerID := range playerIDs {
		rows, err := s.db.QueryContext(ctx, `SELECT rp.id, rp.score_differential
			FROM round_players rp
			JOIN rounds r ON r.id = rp.round_id
			WHERE rp.player_id = ?
			ORDER BY r.played_on DESC, r.id DESC
			LIMIT 20`, playerID)
		if err != nil {
			return err
		}
		type candidate struct {
			id           int64
			differential float64
		}
		var candidates []candidate
		for rows.Next() {
			var item candidate
			if err := rows.Scan(&item.id, &item.differential); err != nil {
				rows.Close()
				return err
			}
			candidates = append(candidates, item)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if len(candidates) < handicap.QualifyingRounds {
			continue
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			return candidates[i].differential < candidates[j].differential
		})
		use, _ := handicap.LowScoreCountTable(len(candidates))
		for i := 0; i < use; i++ {
			countingIDs[candidates[i].id] = true
		}
	}

	for roundIndex := range rounds {
		for participantIndex := range rounds[roundIndex].Participants {
			participant := &rounds[roundIndex].Participants[participantIndex]
			participant.Counting = countingIDs[participant.ID]
		}
	}
	return nil
}

func (s *Store) CreateRound(ctx context.Context, input CreateRoundInput) (Round, error) {
	seenPlayers, err := validateRoundInput(input)
	if err != nil {
		return Round{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Round{}, err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM courses WHERE id = ?`, input.CourseID).Scan(&exists); err != nil || exists == 0 {
		return Round{}, fmt.Errorf("course not found")
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO rounds(played_on, course_id, notes) VALUES(?, ?, ?)`,
		input.PlayedOn, input.CourseID, strings.TrimSpace(input.Notes))
	if err != nil {
		return Round{}, err
	}
	roundID, _ := result.LastInsertId()
	if err := insertRoundPlayers(ctx, tx, roundID, input); err != nil {
		return Round{}, err
	}
	for playerID := range seenPlayers {
		if err := s.recalculatePlayerTx(ctx, tx, playerID); err != nil {
			return Round{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Round{}, err
	}
	return s.Round(ctx, roundID)
}

func (s *Store) UpdateRound(ctx context.Context, id int64, input CreateRoundInput) (Round, error) {
	newPlayers, err := validateRoundInput(input)
	if err != nil {
		return Round{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Round{}, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT player_id FROM round_players WHERE round_id = ?`, id)
	if err != nil {
		return Round{}, err
	}
	affectedPlayers := map[int64]bool{}
	for rows.Next() {
		var playerID int64
		if err := rows.Scan(&playerID); err != nil {
			rows.Close()
			return Round{}, err
		}
		affectedPlayers[playerID] = true
	}
	rows.Close()
	result, err := tx.ExecContext(ctx, `UPDATE rounds SET played_on = ?, course_id = ?, notes = ? WHERE id = ?`,
		input.PlayedOn, input.CourseID, strings.TrimSpace(input.Notes), id)
	if err != nil {
		return Round{}, err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return Round{}, sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM round_players WHERE round_id = ?`, id); err != nil {
		return Round{}, err
	}
	if err := insertRoundPlayers(ctx, tx, id, input); err != nil {
		return Round{}, err
	}
	for playerID := range newPlayers {
		affectedPlayers[playerID] = true
	}
	for playerID := range affectedPlayers {
		if err := s.recalculatePlayerTx(ctx, tx, playerID); err != nil {
			return Round{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Round{}, err
	}
	return s.Round(ctx, id)
}

func validateRoundInput(input CreateRoundInput) (map[int64]bool, error) {
	if _, err := time.Parse("2006-01-02", input.PlayedOn); err != nil {
		return nil, fmt.Errorf("playedOn must be YYYY-MM-DD")
	}
	if len(input.Participants) == 0 {
		return nil, fmt.Errorf("at least one player is required")
	}
	seenPlayers := map[int64]bool{}
	for _, entry := range input.Participants {
		if seenPlayers[entry.PlayerID] {
			return nil, fmt.Errorf("a player cannot appear twice in one round")
		}
		seenPlayers[entry.PlayerID] = true
		for _, score := range entry.Scores {
			if score < 0 || score > 30 {
				return nil, fmt.Errorf("hole scores must be between 0 and 30")
			}
		}
		if entry.HandicapUsed != nil && (*entry.HandicapUsed < -20 || *entry.HandicapUsed > 80) {
			return nil, fmt.Errorf("handicap used must be between -20 and 80")
		}
	}
	return seenPlayers, nil
}

func insertRoundPlayers(ctx context.Context, tx *sql.Tx, roundID int64, input CreateRoundInput) error {
	for _, entry := range input.Participants {
		var teeCourseID int64
		if err := tx.QueryRowContext(ctx, `SELECT course_id FROM tees WHERE id = ?`, entry.TeeID).Scan(&teeCourseID); err != nil {
			return fmt.Errorf("tee not found")
		}
		if teeCourseID != input.CourseID {
			return fmt.Errorf("selected tee does not belong to the round's course")
		}
		result, err := tx.ExecContext(ctx, `INSERT INTO round_players(round_id, player_id, tee_id, handicap_used) VALUES(?, ?, ?, ?)`,
			roundID, entry.PlayerID, entry.TeeID, entry.HandicapUsed)
		if err != nil {
			return friendlyConstraint(err, "player or tee not found")
		}
		roundPlayerID, _ := result.LastInsertId()
		for i, score := range entry.Scores {
			if _, err := tx.ExecContext(ctx, `INSERT INTO hole_scores(round_player_id, hole_number, strokes) VALUES(?, ?, ?)`,
				roundPlayerID, i+1, score); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) DeleteRound(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT player_id FROM round_players WHERE round_id = ?`, id)
	if err != nil {
		return err
	}
	var playerIDs []int64
	for rows.Next() {
		var playerID int64
		if err := rows.Scan(&playerID); err != nil {
			rows.Close()
			return err
		}
		playerIDs = append(playerIDs, playerID)
	}
	rows.Close()
	result, err := tx.ExecContext(ctx, `DELETE FROM rounds WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	for _, playerID := range playerIDs {
		if err := s.recalculatePlayerTx(ctx, tx, playerID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) RecalculateAll(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM players`)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if err := s.recalculatePlayerTx(ctx, tx, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) recalculatePlayerTx(ctx context.Context, tx *sql.Tx, playerID int64) error {
	var playerName string
	var starting sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT name, starting_course_handicap FROM players WHERE id = ?`, playerID).
		Scan(&playerName, &starting); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT rp.id, r.played_on, c.name, t.name, t.rating, t.slope, t.par_json, t.stroke_index_json
		  FROM round_players rp
		  JOIN rounds r ON r.id = rp.round_id
		  JOIN tees t ON t.id = rp.tee_id
		  JOIN courses c ON c.id = t.course_id
		 WHERE rp.player_id = ?
		 ORDER BY r.played_on, r.id`, playerID)
	if err != nil {
		return err
	}
	type record struct {
		roundPlayerID int64
		round         handicap.Round
		course        handicap.Course
	}
	var records []record
	for rows.Next() {
		var rec record
		var parJSON, strokeJSON string
		if err := rows.Scan(&rec.roundPlayerID, &rec.round.Date, &rec.course.Name, &rec.course.Tee,
			&rec.course.Rating, &rec.course.Slope, &parJSON, &strokeJSON); err != nil {
			rows.Close()
			return err
		}
		if err := json.Unmarshal([]byte(parJSON), &rec.course.Par); err != nil {
			rows.Close()
			return err
		}
		if err := json.Unmarshal([]byte(strokeJSON), &rec.course.StrokeIndex); err != nil {
			rows.Close()
			return err
		}
		rec.round.Player = playerName
		rec.round.CourseName = rec.course.Name
		rec.round.Tee = rec.course.Tee
		records = append(records, rec)
	}
	rows.Close()
	data := handicap.Data{StartingHandicaps: map[string]int{}}
	if starting.Valid {
		data.StartingHandicaps[playerName] = int(starting.Int64)
	}
	courseKeys := map[string]bool{}
	for i := range records {
		scoreRows, err := tx.QueryContext(ctx, `SELECT hole_number, strokes FROM hole_scores WHERE round_player_id = ? ORDER BY hole_number`,
			records[i].roundPlayerID)
		if err != nil {
			return err
		}
		count := 0
		for scoreRows.Next() {
			var hole, strokes int
			if err := scoreRows.Scan(&hole, &strokes); err != nil {
				scoreRows.Close()
				return err
			}
			records[i].round.Scores[hole-1] = strokes
			count++
		}
		scoreRows.Close()
		if count != 18 {
			return fmt.Errorf("round player %d has %d hole scores, expected 18", records[i].roundPlayerID, count)
		}
		if !courseKeys[records[i].course.Key()] {
			data.Courses = append(data.Courses, records[i].course)
			courseKeys[records[i].course.Key()] = true
		}
		data.Rounds = append(data.Rounds, records[i].round)
	}
	if err := handicap.RecalculatePlayerRounds(&data, playerName); err != nil {
		return err
	}
	for i, calculated := range data.Rounds {
		var index any
		if i+1 >= handicap.QualifyingRounds {
			index = calculated.EffectiveIndexAfter
		}
		_, err := tx.ExecContext(ctx, `UPDATE round_players
			SET course_handicap = ?, adjusted_gross = ?, score_differential = ?, handicap_index_after = ?,
			    starting_handicap_used = ?, initial_par_five_cap_used = ?
			WHERE id = ?`, calculated.CourseHandicapAt, calculated.AdjustedGrossAt, calculated.ScoreDifferential,
			index, starting.Valid && i < handicap.QualifyingRounds, !starting.Valid && i < handicap.QualifyingRounds,
			records[i].roundPlayerID)
		if err != nil {
			return err
		}
	}
	return nil
}

func intPtr(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
}

func floatPtr(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	return &v.Float64
}

func stringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

func friendlyConstraint(err error, message string) error {
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "constraint") {
		return errors.New(message)
	}
	return err
}

func SortRoundsChronologically(rounds []Round) {
	sort.SliceStable(rounds, func(i, j int) bool {
		if rounds[i].PlayedOn == rounds[j].PlayedOn {
			return rounds[i].ID < rounds[j].ID
		}
		return rounds[i].PlayedOn < rounds[j].PlayedOn
	})
}

package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"golf/internal/handicap"
)

func TestCreateRoundInputAcceptsDecimalHandicap(t *testing.T) {
	var input CreateRoundInput
	if err := json.Unmarshal([]byte(`{"participants":[{"handicapUsed":26.7}]}`), &input); err != nil {
		t.Fatalf("decimal handicap should be accepted: %v", err)
	}
	if got := input.Participants[0].HandicapUsed; got == nil || *got != 26.7 {
		t.Fatalf("handicap used = %v, want 26.7", got)
	}
}

func TestEmptyPlayerListIsAnEmptyCollection(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	players, err := store.Players(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if players == nil || len(players) != 0 {
		t.Fatalf("players = %#v, want an empty non-nil slice", players)
	}
}

func TestPlayerHandicapCategoryDefaultsToMenAndCanBeUpdated(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()

	player, err := store.CreatePlayer(ctx, "Frank", nil)
	if err != nil {
		t.Fatal(err)
	}
	if player.HandicapCategory != handicap.Men {
		t.Fatalf("default handicap category = %q, want %q", player.HandicapCategory, handicap.Men)
	}

	category := handicap.Women
	player, err = store.UpdatePlayer(ctx, player.ID, player.Name, &category, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if player.HandicapCategory != handicap.Women {
		t.Fatalf("updated handicap category = %q, want %q", player.HandicapCategory, handicap.Women)
	}

	invalid := handicap.HandicapCategory("other")
	if _, err := store.UpdatePlayer(ctx, player.ID, player.Name, &invalid, nil, nil); err == nil {
		t.Fatal("invalid handicap category should be rejected")
	}
}

func TestOpenRemovesLegacyStartingHandicapsAndRecalculates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	course := handicap.Course{Name: "Test Club", Tee: "White", Rating: 72, Slope: 113}
	for i := range course.Par {
		course.Par[i] = 4
		course.StrokeIndex[i] = i + 1
	}
	tee, err := store.CreateTee(ctx, course)
	if err != nil {
		t.Fatal(err)
	}
	player, err := store.CreatePlayer(ctx, "Frank", nil)
	if err != nil {
		t.Fatal(err)
	}
	scores := [18]int{}
	for i := range scores {
		scores[i] = 9
	}
	round, err := store.CreateRound(ctx, CreateRoundInput{
		PlayedOn:     "2026-07-30",
		CourseID:     tee.CourseID,
		Participants: []RoundEntry{{PlayerID: player.ID, TeeID: tee.ID, Scores: scores}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := round.Participants[0].AdjustedGross; got != 162 {
		t.Fatalf("initial adjusted gross = %d, want 162 under Par+5", got)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE players SET starting_course_handicap = 30`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE round_players
		SET daily_handicap = 30, adjusted_gross = 138, starting_handicap_used = 1, initial_par_five_cap_used = 0`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE app_metadata SET value = 'old' WHERE key = 'handicap_formula_version'`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	round, err = store.Round(ctx, round.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := round.Participants[0].DailyHandicap; got != 0 {
		t.Fatalf("migrated Daily Handicap = %d, want 0", got)
	}
	if got := round.Participants[0].AdjustedGross; got != 162 {
		t.Fatalf("migrated adjusted gross = %d, want 162 under Par+5", got)
	}
	if !round.Participants[0].InitialParFiveCapUsed {
		t.Fatal("migrated round should be marked as using the initial Par+5 cap")
	}
	var legacyStarting sql.NullInt64
	if err := store.db.QueryRowContext(ctx, `SELECT starting_course_handicap FROM players WHERE id = ?`, player.ID).Scan(&legacyStarting); err != nil {
		t.Fatal(err)
	}
	if legacyStarting.Valid {
		t.Fatalf("legacy starting handicap was not cleared: %d", legacyStarting.Int64)
	}
	var version string
	if err := store.db.QueryRowContext(ctx,
		`SELECT value FROM app_metadata WHERE key = 'handicap_formula_version'`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != handicapFormulaVersion {
		t.Fatalf("formula version = %q, want %q", version, handicapFormulaVersion)
	}
}

func TestMultiPlayerRoundLoadsEveryScorecardAndEstablishesIndexes(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()

	course := handicap.Course{Name: "Test Club", Tee: "White", Rating: 72, Slope: 113}
	for i := 0; i < 18; i++ {
		course.Par[i] = 4
		course.StrokeIndex[i] = i + 1
	}
	tee, err := store.CreateTee(ctx, course)
	if err != nil {
		t.Fatal(err)
	}

	var players []Player
	for _, name := range []string{"Frank", "Grahame", "Shane"} {
		player, err := store.CreatePlayer(ctx, name, nil)
		if err != nil {
			t.Fatal(err)
		}
		players = append(players, player)
	}

	var last Round
	for roundNumber, date := range []string{"2026-01-01", "2026-01-08", "2026-01-15"} {
		input := CreateRoundInput{PlayedOn: date, CourseID: tee.CourseID}
		for playerIndex, player := range players {
			var scores [18]int
			for hole := range scores {
				scores[hole] = 4 + playerIndex + roundNumber
			}
			entry := RoundEntry{PlayerID: player.ID, TeeID: tee.ID, Scores: scores}
			if playerIndex == 0 {
				handicapUsed := 12.5
				entry.HandicapUsed = &handicapUsed
			}
			input.Participants = append(input.Participants, entry)
		}
		last, err = store.CreateRound(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
	}

	if len(last.Participants) != 3 {
		t.Fatalf("participants = %d, want 3", len(last.Participants))
	}
	for i, participant := range last.Participants {
		wantScore := 6 + i
		wantGross := wantScore * 18
		if participant.Gross != wantGross {
			t.Errorf("%s gross = %d, want %d", participant.PlayerName, participant.Gross, wantGross)
		}
		for hole, score := range participant.Scores {
			if score != wantScore {
				t.Errorf("%s hole %d = %d, want %d", participant.PlayerName, hole+1, score, wantScore)
			}
		}
		if participant.HandicapIndexAfter == nil {
			t.Errorf("%s should have an established index after three rounds", participant.PlayerName)
		}
		if i == 0 && (participant.HandicapUsed == nil || participant.NetScore != float64(participant.Gross)-12.5) {
			t.Errorf("%s handicap/net score not loaded: %+v", participant.PlayerName, participant)
		}
		netHandicap := float64(participant.DailyHandicap)
		if participant.HandicapUsed != nil {
			netHandicap = *participant.HandicapUsed
		}
		if participant.NetScore != float64(participant.Gross)-netHandicap {
			t.Errorf("%s net score = %v, want %v", participant.PlayerName, participant.NetScore, float64(participant.Gross)-netHandicap)
		}
		wantNetScores := handicap.NetScores(participant.Scores, course.StrokeIndex, netHandicap)
		if participant.NetScores != wantNetScores {
			t.Errorf("%s net scores = %v, want %v", participant.PlayerName, participant.NetScores, wantNetScores)
		}
	}

	loadedPlayers, err := store.Players(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, player := range loadedPlayers {
		if player.RoundCount != 3 || player.GroupHandicapIndex == nil {
			t.Errorf("%s = %+v, want 3 rounds and an established index", player.Name, player)
		}
	}
	rounds, err := store.Rounds(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	countingByPlayer := map[int64]int{}
	for _, round := range rounds {
		for _, participant := range round.Participants {
			if participant.Counting {
				countingByPlayer[participant.PlayerID]++
			}
		}
	}
	for _, player := range players {
		if countingByPlayer[player.ID] != 1 {
			t.Errorf("%s has %d counting rounds, want 1", player.Name, countingByPlayer[player.ID])
		}
	}

	update := CreateRoundInput{PlayedOn: last.PlayedOn, CourseID: last.CourseID, Notes: "Corrected"}
	for _, participant := range last.Participants {
		scores := participant.Scores
		if participant.PlayerID == players[0].ID {
			scores[0] = 3
		}
		update.Participants = append(update.Participants, RoundEntry{
			PlayerID: participant.PlayerID, TeeID: participant.TeeID, Scores: scores, HandicapUsed: participant.HandicapUsed,
		})
	}
	updated, err := store.UpdateRound(ctx, last.ID, update)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Notes != "Corrected" || updated.Participants[0].Scores[0] != 3 {
		t.Errorf("updated round was not persisted: %+v", updated)
	}
}

func TestCourseAndTeeUpdatesPreserveRoundsAndRecalculate(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()

	course := handicap.Course{Name: "Old Club", Tee: "White", Rating: 72, Slope: 113}
	for i := 0; i < 18; i++ {
		course.Par[i] = 4
		course.StrokeIndex[i] = i + 1
	}
	tee, err := store.CreateTee(ctx, course)
	if err != nil {
		t.Fatal(err)
	}
	player, err := store.CreatePlayer(ctx, "Frank", nil)
	if err != nil {
		t.Fatal(err)
	}
	scores := [18]int{}
	for i := range scores {
		scores[i] = 5
	}
	round, err := store.CreateRound(ctx, CreateRoundInput{
		PlayedOn: "2026-07-24", CourseID: tee.CourseID,
		Participants: []RoundEntry{{PlayerID: player.ID, TeeID: tee.ID, Scores: scores}},
	})
	if err != nil {
		t.Fatal(err)
	}
	originalDifferential := round.Participants[0].ScoreDifferential

	updatedCourse, err := store.UpdateCourse(ctx, tee.CourseID, "New Club")
	if err != nil {
		t.Fatal(err)
	}
	if updatedCourse.Name != "New Club" {
		t.Fatalf("course name = %q, want New Club", updatedCourse.Name)
	}

	course.Name = "New Club"
	course.Tee = "Gold"
	course.Rating = 70
	updatedTee, err := store.UpdateTee(ctx, tee.ID, course)
	if err != nil {
		t.Fatal(err)
	}
	if updatedTee.Name != "Gold" || updatedTee.Rating != 70 {
		t.Fatalf("updated tee = %+v", updatedTee)
	}

	loaded, err := store.Round(ctx, round.ID)
	if err != nil {
		t.Fatal(err)
	}
	participant := loaded.Participants[0]
	if loaded.CourseName != "New Club" || participant.TeeName != "Gold" {
		t.Fatalf("round references were not preserved: %+v", loaded)
	}
	if participant.ScoreDifferential == originalDifferential {
		t.Fatalf("differential was not recalculated after tee rating changed")
	}
	wantDifferential := handicap.ScoreDifferential(participant.AdjustedGross, 70, 113)
	if participant.ScoreDifferential != wantDifferential {
		t.Fatalf("differential = %v, want %v", participant.ScoreDifferential, wantDifferential)
	}
}

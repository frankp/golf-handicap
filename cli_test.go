package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golf/internal/database"
)

// buildCLI compiles the golf binary once into t's temp dir and returns its
// path. Every test that needs the real binary calls this - it never
// touches the developer's working-directory golf.db, because each
// call runs the binary with its own GOLF_DB env var pointing at a
// t.TempDir() path.
func buildCLI(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "golf-under-test")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build CLI: %v\n%s", err, out)
	}
	return bin
}

// run executes the built binary with the given args and stdin, against an
// isolated data file, and returns combined stdout+stderr.
func run(t *testing.T, bin, dataFile, stdin string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(cmd.Env, "GOLF_DB="+dataFile)
	cmd.Stdin = strings.NewReader(stdin)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("running %v failed: %v\noutput:\n%s", args, err, out.String())
	}
	return out.String()
}

const testCourseInput = "Test Golf Club\nWhite\n71.5\n128\n" +
	"4\n7\n5\n11\n4\n1\n3\n17\n4\n5\n5\n9\n4\n13\n3\n15\n4\n3\n5\n18\n" +
	"4\n8\n4\n2\n3\n16\n5\n6\n4\n10\n4\n12\n3\n14\n5\n4\n"

func TestCLIEndToEnd(t *testing.T) {
	bin := buildCLI(t)
	dataFile := filepath.Join(t.TempDir(), "golf.db")

	run(t, bin, dataFile, testCourseInput, "course", "add")

	courses := run(t, bin, dataFile, "", "course", "list")
	if !strings.Contains(courses, "Test Golf Club") {
		t.Errorf("course list missing the added course, got:\n%s", courses)
	}

	// Frank's rounds: gross 91 on CR 71.5 / Slope 128 -> differential
	// (113/128) x (91 - 71.5) = 17.2. Three scores establish an initial
	// Handicap Index of 15.2.
	roundInput := "5\n8\n6\n6\n5\n2\n4\n6\n5\n6\n5\n7\n4\n5\n4\n4\n5\n4\n"
	roundOut := run(t, bin, dataFile, roundInput,
		"round", "add", "--player", "Frank", "--course", "Test Golf Club", "--tee", "White", "--date", "2026-07-01")
	if !strings.Contains(roundOut, "Score Differential 17.2") {
		t.Errorf("round add output missing expected differential, got:\n%s", roundOut)
	}
	if strings.Contains(roundOut, "Handicap Index:") {
		t.Errorf("first round should not establish a Handicap Index, got:\n%s", roundOut)
	}
	run(t, bin, dataFile, roundInput,
		"round", "add", "--player", "Frank", "--course", "Test Golf Club", "--tee", "White", "--date", "2026-07-08")
	roundOut = run(t, bin, dataFile, roundInput,
		"round", "add", "--player", "Frank", "--course", "Test Golf Club", "--tee", "White", "--date", "2026-07-15")
	if !strings.Contains(roundOut, "Handicap Index: 15.2") {
		t.Errorf("round add output missing expected handicap index, got:\n%s", roundOut)
	}

	indexOut := run(t, bin, dataFile, "", "index", "--player", "Frank")
	if !strings.Contains(indexOut, "15.2") {
		t.Errorf("index output = %q, want it to contain 15.2", indexOut)
	}

	handicapOut := run(t, bin, dataFile, "", "handicap", "--player", "Frank", "Test Golf Club", "White")
	if !strings.Contains(handicapOut, "Course Handicap 16") {
		t.Errorf("handicap output = %q, want Course Handicap 16", handicapOut)
	}
}

func TestCLIMultiplePlayersAreIndependent(t *testing.T) {
	bin := buildCLI(t)
	dataFile := filepath.Join(t.TempDir(), "golf.db")

	run(t, bin, dataFile, testCourseInput, "course", "add")

	frankRound := "5\n8\n6\n6\n5\n2\n4\n6\n5\n6\n5\n7\n4\n5\n4\n4\n5\n4\n" // gross 91
	for _, date := range []string{"2026-07-01", "2026-07-08", "2026-07-15"} {
		run(t, bin, dataFile, frankRound,
			"round", "add", "--player", "Frank", "--course", "Test Golf Club", "--tee", "White", "--date", date)
	}

	alexRound := "4\n5\n4\n4\n4\n5\n4\n3\n5\n5\n4\n4\n3\n5\n4\n4\n3\n5\n" // gross 75
	for _, date := range []string{"2026-07-01", "2026-07-08", "2026-07-15"} {
		run(t, bin, dataFile, alexRound,
			"round", "add", "--player", "Alex", "--course", "Test Golf Club", "--tee", "White", "--date", date)
	}

	players := run(t, bin, dataFile, "", "player", "list")
	if !strings.Contains(players, "Frank") || !strings.Contains(players, "15.2") {
		t.Errorf("player list missing Frank's index, got:\n%s", players)
	}
	if !strings.Contains(players, "Alex") || !strings.Contains(players, "1.1") {
		t.Errorf("player list missing Alex's index, got:\n%s", players)
	}

	frankIndex := run(t, bin, dataFile, "", "index", "--player", "Frank")
	if !strings.Contains(frankIndex, "15.2") {
		t.Errorf("Frank's index = %q, want it to contain 15.2", frankIndex)
	}
	alexIndex := run(t, bin, dataFile, "", "index", "--player", "Alex")
	if !strings.Contains(alexIndex, "1.1") {
		t.Errorf("Alex's index = %q, want it to contain 1.1", alexIndex)
	}
}

func TestCLIRoundDelete(t *testing.T) {
	bin := buildCLI(t)
	dataFile := filepath.Join(t.TempDir(), "golf.db")

	run(t, bin, dataFile, testCourseInput, "course", "add")

	goodRound := "5\n8\n6\n6\n5\n2\n4\n6\n5\n6\n5\n7\n4\n5\n4\n4\n5\n4\n"
	run(t, bin, dataFile, goodRound,
		"round", "add", "--player", "Frank", "--course", "Test Golf Club", "--tee", "White", "--date", "2026-07-01")

	mistakeRound := strings.Repeat("9\n", 18) // fat-fingered round to be deleted
	run(t, bin, dataFile, mistakeRound,
		"round", "add", "--player", "Frank", "--course", "Test Golf Club", "--tee", "White", "--date", "2026-07-08")

	// Declining the confirmation must leave both rounds in place.
	declineOut := run(t, bin, dataFile, "n\n", "round", "delete", "2")
	if !strings.Contains(declineOut, "Cancelled") {
		t.Errorf("expected cancellation, got:\n%s", declineOut)
	}
	if list := run(t, bin, dataFile, "", "round", "list"); strings.Count(list, "Frank") != 2 {
		t.Errorf("round should not have been deleted after declining, got:\n%s", list)
	}

	// Confirming deletes round #2 and leaves round #1 (and its differential) untouched.
	deleteOut := run(t, bin, dataFile, "y\n", "round", "delete", "2")
	if !strings.Contains(deleteOut, "Deleted round #2") {
		t.Errorf("expected deletion confirmation, got:\n%s", deleteOut)
	}
	list := run(t, bin, dataFile, "", "round", "list")
	if strings.Count(list, "Frank") != 1 {
		t.Errorf("expected exactly 1 remaining round, got:\n%s", list)
	}
	if !strings.Contains(list, "diff  17.2") {
		t.Errorf("remaining round's differential should be unchanged, got:\n%s", list)
	}

	// --yes skips the prompt entirely.
	yesOut := run(t, bin, dataFile, "", "round", "delete", "1", "--yes")
	if !strings.Contains(yesOut, "Deleted round #1") {
		t.Errorf("expected --yes to delete without a prompt, got:\n%s", yesOut)
	}

	// Out-of-range index is a clean error, not a panic.
	cmd := exec.Command(bin, "round", "delete", "99", "--yes")
	cmd.Env = append(cmd.Env, "GOLF_DB="+dataFile)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Errorf("expected an error deleting an out-of-range round, got output:\n%s", out)
	}
}

// TestCLIRoundDeleteRecalculatesLaterRounds proves that deleting a round
// doesn't just remove it - it replays the player's remaining rounds so a
// later round reads exactly as if the deleted one had never been added.
//
// Course pars (from testCourseInput) are 4,5,4,3,4,5,4,3,4,5,4,4,3,5,4,4,3,5.
// The target round is played at par on every hole except hole 15 (par 4, SI 10),
// which is scored 7 - one over the "no stroke" Net Double Bogey cap of 6,
// but exactly at the "one stroke" cap of 7. So whether that hole gets
// capped depends entirely on whether Round 3's Course Handicap reaches
// SI 10:
//
//   - With the mistaken round present, Frank's index going into the target
//     is driven by its very low differential -> Course Handicap 1,
//     no stroke on hole 15 -> capped to 6 -> adjusted gross 75.
//   - After it is deleted, Frank's index comes from three qualifying
//     rounds -> Course Handicap 16, stroke received on
//     hole 15 -> 7 counts uncapped -> adjusted gross 76.
func TestCLIRoundDeleteRecalculatesLaterRounds(t *testing.T) {
	bin := buildCLI(t)
	dataFile := filepath.Join(t.TempDir(), "golf.db")

	run(t, bin, dataFile, testCourseInput, "course", "add")

	qualifyingRound := "5\n8\n6\n6\n5\n2\n4\n6\n5\n6\n5\n7\n4\n5\n4\n4\n5\n4\n" // gross 91, diff 17.2
	for _, date := range []string{"2026-07-01", "2026-07-08", "2026-07-15"} {
		run(t, bin, dataFile, qualifyingRound,
			"round", "add", "--player", "Frank", "--course", "Test Golf Club", "--tee", "White", "--date", date)
	}

	mistakeRound2 := "4\n5\n4\n4\n4\n5\n4\n3\n5\n5\n4\n4\n3\n5\n4\n4\n3\n5\n" // gross 75, diff ~3.1 - entered by mistake
	run(t, bin, dataFile, mistakeRound2,
		"round", "add", "--player", "Frank", "--course", "Test Golf Club", "--tee", "White", "--date", "2026-07-22")

	round3 := "4\n5\n4\n3\n4\n5\n4\n3\n4\n5\n4\n4\n3\n5\n7\n4\n3\n5\n" // at par except hole 15 = 7
	run(t, bin, dataFile, round3,
		"round", "add", "--player", "Frank", "--course", "Test Golf Club", "--tee", "White", "--date", "2026-08-01")

	before := run(t, bin, dataFile, "", "round", "list", "--player", "Frank")
	beforeLine := lineContaining(t, before, "2026-08-01")
	assertContainsAll(t, beforeLine,
		fmt.Sprintf("gross %3d", 76), fmt.Sprintf("adj %3d", 75), fmt.Sprintf("ch %2d", 1))

	run(t, bin, dataFile, "y\n", "round", "delete", "4") // delete the mistaken round

	after := run(t, bin, dataFile, "", "round", "list", "--player", "Frank")
	afterLine := lineContaining(t, after, "2026-08-01")
	assertContainsAll(t, afterLine,
		fmt.Sprintf("gross %3d", 76), fmt.Sprintf("adj %3d", 76), fmt.Sprintf("ch %2d", 16))
}

func lineContaining(t *testing.T, output, substr string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, substr) {
			return line
		}
	}
	t.Fatalf("no line containing %q in:\n%s", substr, output)
	return ""
}

func assertContainsAll(t *testing.T, line string, substrs ...string) {
	t.Helper()
	for _, s := range substrs {
		if !strings.Contains(line, s) {
			t.Errorf("expected line to contain %q, got:\n%s", s, line)
		}
	}
}

// TestCLIRoundAddOutOfOrderRecalculatesLaterRounds proves that rounds no
// longer need to be entered in chronological order: adding an old round
// after newer ones are already on file must correctly recalculate those
// newer rounds' Course Handicap and Adjusted Gross Score, exactly as if
// it had been entered in date order to begin with.
//
// This mirrors TestCLIRoundDeleteRecalculatesLaterRounds in reverse: there
// a round was removed and later rounds recalculated down; here a round is
// inserted out of order and a later round must recalculate to match.
func TestCLIRoundAddOutOfOrderRecalculatesLaterRounds(t *testing.T) {
	bin := buildCLI(t)
	dataFile := filepath.Join(t.TempDir(), "golf.db")

	run(t, bin, dataFile, testCourseInput, "course", "add")

	roundA := "5\n8\n6\n6\n5\n2\n4\n6\n5\n6\n5\n7\n4\n5\n4\n4\n5\n4\n" // gross 91, diff 17.2
	for _, date := range []string{"2026-07-01", "2026-07-08", "2026-07-15"} {
		run(t, bin, dataFile, roundA,
			"round", "add", "--player", "Frank", "--course", "Test Golf Club", "--tee", "White", "--date", date)
	}

	// Round C, dated latest, is played after three qualifying rounds: prior
	// index is 15.2 -> Course Handicap 16 -> hole 15
	// (par 4, SI 10) scored 7 is within the one-stroke cap of 7, uncapped.
	roundC := "4\n5\n4\n3\n4\n5\n4\n3\n4\n5\n4\n4\n3\n5\n7\n4\n3\n5\n"
	run(t, bin, dataFile, roundC,
		"round", "add", "--player", "Frank", "--course", "Test Golf Club", "--tee", "White", "--date", "2026-08-01")

	before := run(t, bin, dataFile, "", "round", "list", "--player", "Frank")
	beforeLine := lineContaining(t, before, "2026-08-01")
	assertContainsAll(t, beforeLine, fmt.Sprintf("adj %3d", 76), fmt.Sprintf("ch %2d", 16))

	// Now backfill an even earlier round, dated before Round A, with a
	// very low differential (~3.1). Once it's on file, Round C's
	// chronological history contains the backfill plus three qualifying
	// rounds before Round C, so its prior index uses the low backfill with
	// a -1.0 adjustment -> Course Handicap 1 -> hole 15's cap drops
	// to 6, capping the 7 down to 6.
	backfill := "4\n5\n4\n4\n4\n5\n4\n3\n5\n5\n4\n4\n3\n5\n4\n4\n3\n5\n" // gross 75, diff ~3.1
	addOut := run(t, bin, dataFile, backfill,
		"round", "add", "--player", "Frank", "--course", "Test Golf Club", "--tee", "White", "--date", "2026-06-01")
	// All three qualifying rounds and Round C are dated after this backfill.
	if !strings.Contains(addOut, "Recalculated Frank's 4 later round(s)") {
		t.Errorf("expected a recalculation notice, got:\n%s", addOut)
	}

	after := run(t, bin, dataFile, "", "round", "list", "--player", "Frank")
	afterLine := lineContaining(t, after, "2026-08-01")
	assertContainsAll(t, afterLine, fmt.Sprintf("adj %3d", 75), fmt.Sprintf("ch %2d", 1))
}

func TestCLIRecalculateUsesSQLiteData(t *testing.T) {
	bin := buildCLI(t)
	dataFile := filepath.Join(t.TempDir(), "golf.db")
	run(t, bin, dataFile, testCourseInput, "course", "add")
	roundInput := "5\n8\n6\n6\n5\n2\n4\n6\n5\n6\n5\n7\n4\n5\n4\n4\n5\n4\n"
	for _, date := range []string{"2026-07-01", "2026-07-08", "2026-07-15"} {
		run(t, bin, dataFile, roundInput,
			"round", "add", "--player", "Frank", "--course", "Test Golf Club", "--tee", "White", "--date", date)
	}

	out := run(t, bin, dataFile, "", "recalculate")
	if !strings.Contains(out, "Recalculated 3 player round(s) across 1 player(s)") {
		t.Errorf("expected a recalculation summary, got:\n%s", out)
	}

	store, err := database.Open(dataFile)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	players, err := store.Players(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(players) != 1 || players[0].Name != "Frank" || players[0].RoundCount != 3 {
		t.Fatalf("SQLite players = %+v, want Frank with 3 rounds", players)
	}
	if players[0].GroupHandicapIndex == nil || *players[0].GroupHandicapIndex != 15.2 {
		t.Fatalf("SQLite handicap index = %v, want 15.2", players[0].GroupHandicapIndex)
	}
}

// TestCLIStartingHandicap exercises 'golf player set-starting-handicap'
// and 'golf player clear-starting-handicap' end to end: setting it
// changes how an already-recorded first round gets capped, clearing it
// reverts to the Par+5 rule, and 'golf player list' shows the nominated
// value.
func TestCLIStartingHandicap(t *testing.T) {
	bin := buildCLI(t)
	dataFile := filepath.Join(t.TempDir(), "golf.db")

	run(t, bin, dataFile, testCourseInput, "course", "add")

	// Every hole blown up to 9 strokes. testCourseInput's course has some
	// par-3 holes where Par+5 (=8) is still below 9, so the Par+5 rule
	// alone still caps those 4 holes even with no starting handicap:
	// 14 holes uncapped at 9 + 4 par-3 holes capped at 8 = 158.
	blownUp := strings.Repeat("9\n", 18)
	run(t, bin, dataFile, blownUp,
		"round", "add", "--player", "Newbie", "--course", "Test Golf Club", "--tee", "White", "--date", "2026-07-01")

	before := run(t, bin, dataFile, "", "round", "list", "--player", "Newbie")
	line := lineContaining(t, before, "2026-07-01")
	assertContainsAll(t, line, fmt.Sprintf("ch %2d", 0), "adj 158")

	setOut := run(t, bin, dataFile, "", "player", "set-starting-handicap", "--player", "Newbie", "--handicap", "30")
	if !strings.Contains(setOut, "starting Course Handicap set to 30") {
		t.Errorf("expected a confirmation message, got:\n%s", setOut)
	}

	// With a nominated Course Handicap of 30, Net Double Bogey capping
	// applies per hole based on its Stroke Index, giving a total of 139
	// (computed independently against this course's actual par/SI layout).
	afterSet := run(t, bin, dataFile, "", "round", "list", "--player", "Newbie")
	line = lineContaining(t, afterSet, "2026-07-01")
	assertContainsAll(t, line, fmt.Sprintf("ch %2d", 30), "adj 139")

	list := run(t, bin, dataFile, "", "player", "list")
	if !strings.Contains(list, "Newbie") || !strings.Contains(list, "30") {
		t.Errorf("expected player list to show the starting handicap of 30, got:\n%s", list)
	}

	clearOut := run(t, bin, dataFile, "", "player", "clear-starting-handicap", "--player", "Newbie")
	if !strings.Contains(clearOut, "starting Course Handicap cleared") {
		t.Errorf("expected a confirmation message, got:\n%s", clearOut)
	}

	afterClear := run(t, bin, dataFile, "", "round", "list", "--player", "Newbie")
	line = lineContaining(t, afterClear, "2026-07-01")
	assertContainsAll(t, line, fmt.Sprintf("ch %2d", 0), "adj 158") // back to the Par+5 rule
}

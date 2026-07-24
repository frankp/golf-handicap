package main

import (
	"fmt"
	"math"
	"testing"
)

func TestNetDoubleBogeyCap(t *testing.T) {
	cases := []struct {
		name           string
		par            int
		courseHandicap int
		strokeIndex    int
		want           int
	}{
		{"no stroke received", 4, 10, 15, 6},    // par+2, CH doesn't reach SI
		{"one stroke received", 4, 17, 7, 7},    // par+2+1, CH>=SI
		{"two strokes received", 4, 30, 7, 8},   // par+2+2, CH>=SI+18
		{"three strokes received", 4, 43, 7, 9}, // par+2+3, CH>=SI+36
		{"three strokes exact boundary", 4, 37, 1, 9},
		{"exact boundary one stroke", 4, 7, 7, 7},
		{"just under boundary", 4, 6, 7, 6},
		{"negative handicap clamped to zero strokes", 4, -3, 1, 6},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := netDoubleBogeyCap(c.par, c.courseHandicap, c.strokeIndex)
			if got != c.want {
				t.Errorf("netDoubleBogeyCap(%d, %d, %d) = %d, want %d",
					c.par, c.courseHandicap, c.strokeIndex, got, c.want)
			}
		})
	}
}

// TestAdjustedGrossScore mirrors Norman's example from the CONGU/WHS
// worked examples (Appendix I, II.A): Course Handicap 15, three holes
// blown up to 9 strokes each, capped by Net Double Bogey.
//
//	Hole 10, Par 4, SI 6  -> cap 4+2+1=7 (15 >= 6)
//	Hole 11, Par 3, SI 12 -> cap 3+2+1=6 (15 >= 12)
//	Hole 17, Par 5, SI 16 -> cap 5+2+0=7 (15 <  16, no stroke)
//
// The remaining 15 holes are played exactly at par (no cap applies), so
// the expected adjusted total is verifiable by direct arithmetic.
func TestAdjustedGrossScore(t *testing.T) {
	const courseHandicap = 15

	var par, si, scores [18]int
	for i := 0; i < 18; i++ {
		par[i], si[i], scores[i] = 4, i+1, 4 // played exactly at par by default
	}
	par[9], si[9], scores[9] = 4, 6, 9     // hole 10
	par[10], si[10], scores[10] = 3, 12, 9 // hole 11
	par[16], si[16], scores[16] = 5, 16, 9 // hole 17

	otherHolesTotal := 15 * 4 // the 15 holes played exactly at par
	wantTotal := otherHolesTotal + 7 + 6 + 7

	got := adjustedGrossScore(scores, par, si, courseHandicap, false)
	if got != wantTotal {
		t.Errorf("adjustedGrossScore = %d, want %d", got, wantTotal)
	}
}

func TestAdjustedGrossScoreInitialCapAndPickedUp(t *testing.T) {
	var par, si, scores [18]int
	par[0], si[0], scores[0] = 4, 1, 0 // picked up on hole 1
	for i := 1; i < 18; i++ {
		par[i], si[i], scores[i] = 4, i+1, 4
	}

	got := adjustedGrossScore(scores, par, si, 0, true)
	want := (4 + 5) + 17*4 // hole 1 capped at par+5 under the initial-handicap rule
	if got != want {
		t.Errorf("adjustedGrossScore (initial cap, picked up) = %d, want %d", got, want)
	}
}

func TestScoreDifferential(t *testing.T) {
	// Norma's example: (113/129) x (88 - 72.0 - 1) = 13.1
	// (scoreDifferential has no separate PCC term, so the PCC is folded
	// into the rating argument here: 72.0 + 1.0 PCC.)
	if got := round1(scoreDifferential(88, 72.0+1.0, 129)); got != 13.1 {
		t.Errorf("Norma's differential = %v, want 13.1", got)
	}
	// Norman's example: (113/113) x (82 - 67.2 - 1) = 13.8
	if got := round1(scoreDifferential(82, 67.2+1.0, 113)); got != 13.8 {
		t.Errorf("Norman's differential = %v, want 13.8", got)
	}
}

func TestCourseHandicap(t *testing.T) {
	cases := []struct {
		index  float64
		rating float64
		slope  int
		par    int
		want   int
	}{
		{15.0, 72.0, 113, 72, 15},
		{15.0, 72.0, 129, 72, 17},
		{15.0, 69.0, 113, 72, 12}, // Rating-par lowers the Course Handicap by 3.
	}
	for _, c := range cases {
		if got := courseHandicap(c.index, c.rating, c.slope, c.par); got != c.want {
			t.Errorf("courseHandicap(%v, %v, %d, %d) = %d, want %d",
				c.index, c.rating, c.slope, c.par, got, c.want)
		}
	}
}

func TestLowScoreCountTable(t *testing.T) {
	cases := []struct {
		n          int
		wantUse    int
		wantAdjust float64
	}{
		{3, 1, -2.0},
		{4, 1, -1.0},
		{5, 1, 0.0},
		{6, 2, -1.0},
		{7, 2, 0.0}, {8, 2, 0.0},
		{9, 3, 0.0}, {10, 3, 0.0}, {11, 3, 0.0},
		{12, 4, 0.0}, {13, 4, 0.0}, {14, 4, 0.0},
		{15, 5, 0.0}, {16, 5, 0.0},
		{17, 6, 0.0}, {18, 6, 0.0},
		{19, 7, 0.0},
		{20, 8, 0.0}, {25, 8, 0.0},
	}
	for _, c := range cases {
		use, adj := lowScoreCountTable(c.n)
		if use != c.wantUse || adj != c.wantAdjust {
			t.Errorf("lowScoreCountTable(%d) = (%d, %v), want (%d, %v)",
				c.n, use, adj, c.wantUse, c.wantAdjust)
		}
	}
}

// TestHandicapIndexJohnExample replays the worked example from the
// CONGU/WHS "Guidance on the Rules of Handicapping" Appendix I, where a
// new player John accumulates differentials 22.1, 25.0, 26.4, 22.0, 24.0,
// 21.7, 21.2, 22.0, 19.4 (chronological, oldest first) and his Handicap
// Index is checked after each one.
func TestHandicapIndexJohnExample(t *testing.T) {
	chronological := []float64{22.1, 25.0, 26.4, 22.0, 24.0, 21.7, 21.2, 22.0, 19.4}
	wantAtCount := map[int]float64{
		3: 20.1,
		4: 21.0,
		5: 22.0,
		6: 20.9,
		7: 21.5,
		8: 21.5,
		9: 20.8,
	}
	for n := 3; n <= len(chronological); n++ {
		mostRecentFirst := toRoundsMostRecentFirst(chronological[:n])
		got, ok := handicapIndex(mostRecentFirst)
		if !ok {
			t.Fatalf("handicapIndex returned ok=false for %d rounds", n)
		}
		if want := wantAtCount[n]; got != want {
			t.Errorf("after %d rounds: handicapIndex = %v, want %v", n, got, want)
		}
	}
}

// TestHandicapIndexTwentyScoreExample replays the worked example from the
// same Appendix showing the "best 8 of the most recent 20" calculation,
// and how adding a 21st score displaces the oldest one. The source table
// is already given most-recent-first, so no reversal is needed here.
func TestHandicapIndexTwentyScoreExample(t *testing.T) {
	mostRecentFirst := []float64{
		18.5, 19.4, 25.8, 16.7, 18.4, 12.8, 24.0, 15.8, 13.5, 24.0,
		15.6, 11.0, 10.4, 21.2, 18.3, 24.0, 13.1, 20.3, 21.2, 10.1,
	}
	got, ok := handicapIndex(toRounds(mostRecentFirst))
	if !ok {
		t.Fatal("handicapIndex returned ok=false")
	}
	if got != 12.8 {
		t.Errorf("initial 20-score handicap index = %v, want 12.8", got)
	}

	// A new score of 11.8 arrives; the oldest (10.1, last in this slice) drops off.
	withNewScore := append([]float64{11.8}, mostRecentFirst[:19]...)
	got, ok = handicapIndex(toRounds(withNewScore))
	if !ok {
		t.Fatal("handicapIndex returned ok=false")
	}
	if got != 13.0 {
		t.Errorf("handicap index after new score = %v, want 13.0", got)
	}
}

func TestHandicapIndexNoRounds(t *testing.T) {
	if _, ok := handicapIndex(nil); ok {
		t.Error("handicapIndex(nil) should return ok=false")
	}
}

func TestHandicapIndexRequiresThreeRounds(t *testing.T) {
	for n := 1; n < 3; n++ {
		if _, ok := handicapIndex(toRounds([]float64{10, 11}[:n])); ok {
			t.Errorf("handicapIndex returned ok=true for %d round(s), want false", n)
		}
	}
}

// --- test helpers ---

func round1(f float64) float64 {
	return math.Round(f*10) / 10
}

// toRounds wraps differentials into Rounds, preserving order as given.
func toRounds(diffs []float64) []Round {
	rounds := make([]Round, len(diffs))
	for i, d := range diffs {
		rounds[i] = Round{ScoreDifferential: d}
	}
	return rounds
}

// toRoundsMostRecentFirst takes differentials in chronological (oldest
// first) order and returns them as Rounds in most-recent-first order,
// matching what handicapIndex expects.
func toRoundsMostRecentFirst(chronological []float64) []Round {
	reversed := make([]float64, len(chronological))
	for i, d := range chronological {
		reversed[len(chronological)-1-i] = d
	}
	return toRounds(reversed)
}

// TestApplyHandicapCap replays the worked example from the WHS Rule 5.8
// soft-cap/hard-cap explainer: Low Handicap Index 12, a raw calculation
// of 17.0. The rise of 5.0 exceeds the 3.0 soft-cap threshold, so only
// half the excess above 3.0 counts: 12 + 3.0 + 0.5*(17.0-15.0) = 16.0.
func TestApplyHandicapCap(t *testing.T) {
	cases := []struct {
		name     string
		rawIndex float64
		lowIndex float64
		want     float64
	}{
		{"decrease passes through unchanged", 8.0, 12.0, 8.0},
		{"small rise (<=3.0) passes through unchanged", 14.5, 12.0, 14.5},
		{"exactly at the 3.0 threshold is unchanged", 15.0, 12.0, 15.0},
		{"soft cap halves the excess above 3.0", 17.0, 12.0, 16.0},
		{"hard cap ceilings the total rise at 5.0", 20.0, 12.0, 17.0},
		{"far beyond hard cap is still ceilinged at 5.0", 40.0, 12.0, 17.0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := applyHandicapCap(c.rawIndex, c.lowIndex); got != c.want {
				t.Errorf("applyHandicapCap(%v, %v) = %v, want %v", c.rawIndex, c.lowIndex, got, c.want)
			}
		})
	}
}

func TestLowHandicapIndex(t *testing.T) {
	rounds := []Round{
		{Date: "2025-01-01", EffectiveIndexAfter: 20.0}, // position 0
		{Date: "2025-06-01", EffectiveIndexAfter: 10.0}, // position 1: the true low
		{Date: "2025-08-01", EffectiveIndexAfter: 15.0}, // position 2
	}

	t.Run("finds the lowest within the trailing 365 days", func(t *testing.T) {
		low, ok := lowHandicapIndex(rounds, []int{0, 1, 2}, "2025-12-01")
		if !ok || low != 10.0 {
			t.Errorf("lowHandicapIndex = (%v, %v), want (10.0, true)", low, ok)
		}
	})

	t.Run("excludes rounds older than 365 days", func(t *testing.T) {
		// Cutoff far enough past 2025-06-01 that only the 15.0 round remains.
		low, ok := lowHandicapIndex(rounds, []int{0, 1, 2}, "2026-07-01")
		if !ok || low != 15.0 {
			t.Errorf("lowHandicapIndex = (%v, %v), want (15.0, true)", low, ok)
		}
	})

	t.Run("no positions means no low", func(t *testing.T) {
		if _, ok := lowHandicapIndex(rounds, nil, "2025-12-01"); ok {
			t.Error("expected ok=false with no positions given")
		}
	})
}

// TestRecalculatePlayerRoundsAppliesCap checks that the soft/hard cap is
// actually wired into recalculatePlayerRounds, not just correct in
// isolation. Twenty good rounds establish a Low Handicap Index, then
// enough bad rounds displace those scores from the most recent 20 and
// push the raw Rule 5.2b average above the cap threshold.
func TestRecalculatePlayerRoundsAppliesCap(t *testing.T) {
	var course Course
	course.Name, course.Tee, course.Rating, course.Slope = "Synthetic", "Test", 72.0, 113
	for i := 0; i < 18; i++ {
		course.Par[i] = 4
		course.StrokeIndex[i] = i + 1
	}

	var scoresAtPar, scoresBad [18]int
	for i := 0; i < 18; i++ {
		scoresAtPar[i] = 4 // adjusted gross 72 -> differential 72.0
		scoresBad[i] = 15  // capped hard by Net Double Bogey each round, but still much worse
	}

	d := Data{Courses: []Course{course}}
	dates := []string{
		"2025-01-01", "2025-01-08", "2025-01-15", "2025-01-22", "2025-01-29",
		"2025-02-05", "2025-02-12", "2025-02-19", "2025-02-26", "2025-03-05",
		"2025-03-12", "2025-03-19", "2025-03-26", "2025-04-02", "2025-04-09",
		"2025-04-16", "2025-04-23", "2025-04-30", "2025-05-07", "2025-05-14",
		"2025-05-21", "2025-05-28", "2025-06-04", "2025-06-11", "2025-06-18",
		"2025-06-25", "2025-07-02", "2025-07-09", "2025-07-16", "2025-07-23",
		"2025-07-30", "2025-08-06", "2025-08-13", "2025-08-20", "2025-08-27",
		"2025-09-03", "2025-09-10", "2025-09-17", "2025-09-24", "2025-10-01",
	}
	for _, date := range dates[:20] {
		d.Rounds = append(d.Rounds, Round{Player: "Cap", Date: date, CourseName: course.Name, Tee: course.Tee, Scores: scoresAtPar})
	}
	for _, date := range dates[20:] {
		d.Rounds = append(d.Rounds, Round{Player: "Cap", Date: date, CourseName: course.Name, Tee: course.Tee, Scores: scoresBad})
	}

	if err := recalculatePlayerRounds(&d, "Cap"); err != nil {
		t.Fatalf("recalculatePlayerRounds failed: %v", err)
	}

	positions := datedPositionsForPlayer(d.Rounds, "Cap")
	last := len(positions) - 1

	throughLastMostRecentFirst := make([]Round, last+1)
	for j := 0; j <= last; j++ {
		throughLastMostRecentFirst[last-j] = d.Rounds[positions[j]]
	}
	rawIndex, ok := handicapIndex(throughLastMostRecentFirst)
	if !ok {
		t.Fatal("handicapIndex returned ok=false")
	}
	low, haveLow := lowHandicapIndex(d.Rounds, positions[19:last], d.Rounds[positions[last]].Date)
	if !haveLow {
		t.Fatal("expected a Low Handicap Index to exist by the last round")
	}
	wantEffective := applyHandicapCap(rawIndex, low)
	gotEffective := d.Rounds[positions[last]].EffectiveIndexAfter

	if rawIndex-low <= 3.0 {
		t.Fatalf("test setup didn't produce a rise big enough to trigger the cap (raw %v, low %v)", rawIndex, low)
	}
	if gotEffective != wantEffective {
		t.Errorf("last round's EffectiveIndexAfter = %v, want %v (raw %v capped against low %v)",
			gotEffective, wantEffective, rawIndex, low)
	}
	if gotEffective >= rawIndex {
		t.Errorf("cap should have reduced the effective index below the raw one: effective %v, raw %v", gotEffective, rawIndex)
	}
}

func TestRecalculatePlayerRoundsDoesNotCapBeforeTwentyScores(t *testing.T) {
	var course Course
	course.Name, course.Tee, course.Rating, course.Slope = "Synthetic", "Test", 72.0, 113
	for i := 0; i < 18; i++ {
		course.Par[i] = 4
		course.StrokeIndex[i] = i + 1
	}

	var good, bad [18]int
	for i := range good {
		good[i] = 4
		bad[i] = 15
	}

	d := Data{Courses: []Course{course}}
	for i := 0; i < 19; i++ {
		scores := bad
		if i < 3 {
			scores = good
		}
		d.Rounds = append(d.Rounds, Round{
			Player: "New", Date: fmt.Sprintf("2025-01-%02d", i+1),
			CourseName: course.Name, Tee: course.Tee, Scores: scores,
		})
	}
	if err := recalculatePlayerRounds(&d, "New"); err != nil {
		t.Fatalf("recalculatePlayerRounds failed: %v", err)
	}

	raw, ok := handicapIndex(mostRecentFirstForPlayer(d.Rounds, "New"))
	if !ok {
		t.Fatal("expected an established Handicap Index")
	}
	if got := d.Rounds[18].EffectiveIndexAfter; got != raw {
		t.Errorf("19th round effective index = %v, want uncapped raw index %v", got, raw)
	}
}

// TestStartingHandicapAppliesForQualifyingRoundsOnly uses the same
// synthetic course (Rating equals par, Slope 113, so Course Handicap
// equals index) to check the nominated-starting-handicap feature precisely.
//
// A blown-up round (every hole scored 9, i.e. Par+5) is played 4 times:
//   - With no starting handicap, round 1 falls back to the Par+5 rule,
//     so nothing is capped: adjusted gross stays at 18*9=162.
//   - With a starting handicap of 30 set, Net Double Bogey capping
//     applies from round 1: holes with Stroke Index 1-12 get 2 strokes
//     (cap 4+2+2=8, since 30 >= SI+18) and holes 13-18 get 1 (cap
//     4+2+1=7, since 30 >= SI but 30 < SI+18) -> 12*8 + 6*7 = 138.
//   - That applies to rounds 1-3 (qualifyingRounds). Round 4 must revert
//     to the normal derived-Course-Handicap behavior.
func TestStartingHandicapAppliesForQualifyingRoundsOnly(t *testing.T) {
	var course Course
	course.Name, course.Tee, course.Rating, course.Slope = "Synthetic", "Test", 72.0, 113
	for i := 0; i < 18; i++ {
		course.Par[i] = 4
		course.StrokeIndex[i] = i + 1
	}
	var blownUp [18]int
	for i := range blownUp {
		blownUp[i] = 9
	}

	newData := func() Data {
		d := Data{Courses: []Course{course}}
		dates := []string{"2025-01-01", "2025-01-08", "2025-01-15", "2025-01-22"}
		for _, date := range dates {
			d.Rounds = append(d.Rounds, Round{
				Player: "Newbie", Date: date, CourseName: course.Name, Tee: course.Tee, Scores: blownUp,
			})
		}
		return d
	}

	t.Run("no starting handicap falls back to Par+5 on round 1", func(t *testing.T) {
		d := newData()
		if err := recalculatePlayerRounds(&d, "Newbie"); err != nil {
			t.Fatalf("recalculatePlayerRounds failed: %v", err)
		}
		for i := 0; i < qualifyingRounds; i++ {
			if got := d.Rounds[i].AdjustedGrossAt; got != 162 {
				t.Errorf("round %d adjusted gross = %d, want 162 (Par+5, uncapped)", i+1, got)
			}
		}
	})

	t.Run("starting handicap caps rounds 1-3, round 4 reverts to normal", func(t *testing.T) {
		d := newData()
		d.SetStartingHandicap("Newbie", 30)
		if err := recalculatePlayerRounds(&d, "Newbie"); err != nil {
			t.Fatalf("recalculatePlayerRounds failed: %v", err)
		}
		for i := 0; i < 3; i++ {
			if got := d.Rounds[i].CourseHandicapAt; got != 30 {
				t.Errorf("round %d Course Handicap = %d, want 30 (nominated)", i+1, got)
			}
			if got := d.Rounds[i].AdjustedGrossAt; got != 138 {
				t.Errorf("round %d adjusted gross = %d, want 138", i+1, got)
			}
		}
		round3Effective := d.Rounds[2].EffectiveIndexAfter
		wantRound4CH := courseHandicap(round3Effective, course.Rating, course.Slope, 72)
		if got := d.Rounds[3].CourseHandicapAt; got != wantRound4CH {
			t.Errorf("round 4 Course Handicap = %d, want %d (derived from round 3's index, not the nominated 30)",
				got, wantRound4CH)
		}
	})

	t.Run("clearing the starting handicap reverts round 1 to Par+5", func(t *testing.T) {
		d := newData()
		d.SetStartingHandicap("Newbie", 30)
		if err := recalculatePlayerRounds(&d, "Newbie"); err != nil {
			t.Fatalf("recalculatePlayerRounds failed: %v", err)
		}
		d.ClearStartingHandicap("Newbie")
		if err := recalculatePlayerRounds(&d, "Newbie"); err != nil {
			t.Fatalf("recalculatePlayerRounds failed: %v", err)
		}
		if got := d.Rounds[0].AdjustedGrossAt; got != 162 {
			t.Errorf("round 1 adjusted gross after clearing = %d, want 162", got)
		}
	})
}

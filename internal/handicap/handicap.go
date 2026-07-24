package handicap

import (
	"fmt"
	"math"
	"sort"
	"time"
)

const QualifyingRounds = 3

type Course struct {
	Name        string  `json:"name"`
	Tee         string  `json:"tee"`
	Rating      float64 `json:"rating"`
	Slope       int     `json:"slope"`
	Par         [18]int `json:"par"`
	StrokeIndex [18]int `json:"strokeIndex"`
}

func (c Course) Key() string {
	return c.Name + "|" + c.Tee
}

func (c Course) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("course name is required")
	}
	if c.Tee == "" {
		return fmt.Errorf("tee name is required")
	}
	if c.Rating <= 0 {
		return fmt.Errorf("course rating must be greater than zero")
	}
	if c.Slope < 55 || c.Slope > 155 {
		return fmt.Errorf("slope rating must be between 55 and 155, got %d", c.Slope)
	}
	seen := map[int]bool{}
	for _, si := range c.StrokeIndex {
		if si < 1 || si > 18 || seen[si] {
			return fmt.Errorf("stroke indices must be a permutation of 1-18")
		}
		seen[si] = true
	}
	for _, p := range c.Par {
		if p < 3 || p > 6 {
			return fmt.Errorf("par values must be between 3 and 6")
		}
	}
	return nil
}

type Round struct {
	Player              string  `json:"player"`
	Date                string  `json:"date"`
	CourseName          string  `json:"courseName"`
	Tee                 string  `json:"tee"`
	Scores              [18]int `json:"scores"`
	CourseHandicapAt    int     `json:"courseHandicapAt"`
	AdjustedGrossAt     int     `json:"adjustedGrossAt"`
	ScoreDifferential   float64 `json:"scoreDifferential"`
	EffectiveIndexAfter float64 `json:"effectiveIndexAfter"`
}

type Data struct {
	Courses           []Course       `json:"courses"`
	Rounds            []Round        `json:"rounds"`
	StartingHandicaps map[string]int `json:"startingHandicaps,omitempty"`
}

func (d *Data) StartingHandicap(player string) (int, bool) {
	ch, ok := d.StartingHandicaps[player]
	return ch, ok
}

func (d *Data) SetStartingHandicap(player string, ch int) {
	if d.StartingHandicaps == nil {
		d.StartingHandicaps = map[string]int{}
	}
	d.StartingHandicaps[player] = ch
}

func (d *Data) ClearStartingHandicap(player string) {
	delete(d.StartingHandicaps, player)
}

func (d *Data) FindCourse(name, tee string) (Course, bool) {
	for _, c := range d.Courses {
		if c.Name == name && c.Tee == tee {
			return c, true
		}
	}
	return Course{}, false
}

func NetDoubleBogeyCap(par, courseHandicap, strokeIndex int) int {
	if courseHandicap < 0 {
		courseHandicap = 0
	}
	if courseHandicap > 54 {
		courseHandicap = 54
	}
	strokes := courseHandicap / 18
	if courseHandicap%18 >= strokeIndex {
		strokes++
	}
	return par + 2 + strokes
}

func AdjustedGrossScore(scores, par, strokeIndex [18]int, courseHandicap int, useInitialCap bool) int {
	total := 0
	for i := 0; i < 18; i++ {
		cap := par[i] + 5
		if !useInitialCap {
			cap = NetDoubleBogeyCap(par[i], courseHandicap, strokeIndex[i])
		}
		s := scores[i]
		if s <= 0 || s > cap {
			s = cap
		}
		total += s
	}
	return total
}

func ScoreDifferential(adjustedGross int, rating float64, slope int) float64 {
	return (113.0 / float64(slope)) * (float64(adjustedGross) - rating)
}

func CourseHandicap(index, rating float64, slope, par int) int {
	return int(math.Round(index*float64(slope)/113.0 + rating - float64(par)))
}

func TotalPar(par [18]int) int {
	total := 0
	for _, p := range par {
		total += p
	}
	return total
}

func ApplyHandicapCap(rawIndex, lowIndex float64) float64 {
	if rawIndex-lowIndex <= 3.0 {
		return rawIndex
	}
	excess := rawIndex - (lowIndex + 3.0)
	softCapped := lowIndex + 3.0 + 0.5*excess
	hardCapped := lowIndex + 5.0
	if softCapped > hardCapped {
		return hardCapped
	}
	return softCapped
}

func LowHandicapIndex(rounds []Round, positions []int, cutoffDate string) (float64, bool) {
	cutoff, err := time.Parse("2006-01-02", cutoffDate)
	if err != nil {
		return 0, false
	}
	windowStart := cutoff.AddDate(0, 0, -365)
	low := math.Inf(1)
	ok := false
	for _, p := range positions {
		d, err := time.Parse("2006-01-02", rounds[p].Date)
		if err != nil || d.Before(windowStart) {
			continue
		}
		if rounds[p].EffectiveIndexAfter < low {
			low = rounds[p].EffectiveIndexAfter
			ok = true
		}
	}
	return low, ok
}

func DatedPositionsForPlayer(rounds []Round, player string) []int {
	var positions []int
	for i, r := range rounds {
		if r.Player == player {
			positions = append(positions, i)
		}
	}
	sort.SliceStable(positions, func(a, b int) bool {
		return rounds[positions[a]].Date < rounds[positions[b]].Date
	})
	return positions
}

func RecalculatePlayerRounds(d *Data, player string) error {
	positions := DatedPositionsForPlayer(d.Rounds, player)
	for k, pos := range positions {
		r := &d.Rounds[pos]
		course, ok := d.FindCourse(r.CourseName, r.Tee)
		if !ok {
			return fmt.Errorf("course %q tee %q is not defined", r.CourseName, r.Tee)
		}

		startingCH, hasStarting := d.StartingHandicap(player)
		useInitialCap := false
		ch := 0
		switch {
		case hasStarting && k < QualifyingRounds:
			ch = startingCH
		case k < QualifyingRounds:
			useInitialCap = true
		default:
			previousIndex := d.Rounds[positions[k-1]].EffectiveIndexAfter
			ch = CourseHandicap(previousIndex, course.Rating, course.Slope, TotalPar(course.Par))
		}
		r.CourseHandicapAt = ch
		r.AdjustedGrossAt = AdjustedGrossScore(r.Scores, course.Par, course.StrokeIndex, ch, useInitialCap)
		r.ScoreDifferential = ScoreDifferential(r.AdjustedGrossAt, course.Rating, course.Slope)

		history := make([]Round, k+1)
		for j := 0; j <= k; j++ {
			history[k-j] = d.Rounds[positions[j]]
		}
		rawIndex, established := HandicapIndex(history)
		if !established {
			r.EffectiveIndexAfter = 0
			continue
		}
		r.EffectiveIndexAfter = rawIndex
		if k >= 20 {
			if low, ok := LowHandicapIndex(d.Rounds, positions[19:k], r.Date); ok {
				r.EffectiveIndexAfter = ApplyHandicapCap(rawIndex, low)
			}
		}
	}
	return nil
}

func RoundsUpToDate(rounds []Round, player, cutoff string) []Round {
	positions := DatedPositionsForPlayer(rounds, player)
	var filtered []int
	for _, p := range positions {
		if rounds[p].Date <= cutoff {
			filtered = append(filtered, p)
		}
	}
	out := make([]Round, len(filtered))
	for i, p := range filtered {
		out[len(filtered)-1-i] = rounds[p]
	}
	return out
}

func CurrentEffectiveIndex(rounds []Round, player string) (float64, bool) {
	mostRecent := MostRecentFirstForPlayer(rounds, player)
	if len(mostRecent) < QualifyingRounds {
		return 0, false
	}
	return mostRecent[0].EffectiveIndexAfter, true
}

func MostRecentFirstForPlayer(rounds []Round, player string) []Round {
	positions := DatedPositionsForPlayer(rounds, player)
	out := make([]Round, len(positions))
	for i, p := range positions {
		out[len(positions)-1-i] = rounds[p]
	}
	return out
}

func LowScoreCountTable(n int) (use int, adjustment float64) {
	switch {
	case n <= 3:
		return 1, -2.0
	case n == 4:
		return 1, -1.0
	case n == 5:
		return 1, 0
	case n == 6:
		return 2, -1.0
	case n <= 8:
		return 2, 0
	case n <= 11:
		return 3, 0
	case n <= 14:
		return 4, 0
	case n <= 16:
		return 5, 0
	case n <= 18:
		return 6, 0
	case n == 19:
		return 7, 0
	default:
		return 8, 0
	}
}

func HandicapIndex(roundsMostRecentFirst []Round) (float64, bool) {
	n := len(roundsMostRecentFirst)
	if n < QualifyingRounds {
		return 0, false
	}
	recent := roundsMostRecentFirst
	if n > 20 {
		recent = roundsMostRecentFirst[:20]
		n = 20
	}
	diffs := make([]float64, n)
	for i, r := range recent {
		diffs[i] = r.ScoreDifferential
	}
	sort.Float64s(diffs)
	use, adjustment := LowScoreCountTable(n)
	sum := 0.0
	for i := 0; i < use; i++ {
		sum += diffs[i]
	}
	return math.Round((sum/float64(use)+adjustment)*10) / 10, true
}

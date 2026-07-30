package main

import "golf/internal/handicap"

type Course = handicap.Course
type Round = handicap.Round
type Data = handicap.Data
type HandicapCategory = handicap.HandicapCategory

const qualifyingRounds = handicap.QualifyingRounds
const (
	men   = handicap.Men
	women = handicap.Women
)

func netDoubleBogeyCap(par, dailyHandicap, strokeIndex int) int {
	return handicap.NetDoubleBogeyCap(par, dailyHandicap, strokeIndex)
}

func netScores(scores, strokeIndex [18]int, handicapUsed float64) [18]int {
	return handicap.NetScores(scores, strokeIndex, handicapUsed)
}

func adjustedGrossScore(scores, par, strokeIndex [18]int, dailyHandicap int, useInitialCap bool) int {
	return handicap.AdjustedGrossScore(scores, par, strokeIndex, dailyHandicap, useInitialCap)
}

func scoreDifferential(adjustedGross int, rating float64, slope int) float64 {
	return handicap.ScoreDifferential(adjustedGross, rating, slope)
}

func dailyHandicap(index, rating float64, slope, par int, category handicap.HandicapCategory) int {
	return handicap.DailyHandicap(index, rating, slope, par, category)
}

func totalPar(par [18]int) int {
	return handicap.TotalPar(par)
}

func applyHandicapCap(rawIndex, lowIndex float64) float64 {
	return handicap.ApplyHandicapCap(rawIndex, lowIndex)
}

func lowHandicapIndex(rounds []Round, positions []int, cutoffDate string) (float64, bool) {
	return handicap.LowHandicapIndex(rounds, positions, cutoffDate)
}

func datedPositionsForPlayer(rounds []Round, player string) []int {
	return handicap.DatedPositionsForPlayer(rounds, player)
}

func recalculatePlayerRounds(d *Data, player string) error {
	return handicap.RecalculatePlayerRounds(d, player)
}

func roundsUpToDate(rounds []Round, player, cutoff string) []Round {
	return handicap.RoundsUpToDate(rounds, player, cutoff)
}

func currentEffectiveIndex(rounds []Round, player string) (float64, bool) {
	return handicap.CurrentEffectiveIndex(rounds, player)
}

func mostRecentFirstForPlayer(rounds []Round, player string) []Round {
	return handicap.MostRecentFirstForPlayer(rounds, player)
}

func lowScoreCountTable(n int) (int, float64) {
	return handicap.LowScoreCountTable(n)
}

func handicapIndex(roundsMostRecentFirst []Round) (float64, bool) {
	return handicap.HandicapIndex(roundsMostRecentFirst)
}

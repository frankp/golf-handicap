package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golf/internal/database"
	"golf/internal/handicap"
)

const databaseFlagDefault = "golf.db"

var stdin = bufio.NewReader(os.Stdin)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	databasePath := os.Getenv("GOLF_DB")
	if databasePath == "" {
		databasePath = databaseFlagDefault
	}
	store, err := database.Open(databasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer store.Close()

	ctx := context.Background()
	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "course":
		err = courseCmd(ctx, store, args)
	case "round":
		err = roundCmd(ctx, store, args)
	case "index":
		err = indexCmd(ctx, store, args)
	case "handicap":
		err = handicapCmd(ctx, store, args)
	case "player":
		err = playerCmd(ctx, store, args)
	case "recalculate":
		err = recalculateCmd(ctx, store, args)
	case "help", "-h", "--help":
		usage()
		return
	default:
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`golf - WHS-style handicap tracker

Usage:
  golf course add                 Define a course/tee (par + stroke index for each hole)
  golf course list                List defined courses/tees
  golf player list                List players
  golf round add --player NAME --course COURSE --tee TEE --date YYYY-MM-DD
                 [--handicap-used N]
                                   Record a round, entering each hole's score
  golf round list [--player NAME] List recorded rounds and their differentials
  golf round delete ID [--yes]    Delete a round by the ID shown in 'round list'
  golf index --player NAME        Show a player's current group Handicap Index
  golf handicap --player NAME COURSE TEE
                                   Show a player's Daily Handicap for a course/tee
  golf recalculate                Recompute every round's cached handicap values

Data is stored in ./golf.db (override with GOLF_DB).`)
}

func prompt(label string) string {
	fmt.Print(label + ": ")
	line, _ := stdin.ReadString('\n')
	return strings.TrimSpace(line)
}

func promptInt(label string) (int, error) {
	return strconv.Atoi(prompt(label))
}

func promptFloat(label string) (float64, error) {
	return strconv.ParseFloat(prompt(label), 64)
}

func courseCmd(ctx context.Context, store *database.Store, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected a subcommand: add, list")
	}
	switch args[0] {
	case "add":
		return courseAdd(ctx, store)
	case "list":
		return courseList(ctx, store)
	default:
		return fmt.Errorf("unknown course subcommand %q", args[0])
	}
}

func courseAdd(ctx context.Context, store *database.Store) error {
	var course handicap.Course
	var err error
	course.Name = prompt("Course name")
	course.Tee = prompt("Tee (e.g. White, Yellow)")
	if course.Rating, err = promptFloat("18-hole Course Rating (e.g. 71.5)"); err != nil {
		return fmt.Errorf("invalid course rating: %w", err)
	}
	if course.Slope, err = promptInt("18-hole Slope Rating (e.g. 128)"); err != nil {
		return fmt.Errorf("invalid slope rating: %w", err)
	}

	fmt.Println("Enter par and stroke index for each hole (stroke index 1-18, each used once):")
	for i := 0; i < 18; i++ {
		hole := i + 1
		if course.Par[i], err = promptInt(fmt.Sprintf("  Hole %d par", hole)); err != nil {
			return fmt.Errorf("invalid par for hole %d: %w", hole, err)
		}
		if course.StrokeIndex[i], err = promptInt(fmt.Sprintf("  Hole %d stroke index", hole)); err != nil {
			return fmt.Errorf("invalid stroke index for hole %d: %w", hole, err)
		}
	}

	if _, err := store.CreateTee(ctx, course); err != nil {
		return err
	}
	fmt.Printf("Saved %s (%s tee).\n", course.Name, course.Tee)
	return nil
}

func courseList(ctx context.Context, store *database.Store) error {
	courses, err := store.Courses(ctx)
	if err != nil {
		return err
	}
	tees := 0
	for _, course := range courses {
		for _, tee := range course.Tees {
			fmt.Printf("%-25s %-10s CR %.1f  Slope %d\n", course.Name, tee.Name, tee.Rating, tee.Slope)
			tees++
		}
	}
	if tees == 0 {
		fmt.Println("No courses defined yet. Use 'golf course add'.")
	}
	return nil
}

func roundCmd(ctx context.Context, store *database.Store, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected a subcommand: add, list, delete")
	}
	switch args[0] {
	case "add":
		return roundAdd(ctx, store, args[1:])
	case "list":
		return roundList(ctx, store, args[1:])
	case "delete":
		return roundDelete(ctx, store, args[1:])
	default:
		return fmt.Errorf("unknown round subcommand %q", args[0])
	}
}

func roundAdd(ctx context.Context, store *database.Store, args []string) error {
	fs := flag.NewFlagSet("round add", flag.ContinueOnError)
	playerName := fs.String("player", "", "player name")
	courseName := fs.String("course", "", "course name")
	teeName := fs.String("tee", "", "tee name")
	date := fs.String("date", "", "date played, YYYY-MM-DD")
	handicapUsed := fs.Float64("handicap-used", 0, "handicap used for the round")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *playerName == "" || *courseName == "" || *teeName == "" || *date == "" {
		return fmt.Errorf("--player, --course, --tee and --date are all required")
	}
	if _, err := time.Parse("2006-01-02", *date); err != nil {
		return fmt.Errorf("--date must be in YYYY-MM-DD form, got %q", *date)
	}
	var suppliedHandicap *float64
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "handicap-used" {
			suppliedHandicap = handicapUsed
		}
	})

	tee, err := findTee(ctx, store, *courseName, *teeName)
	if err != nil {
		return err
	}
	player, found, err := findPlayer(ctx, store, *playerName)
	if err != nil {
		return err
	}
	if !found {
		player, err = store.CreatePlayer(ctx, *playerName, nil)
		if err != nil {
			return err
		}
	}

	laterCount, err := laterRoundCount(ctx, store, player.ID, *date)
	if err != nil {
		return err
	}
	fmt.Printf("%s playing %s (%s), CR %.1f / Slope %d.\n",
		player.Name, tee.CourseName, tee.Name, tee.Rating, tee.Slope)
	switch {
	case player.GroupHandicapIndex == nil:
		fmt.Println("No Handicap Index on file yet - hole caps use Par+5 (WHS initial-handicap rule).")
	default:
		dh := handicap.DailyHandicap(*player.GroupHandicapIndex, tee.Rating, tee.Slope, tee.TotalPar, player.HandicapCategory)
		fmt.Printf("Current Handicap Index: %.1f -> Daily Handicap %d\n", *player.GroupHandicapIndex, dh)
	}
	if laterCount > 0 {
		fmt.Printf("Note: %s has %d round(s) on file dated after %s. They'll be recalculated\n"+
			"afterward so their history correctly includes this one.\n", player.Name, laterCount, *date)
	}
	fmt.Println("Enter gross strokes for each hole (enter 0 if you picked up / didn't hole out):")

	var scores [18]int
	for i := range scores {
		scores[i], err = promptInt(fmt.Sprintf("  Hole %d (par %d, SI %d)", i+1, tee.Par[i], tee.StrokeIndex[i]))
		if err != nil {
			return fmt.Errorf("invalid score for hole %d: %w", i+1, err)
		}
	}

	created, err := store.CreateRound(ctx, database.CreateRoundInput{
		PlayedOn: *date,
		CourseID: tee.CourseID,
		Participants: []database.RoundEntry{{
			PlayerID: player.ID, TeeID: tee.ID, Scores: scores, HandicapUsed: suppliedHandicap,
		}},
	})
	if err != nil {
		return err
	}
	result := created.Participants[0]
	fmt.Printf("\nGross %d, Adjusted Gross %d, Score Differential %.1f\n",
		result.Gross, result.AdjustedGross, result.ScoreDifferential)
	if result.HandicapIndexAfter != nil {
		fmt.Printf("%s's updated Handicap Index: %.1f\n", player.Name, *result.HandicapIndexAfter)
	}
	if laterCount > 0 {
		fmt.Printf("Recalculated %s's %d later round(s).\n", player.Name, laterCount)
	}
	return nil
}

func roundList(ctx context.Context, store *database.Store, args []string) error {
	fs := flag.NewFlagSet("round list", flag.ContinueOnError)
	playerName := fs.String("player", "", "only show this player's rounds")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rounds, err := store.Rounds(ctx, 10000)
	if err != nil {
		return err
	}
	if len(rounds) == 0 {
		fmt.Println("No rounds recorded yet. Use 'golf round add'.")
		return nil
	}

	matches := 0
	for i := len(rounds) - 1; i >= 0; i-- {
		round := rounds[i]
		for _, participant := range round.Participants {
			if *playerName != "" && !strings.EqualFold(participant.PlayerName, *playerName) {
				continue
			}
			fmt.Printf("%2d  %-20s %-10s %-20s %-10s gross %3d  adj %3d  dh %2d  diff %5.1f\n",
				round.ID, participant.PlayerName, round.PlayedOn, round.CourseName, participant.TeeName,
				participant.Gross, participant.AdjustedGross, participant.DailyHandicap, participant.ScoreDifferential)
			matches++
		}
	}
	if matches == 0 {
		fmt.Printf("No rounds recorded for player %q.\n", *playerName)
	}
	return nil
}

func roundDelete(ctx context.Context, store *database.Store, args []string) error {
	yes := false
	var rest []string
	for _, arg := range args {
		if arg == "--yes" || arg == "-yes" {
			yes = true
		} else {
			rest = append(rest, arg)
		}
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: golf round delete ID [--yes]  (ID is shown by 'golf round list')")
	}
	id, err := strconv.ParseInt(rest[0], 10, 64)
	if err != nil {
		return fmt.Errorf("expected a round ID, got %q", rest[0])
	}
	round, err := store.Round(ctx, id)
	if err != nil {
		return fmt.Errorf("no round #%d", id)
	}
	names := participantNames(round)
	fmt.Printf("Round #%d: %s, %s, %s, gross %s\n",
		id, strings.Join(names, ", "), round.PlayedOn, round.CourseName, participantGrosses(round))
	if !yes {
		fmt.Print("Delete this round? [y/N]: ")
		line, _ := stdin.ReadString('\n')
		answer := strings.ToLower(strings.TrimSpace(line))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}
	if err := store.DeleteRound(ctx, id); err != nil {
		return err
	}
	fmt.Printf("Deleted round #%d.\n", id)
	return nil
}

func indexCmd(ctx context.Context, store *database.Store, args []string) error {
	fs := flag.NewFlagSet("index", flag.ContinueOnError)
	playerName := fs.String("player", "", "player name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *playerName == "" {
		return fmt.Errorf("--player is required")
	}
	player, found, err := findPlayer(ctx, store, *playerName)
	if err != nil {
		return err
	}
	if !found || player.RoundCount == 0 {
		fmt.Printf("No rounds recorded yet for %q.\n", *playerName)
		return nil
	}
	if player.GroupHandicapIndex == nil {
		fmt.Printf("%s has %d round(s) on file; %d are required to establish a Handicap Index.\n",
			player.Name, player.RoundCount, handicap.QualifyingRounds)
		return nil
	}
	fmt.Printf("%s's Handicap Index: %.1f  (from %d round(s) on file)\n",
		player.Name, *player.GroupHandicapIndex, player.RoundCount)
	return nil
}

func handicapCmd(ctx context.Context, store *database.Store, args []string) error {
	fs := flag.NewFlagSet("handicap", flag.ContinueOnError)
	playerName := fs.String("player", "", "player name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if *playerName == "" || len(rest) < 2 {
		return fmt.Errorf("usage: golf handicap --player NAME COURSE_NAME TEE")
	}
	player, found, err := findPlayer(ctx, store, *playerName)
	if err != nil {
		return err
	}
	if !found || player.RoundCount == 0 {
		return fmt.Errorf("no rounds recorded yet for %q, so no Handicap Index to convert", *playerName)
	}
	if player.GroupHandicapIndex == nil {
		return fmt.Errorf("%q has only %d round(s); %d are required to establish a Handicap Index",
			player.Name, player.RoundCount, handicap.QualifyingRounds)
	}
	tee, err := findTee(ctx, store, rest[0], rest[1])
	if err != nil {
		return err
	}
	dh := handicap.DailyHandicap(*player.GroupHandicapIndex, tee.Rating, tee.Slope, tee.TotalPar, player.HandicapCategory)
	fmt.Printf("%s's Handicap Index %.1f on %s (%s, Slope %d) -> Daily Handicap %d\n",
		player.Name, *player.GroupHandicapIndex, tee.CourseName, tee.Name, tee.Slope, dh)
	return nil
}

func playerCmd(ctx context.Context, store *database.Store, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: golf player list")
	}
	switch args[0] {
	case "list":
		return playerList(ctx, store)
	default:
		return fmt.Errorf("unknown player subcommand %q", args[0])
	}
}

func playerList(ctx context.Context, store *database.Store) error {
	players, err := store.Players(ctx)
	if err != nil {
		return err
	}
	if len(players) == 0 {
		fmt.Println("No players recorded yet.")
		return nil
	}
	fmt.Printf("%-20s %-8s %s\n", "PLAYER", "INDEX", "ROUNDS")
	for _, player := range players {
		index := "-"
		if player.GroupHandicapIndex != nil {
			index = fmt.Sprintf("%.1f", *player.GroupHandicapIndex)
		}
		fmt.Printf("%-20s %-8s %d round(s)\n", player.Name, index, player.RoundCount)
	}
	return nil
}

func recalculateCmd(ctx context.Context, store *database.Store, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: golf recalculate")
	}
	if err := store.RecalculateAll(ctx); err != nil {
		return err
	}
	players, err := store.Players(ctx)
	if err != nil {
		return err
	}
	roundCount := 0
	playerCount := 0
	for _, player := range players {
		roundCount += player.RoundCount
		if player.RoundCount > 0 {
			playerCount++
		}
	}
	if roundCount == 0 {
		fmt.Println("No rounds recorded yet.")
		return nil
	}
	fmt.Printf("Recalculated %d player round(s) across %d player(s):\n", roundCount, playerCount)
	for _, player := range players {
		if player.RoundCount == 0 {
			continue
		}
		index := "-"
		if player.GroupHandicapIndex != nil {
			index = fmt.Sprintf("%.1f", *player.GroupHandicapIndex)
		}
		fmt.Printf("  %-20s Handicap Index %s\n", player.Name, index)
	}
	return nil
}

func findPlayer(ctx context.Context, store *database.Store, name string) (database.Player, bool, error) {
	players, err := store.Players(ctx)
	if err != nil {
		return database.Player{}, false, err
	}
	for _, player := range players {
		if strings.EqualFold(player.Name, strings.TrimSpace(name)) {
			return player, true, nil
		}
	}
	return database.Player{}, false, nil
}

func findTee(ctx context.Context, store *database.Store, courseName, teeName string) (database.Tee, error) {
	courses, err := store.Courses(ctx)
	if err != nil {
		return database.Tee{}, err
	}
	for _, course := range courses {
		if !strings.EqualFold(course.Name, strings.TrimSpace(courseName)) {
			continue
		}
		for _, tee := range course.Tees {
			if strings.EqualFold(tee.Name, strings.TrimSpace(teeName)) {
				return tee, nil
			}
		}
	}
	return database.Tee{}, fmt.Errorf("no course %q tee %q defined", courseName, teeName)
}

func laterRoundCount(ctx context.Context, store *database.Store, playerID int64, date string) (int, error) {
	rounds, err := store.Rounds(ctx, 10000)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, round := range rounds {
		if round.PlayedOn <= date {
			continue
		}
		for _, participant := range round.Participants {
			if participant.PlayerID == playerID {
				count++
				break
			}
		}
	}
	return count, nil
}

func participantNames(round database.Round) []string {
	names := make([]string, 0, len(round.Participants))
	for _, participant := range round.Participants {
		names = append(names, participant.PlayerName)
	}
	return names
}

func participantGrosses(round database.Round) string {
	values := make([]string, 0, len(round.Participants))
	for _, participant := range round.Participants {
		values = append(values, fmt.Sprintf("%s %d", participant.PlayerName, participant.Gross))
	}
	return strings.Join(values, ", ")
}

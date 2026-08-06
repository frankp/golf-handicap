package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"

	"golf/internal/database"
	"golf/internal/handicap"
)

func main() {
	jsonPath := flag.String("json", "golf-data.json", "legacy JSON data file")
	databasePath := flag.String("db", "golf.db", "destination SQLite database")
	flag.Parse()

	raw, err := os.ReadFile(*jsonPath)
	if err != nil {
		log.Fatal(err)
	}
	var legacy handicap.Data
	if err := json.Unmarshal(raw, &legacy); err != nil {
		log.Fatal(err)
	}

	store, err := database.Open(*databasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	empty, err := store.Empty(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if !empty {
		log.Fatal("destination database is not empty; refusing to import")
	}

	teeIDs := map[string]int64{}
	courseIDs := map[string]int64{}
	for _, course := range legacy.Courses {
		tee, err := store.CreateTee(ctx, course)
		if err != nil {
			log.Fatalf("import course %s (%s): %v", course.Name, course.Tee, err)
		}
		teeIDs[course.Key()] = tee.ID
		courseIDs[course.Name] = tee.CourseID
	}

	playerNames := map[string]bool{}
	for _, round := range legacy.Rounds {
		playerNames[round.Player] = true
	}
	for name := range legacy.StartingHandicaps {
		playerNames[name] = true
	}
	names := make([]string, 0, len(playerNames))
	for name := range playerNames {
		names = append(names, name)
	}
	sort.Strings(names)
	playerIDs := map[string]int64{}
	for _, name := range names {
		player, err := store.CreatePlayer(ctx, name, nil)
		if err != nil {
			log.Fatalf("import player %s: %v", name, err)
		}
		playerIDs[name] = player.ID
	}

	type groupedRound struct {
		date         string
		courseName   string
		participants []database.RoundEntry
	}
	grouped := map[string]*groupedRound{}
	var keys []string
	for _, round := range legacy.Rounds {
		key := round.Date + "\x00" + round.CourseName
		item := grouped[key]
		if item == nil {
			item = &groupedRound{date: round.Date, courseName: round.CourseName}
			grouped[key] = item
			keys = append(keys, key)
		}
		item.participants = append(item.participants, database.RoundEntry{
			PlayerID: playerIDs[round.Player],
			TeeID:    teeIDs[round.CourseName+"|"+round.Tee],
			Scores:   round.Scores,
		})
	}
	sort.SliceStable(keys, func(i, j int) bool {
		return grouped[keys[i]].date < grouped[keys[j]].date
	})
	for _, key := range keys {
		item := grouped[key]
		_, err := store.CreateRound(ctx, database.CreateRoundInput{
			PlayedOn: item.date, CourseID: courseIDs[item.courseName], Participants: item.participants,
		})
		if err != nil {
			log.Fatalf("import round %s at %s: %v", item.date, item.courseName, err)
		}
	}

	fmt.Printf("Imported %d player(s), %d tee(s), and %d group round(s) into %s.\n",
		len(playerIDs), len(teeIDs), len(keys), *databasePath)
}

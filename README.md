# golf

A small command-line tool that tracks golf scores and computes a
[World Handicap System (WHS)](https://www.usga.org/handicapping.html)
Handicap Index and Course Handicap for one or more players.

The primary interface is now a private web application backed by SQLite.
The CLI uses the same SQLite database. Legacy JSON storage is supported
only by the one-time importer.

## Web application

Build the Vue frontend and start the Go server:

```sh
cd web
npm install
npm run build
cd ..

go run ./cmd/server
```

Open [http://localhost:8080](http://localhost:8080). The interface is
responsive and includes a one-hole-at-a-time score entry layout for phones.
To use it from a phone on the same network, open port 8080 using the
computer's local network address.

The web application uses `golf.db` by default. Override the database or
listen address with `GOLF_DB` and `GOLF_ADDR`.

To import a legacy JSON file into a new, empty database:

```sh
go run ./cmd/import-json --json golf-data.json --db golf.db
```

The importer retains raw hole scores, courses, tees, players, nominated
starting handicaps, and recalculates all derived handicap values.

For frontend development with hot reload, run the API and Vite separately:

```sh
go run ./cmd/server
cd web && npm run dev
```

Then open [http://localhost:5173](http://localhost:5173).

## CLI

## Quick start

```sh
go build -o golf .

./golf course add                 # define a course/tee once
./golf round add --player Frank --course "Pine Valley" --tee White --date 2026-07-22
./golf index --player Frank
./golf handicap --player Frank "Pine Valley" White
```

## Concepts

**Course / Tee** — a specific set of tees on a course: its 18-hole Course
Rating, Slope Rating, and the par + stroke index of every hole. Defined
once with `golf course add`, then reused by every round played there,
by any player.

**Round** — one 18-hole round played by one player on one course/tee, on
a given date. You enter the gross score for each hole.

**Player** — just a name attached to a round. There's no separate
registration step; the first round you add for a name creates that
player implicitly.

**Adjusted Gross Score** — your round score after capping each hole at
its **Net Double Bogey** maximum (see below). This is what actually goes
into the handicap calculation, not your raw gross score.

**Score Differential** — a single round's contribution to your handicap
record, adjusted for the difficulty of the course you played:

```
Score Differential = (113 / Slope Rating) × (Adjusted Gross Score − Course Rating)
```

**Handicap Index** — your overall number, derived from your best recent
Score Differentials (see [How the Handicap Index is calculated](#how-the-handicap-index-is-calculated)).

**Course Handicap** — your Handicap Index converted to a whole number of
strokes for a specific course/tee:

```
Course Handicap = round(Handicap Index × Slope Rating / 113 + Course Rating − par)
```

## How the Net Double Bogey cap works

Before a hole score is used for handicap purposes, it's capped at:

```
Par + 2 + handicap strokes received on that hole
```

A player receives a stroke on a hole once their Course Handicap reaches
that hole's Stroke Index, a second once it reaches `Stroke Index + 18`,
and a third once it reaches `Stroke Index + 36`. So a 40-handicapper
receives two strokes everywhere and a third on the four hardest holes.

Entering `0` for a hole (picked up / didn't hole out) is treated as "at
least the cap" and scored as the cap.

**Exception:** a Handicap Index is not established until a player has
submitted three 18-hole scores. During those first three rounds, WHS's
initial-handicap rule caps every hole at `Par + 5`. A nominated starting
Course Handicap can be used instead.

## How the Handicap Index is calculated

WHS Rule 5.2a: take your most recent 20 Score Differentials (or fewer, if
you don't have 20 rounds yet). Average the lowest few of them, apply a
small adjustment if you have a very short scoring history, and round to
one decimal place.

| Rounds on file | Differentials averaged | Adjustment |
|---:|---:|---:|
| 3 | lowest 1 | −2.0 |
| 4 | lowest 1 | −1.0 |
| 5 | lowest 1 | 0 |
| 6 | lowest 2 | −1.0 |
| 7–8 | lowest 2 | 0 |
| 9–11 | lowest 3 | 0 |
| 12–14 | lowest 4 | 0 |
| 15–16 | lowest 5 | 0 |
| 17–18 | lowest 6 | 0 |
| 19 | lowest 7 | 0 |
| 20+ | lowest 8 (of the most recent 20) | 0 |

There is **no 0.96 multiplier** in the current version of WHS — that's a
detail from an earlier iteration that no longer applies. The table above
*is* the whole calculation of the **raw** index — but see the next
section for the cap applied on top of it.

## The soft cap and hard cap (Rule 5.8)

WHS limits how fast a Handicap Index is allowed to *rise*, even if recent
scores say it should rise faster. This is measured against the player's
**Low Handicap Index**: the lowest Handicap Index they've had in the
trailing 365 days. A Low Handicap Index is first established once the
player has 20 scores, so the caps do not apply before then.

- If the newly calculated (raw) index is **more than 3.0** above the Low
  Handicap Index, only half of the excess above 3.0 counts (the **soft
  cap**).
- The total rise is never allowed to exceed **5.0** above the Low
  Handicap Index, no matter how bad the scores (the **hard cap**).

```
rise = rawIndex − lowIndex
if rise <= 3.0:  effectiveIndex = rawIndex
else:            effectiveIndex = min(lowIndex + 3.0 + 0.5×(rise − 3.0), lowIndex + 5.0)
```

Decreases are never capped — only rises. This effective (capped) index is
what actually determines a player's Course Handicap for their *next*
round, and it's what `golf index` and `golf player list` report — not the
uncapped Rule 5.2a average.

## Commands

```
golf course add                     Define a course/tee, prompting for
                                     rating, slope, and each hole's par + SI
golf course list                    List defined courses/tees

golf player list                    List players who have recorded rounds

golf round add --player NAME --course COURSE --tee TEE --date YYYY-MM-DD
                                     Record a round, entering each hole's score
golf round list [--player NAME]     List recorded rounds (optionally one player's)
golf round delete ID [--yes]        Delete the round ID shown by 'round list',
                                     asking for confirmation unless --yes is given

golf index --player NAME            Show a player's current Handicap Index
golf handicap --player NAME COURSE TEE
                                     Show a player's Course Handicap for a course/tee
golf recalculate                    Recompute every round's cached stats in true
                                     date order, for every player (safe to re-run anytime)
```

The CLI and web application both use `./golf.db`. Override the location
with `GOLF_DB`, e.g. `GOLF_DB=~/golf.db golf index --player Frank`.

**Rounds can be added in any order.** `--date` (required) is the real
source of truth for chronological order, not the order you happen to run
`golf round add` in. Adding an old round after newer ones are already on
file — backfilling — automatically recalculates every later round for
that player so their Course Handicap and Score Differential correctly
reflect the newly-inserted history. `golf round add` tells you when this
happens (`Recalculated <player>'s N later round(s)`), and `golf round
list` sorts oldest-first regardless of entry order.

**Deleting a round** (`golf round delete ID`) removes that round and then
replays the affected players' remaining rounds in date order, recomputing
each Course Handicap, Adjusted Gross Score, and Score Differential from
the raw hole scores. A web-entered group round contains multiple players;
deleting its ID removes the complete group round.

**`golf recalculate`** does the same replay for every player's rounds in
one pass. It is a no-op if nothing is out of sync, so it is always safe
to run.

## Known simplifications

This is a personal tool, not an accredited WHS implementation. It
intentionally skips some things full handicap software would handle:

- **No Playing Conditions Calculation (PCC).** Real WHS software adjusts
  every score slightly based on how the whole field played that day
  (unusual weather, etc.). This tool always assumes PCC = 0.
- **Plus-handicap players aren't modeled.** A negative Handicap Index is
  computed correctly, but the Net Double Bogey stroke allocation clamps
  at 0 strokes received rather than modeling strokes *given*.
- **9-hole rounds aren't supported** — every round assumes a complete 18
  holes.
- **No exceptional-score handling, penalty scores, or committee review**
  — features aimed at club administration rather than personal tracking.

None of these affect the core math for a normal 18-hole round from a
single-digit-to-30-something handicap golfer, which is the common case
this tool is built for.

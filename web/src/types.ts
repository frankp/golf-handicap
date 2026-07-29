export interface Player {
  id: number
  name: string
  startingCourseHandicap: number | null
  officialHandicapIndex: number | null
  officialHandicapDate: string | null
  groupHandicapIndex: number | null
  roundCount: number
}

export interface Tee {
  id: number
  courseId: number
  courseName: string
  name: string
  rating: number
  slope: number
  par: number[]
  strokeIndex: number[]
  totalPar: number
}

export interface Course {
  id: number
  name: string
  tees: Tee[]
}

export interface RoundPlayer {
  id: number
  playerId: number
  playerName: string
  teeId: number
  teeName: string
  scores: number[]
  gross: number
  handicapUsed: number | null
  netScore: number | null
  netScores: number[] | null
  courseHandicap: number
  adjustedGross: number
  scoreDifferential: number
  handicapIndexAfter: number | null
  startingHandicapUsed: boolean
  initialParFiveCapUsed: boolean
  counting: boolean
}

export interface Round {
  id: number
  playedOn: string
  courseId: number
  courseName: string
  notes: string
  participants: RoundPlayer[]
}

export interface PlayerDetail {
  player: Player
  rounds: Round[]
}

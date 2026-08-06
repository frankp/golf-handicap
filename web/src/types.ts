export type HandicapCategory = 'men' | 'women'

export interface Player {
  id: number
  name: string
  handicapCategory: HandicapCategory
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
  netScore: number
  netScores: number[]
  dailyHandicap: number
  adjustedGross: number
  scoreDifferential: number
  handicapIndexAfter: number | null
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

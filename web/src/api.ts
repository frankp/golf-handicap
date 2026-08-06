import type { Course, HandicapCategory, Player, PlayerDetail, Round } from './types'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
  })
  if (response.status === 401) {
    window.dispatchEvent(new Event('golf:unauthorized'))
  }
  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: response.statusText }))
    throw new Error(body.error || 'Request failed')
  }
  if (response.status === 204) return undefined as T
  return response.json()
}

export const api = {
  authSession: () => request<{ authenticated: boolean; enabled: boolean }>('/api/auth/session'),
  login: (password: string) =>
    request<{ authenticated: boolean }>('/api/auth/login', { method: 'POST', body: JSON.stringify({ password }) }),
  logout: () => request<void>('/api/auth/logout', { method: 'POST' }),
  players: () => request<Player[]>('/api/players'),
  player: (id: number) => request<PlayerDetail>(`/api/players/${id}`),
  createPlayer: (body: { name: string; handicapCategory: HandicapCategory }) =>
    request<Player>('/api/players', { method: 'POST', body: JSON.stringify(body) }),
  updatePlayer: (id: number, body: {
    name: string
    handicapCategory: HandicapCategory
    officialHandicapIndex: number | null
    officialHandicapDate: string | null
  }) => request<Player>(`/api/players/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deletePlayer: (id: number) => request<void>(`/api/players/${id}`, { method: 'DELETE' }),
  courses: () => request<Course[]>('/api/courses'),
  createTee: (body: {
    name: string
    tee: string
    rating: number
    slope: number
    par: number[]
    strokeIndex: number[]
  }) => request('/api/courses', { method: 'POST', body: JSON.stringify(body) }),
  updateCourse: (id: number, body: { name: string }) =>
    request<Course>(`/api/courses/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  updateTee: (id: number, body: {
    name: string
    tee: string
    rating: number
    slope: number
    par: number[]
    strokeIndex: number[]
  }) => request(`/api/tees/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteTee: (id: number) => request<void>(`/api/tees/${id}`, { method: 'DELETE' }),
  rounds: (limit = 100) => request<Round[]>(`/api/rounds?limit=${limit}`),
  round: (id: number) => request<Round>(`/api/rounds/${id}`),
  createRound: (body: {
    playedOn: string
    courseId: number
    notes: string
    participants: { playerId: number; teeId: number; scores: number[]; handicapUsed: number | null }[]
  }) => request<Round>('/api/rounds', { method: 'POST', body: JSON.stringify(body) }),
  updateRound: (id: number, body: {
    playedOn: string
    courseId: number
    notes: string
    participants: { playerId: number; teeId: number; scores: number[]; handicapUsed: number | null }[]
  }) => request<Round>(`/api/rounds/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteRound: (id: number) => request<void>(`/api/rounds/${id}`, { method: 'DELETE' }),
}

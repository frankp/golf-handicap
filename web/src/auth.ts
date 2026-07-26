import { reactive } from 'vue'
import { api } from './api'

export const authState = reactive({
  authenticated: false,
  enabled: false,
  ready: false,
})

let initialization: Promise<void> | null = null

export function initializeAuth() {
  if (authState.ready) return Promise.resolve()
  if (initialization) return initialization
  initialization = api.authSession()
    .then((session) => {
      authState.authenticated = session.authenticated
      authState.enabled = session.enabled
    })
    .catch(() => {
      authState.authenticated = false
      authState.enabled = false
    })
    .finally(() => {
      authState.ready = true
      initialization = null
    })
  return initialization
}

export async function login(password: string) {
  await api.login(password)
  authState.authenticated = true
  authState.enabled = true
}

export async function logout() {
  await api.logout()
  authState.authenticated = false
}

if (typeof window !== 'undefined') {
  window.addEventListener('golf:unauthorized', () => {
    authState.authenticated = false
  })
}

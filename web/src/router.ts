import { createRouter, createWebHistory } from 'vue-router'
import DashboardView from './views/DashboardView.vue'
import PlayersView from './views/PlayersView.vue'
import PlayerView from './views/PlayerView.vue'
import CoursesView from './views/CoursesView.vue'
import RoundsView from './views/RoundsView.vue'
import RoundView from './views/RoundView.vue'
import NewRoundView from './views/NewRoundView.vue'
import LoginView from './views/LoginView.vue'
import { authState, initializeAuth } from './auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: DashboardView },
    { path: '/login', component: LoginView },
    { path: '/players', component: PlayersView },
    { path: '/players/:id', component: PlayerView },
    { path: '/courses', component: CoursesView },
    { path: '/rounds', component: RoundsView },
    { path: '/rounds/new', component: NewRoundView, meta: { requiresAuth: true } },
    { path: '/rounds/:id/edit', component: NewRoundView, meta: { requiresAuth: true } },
    { path: '/rounds/:id', component: RoundView },
  ],
})

router.beforeEach(async (to) => {
  await initializeAuth()
  if (to.meta.requiresAuth && !authState.authenticated) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }
  if (to.path === '/login' && authState.authenticated) {
    return '/'
  }
})

export default router

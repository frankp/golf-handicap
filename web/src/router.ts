import { createRouter, createWebHistory } from 'vue-router'
import DashboardView from './views/DashboardView.vue'
import PlayersView from './views/PlayersView.vue'
import PlayerView from './views/PlayerView.vue'
import CoursesView from './views/CoursesView.vue'
import RoundsView from './views/RoundsView.vue'
import RoundView from './views/RoundView.vue'
import NewRoundView from './views/NewRoundView.vue'

export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: DashboardView },
    { path: '/players', component: PlayersView },
    { path: '/players/:id', component: PlayerView },
    { path: '/courses', component: CoursesView },
    { path: '/rounds', component: RoundsView },
    { path: '/rounds/new', component: NewRoundView },
    { path: '/rounds/:id/edit', component: NewRoundView },
    { path: '/rounds/:id', component: RoundView },
  ],
})

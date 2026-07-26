<script setup lang="ts">
import { Gauge, Users, MapPinned, ListChecks, LogIn, LogOut, Plus, Menu, X } from '@lucide/vue'
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { authState, logout } from './auth'

const menuOpen = ref(false)
const router = useRouter()
const links = [
  { to: '/', label: 'Overview', icon: Gauge },
  { to: '/players', label: 'Players', icon: Users },
  { to: '/rounds', label: 'Rounds', icon: ListChecks },
  { to: '/courses', label: 'Courses', icon: MapPinned },
]

async function signOut() {
  await logout()
  menuOpen.value = false
  await router.push('/')
}
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar" :class="{ open: menuOpen }">
      <div class="brand">
        <span class="brand-mark">HL</span>
        <div>
          <strong>Handicap Ledger</strong>
          <small>Group scoring record</small>
        </div>
        <button class="icon-button mobile-only" title="Close menu" @click="menuOpen = false"><X :size="20" /></button>
      </div>
      <nav aria-label="Primary navigation">
        <RouterLink v-for="link in links" :key="link.to" :to="link.to" @click="menuOpen = false">
          <component :is="link.icon" :size="19" />
          {{ link.label }}
        </RouterLink>
      </nav>
      <div class="sidebar-footer">
        <RouterLink v-if="authState.authenticated" class="button sidebar-action" to="/rounds/new" @click="menuOpen = false">
          <Plus :size="18" /> Record round
        </RouterLink>
        <button v-if="authState.authenticated" class="sidebar-auth" @click="signOut"><LogOut :size="17" /> Sign out</button>
        <RouterLink v-else class="sidebar-auth" to="/login" @click="menuOpen = false"><LogIn :size="17" /> Admin sign in</RouterLink>
      </div>
    </aside>
    <div v-if="menuOpen" class="nav-scrim" @click="menuOpen = false" />
    <main class="content-shell">
      <header class="mobile-header">
        <button class="icon-button" title="Open menu" @click="menuOpen = true"><Menu :size="22" /></button>
        <strong>Handicap Ledger</strong>
        <RouterLink v-if="authState.authenticated" class="icon-button primary-icon" to="/rounds/new" title="Record round"><Plus :size="22" /></RouterLink>
        <RouterLink v-else class="icon-button" to="/login" title="Admin sign in"><LogIn :size="21" /></RouterLink>
      </header>
      <RouterView />
    </main>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ArrowRight, Pencil, Plus, Trash2 } from '@lucide/vue'
import { api } from '@/api'
import type { HandicapCategory, Player } from '@/types'
import AppModal from '@/components/AppModal.vue'
import EmptyState from '@/components/EmptyState.vue'
import { authState } from '@/auth'

const players = ref<Player[]>([])
const loading = ref(true)
const error = ref('')
const showAdd = ref(false)
const editing = ref<Player | null>(null)
const saving = ref(false)
const form = ref<{ name: string; category: HandicapCategory; starting: string; official: string; officialDate: string }>({
  name: '', category: 'men', starting: '', official: '', officialDate: '',
})

onMounted(load)

async function load() {
  loading.value = true
  try {
    players.value = await api.players()
    error.value = ''
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    loading.value = false
  }
}

function openAdd() {
  form.value = { name: '', category: 'men', starting: '', official: '', officialDate: '' }
  showAdd.value = true
}

function openEdit(player: Player) {
  editing.value = player
  form.value = {
    name: player.name,
    category: player.handicapCategory,
    starting: player.startingDailyHandicap?.toString() ?? '',
    official: player.officialHandicapIndex?.toString() ?? '',
    officialDate: player.officialHandicapDate ?? '',
  }
}

async function saveAdd() {
  saving.value = true
  try {
    await api.createPlayer({
      name: form.value.name,
      handicapCategory: form.value.category,
      startingDailyHandicap: form.value.starting === '' ? null : Number(form.value.starting),
    })
    showAdd.value = false
    await load()
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    saving.value = false
  }
}

async function saveEdit() {
  if (!editing.value) return
  saving.value = true
  try {
    await api.updatePlayer(editing.value.id, {
      name: form.value.name,
      handicapCategory: form.value.category,
      startingDailyHandicap: form.value.starting === '' ? null : Number(form.value.starting),
      officialHandicapIndex: form.value.official === '' ? null : Number(form.value.official),
      officialHandicapDate: form.value.officialDate || null,
    })
    editing.value = null
    await load()
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    saving.value = false
  }
}

async function remove(player: Player) {
  if (!confirm(`Delete ${player.name}?`)) return
  try {
    await api.deletePlayer(player.id)
    await load()
  } catch (err) {
    error.value = (err as Error).message
  }
}
</script>

<template>
  <div class="page">
    <header class="page-header">
      <div><p class="eyebrow">Group record</p><h1>Players</h1></div>
      <button v-if="authState.authenticated" class="button" @click="openAdd"><Plus :size="18" /> Add player</button>
    </header>
    <p v-if="error" class="alert error">{{ error }}</p>
    <div v-if="loading" class="loading">Loading players...</div>
    <EmptyState v-else-if="players.length === 0" title="No players yet" detail="Add your regular playing group.">
      <button v-if="authState.authenticated" class="button" @click="openAdd">Add player</button>
    </EmptyState>
    <div v-else class="table-wrap">
      <table>
        <thead><tr><th>Player</th><th>Category</th><th>Group HI</th><th>Official HI</th><th>Starting DH</th><th>Rounds</th><th></th></tr></thead>
        <tbody>
          <tr v-for="player in players" :key="player.id">
            <td><RouterLink class="primary-link" :to="`/players/${player.id}`">{{ player.name }}</RouterLink></td>
            <td>{{ player.handicapCategory === 'women' ? 'Women/Girls' : 'Men/Boys' }}</td>
            <td><strong>{{ player.groupHandicapIndex?.toFixed(1) ?? 'Pending' }}</strong></td>
            <td>{{ player.officialHandicapIndex?.toFixed(1) ?? '—' }}</td>
            <td>{{ player.startingDailyHandicap ?? '—' }}</td>
            <td>{{ player.roundCount }}</td>
            <td class="button-cluster">
              <button v-if="authState.authenticated" class="icon-button" title="Edit player" @click="openEdit(player)"><Pencil :size="17" /></button>
              <button v-if="authState.authenticated" class="icon-button danger" title="Delete player" :disabled="player.roundCount > 0" @click="remove(player)"><Trash2 :size="17" /></button>
              <RouterLink class="icon-button" :to="`/players/${player.id}`" title="View player"><ArrowRight :size="17" /></RouterLink>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <AppModal v-if="showAdd" title="Add player" @close="showAdd = false">
      <form class="form-stack" @submit.prevent="saveAdd">
        <label>Player name<input v-model.trim="form.name" required autofocus /></label>
        <label>Handicap category<select v-model="form.category"><option value="men">Men/Boys</option><option value="women">Women/Girls</option></select></label>
        <label>Starting Daily Handicap <span class="optional">Optional</span><input v-model="form.starting" type="number" min="0" max="99" /></label>
        <div class="form-actions"><button type="button" class="button ghost" @click="showAdd = false">Cancel</button><button class="button" :disabled="saving">Add player</button></div>
      </form>
    </AppModal>

    <AppModal v-if="editing" title="Edit player" @close="editing = null">
      <form class="form-stack" @submit.prevent="saveEdit">
        <label>Player name<input v-model.trim="form.name" required /></label>
        <label>Handicap category<select v-model="form.category"><option value="men">Men/Boys</option><option value="women">Women/Girls</option></select></label>
        <label>Starting Daily Handicap <span class="optional">First three rounds</span><input v-model="form.starting" type="number" min="0" max="99" /></label>
        <div class="field-pair">
          <label>Official Handicap Index <span class="optional">Reference only</span><input v-model="form.official" type="number" min="-10" max="54" step="0.1" /></label>
          <label>Official index date<input v-model="form.officialDate" type="date" :required="form.official !== ''" /></label>
        </div>
        <div class="form-actions"><button type="button" class="button ghost" @click="editing = null">Cancel</button><button class="button" :disabled="saving">Save changes</button></div>
      </form>
    </AppModal>
  </div>
</template>

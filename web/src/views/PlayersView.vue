<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ArrowRight, Pencil, Plus, Trash2 } from '@lucide/vue'
import { api } from '@/api'
import type { HandicapCategory, Player } from '@/types'
import AppField from '@/components/AppField.vue'
import AppModal from '@/components/AppModal.vue'
import AppNumberField from '@/components/AppNumberField.vue'
import AppSelect from '@/components/AppSelect.vue'
import EmptyState from '@/components/EmptyState.vue'
import { authState } from '@/auth'

const handicapCategories: ReadonlyArray<{ value: HandicapCategory; label: string }> = [
  { value: 'men', label: 'Men/Boys' },
  { value: 'women', label: 'Women/Girls' },
]

const players = ref<Player[]>([])
const loading = ref(true)
const error = ref('')
const showAdd = ref(false)
const editing = ref<Player | null>(null)
const saving = ref(false)
const form = ref<{ name: string; category: HandicapCategory; official?: number; officialDate: string }>({
  name: '', category: 'men', official: undefined, officialDate: '',
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
  form.value = { name: '', category: 'men', official: undefined, officialDate: '' }
  showAdd.value = true
}

function openEdit(player: Player) {
  editing.value = player
  form.value = {
    name: player.name,
    category: player.handicapCategory,
    official: player.officialHandicapIndex ?? undefined,
    officialDate: player.officialHandicapDate ?? '',
  }
}

async function saveAdd() {
  saving.value = true
  try {
    await api.createPlayer({
      name: form.value.name,
      handicapCategory: form.value.category,
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
      officialHandicapIndex: form.value.official ?? null,
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
        <thead><tr><th>Player</th><th>Category</th><th>Group HI</th><th>Official HI</th><th>Rounds</th><th></th></tr></thead>
        <tbody>
          <tr v-for="player in players" :key="player.id">
            <td><RouterLink class="primary-link" :to="`/players/${player.id}`">{{ player.name }}</RouterLink></td>
            <td>{{ player.handicapCategory === 'women' ? 'Women/Girls' : 'Men/Boys' }}</td>
            <td><strong>{{ player.groupHandicapIndex?.toFixed(1) ?? 'Pending' }}</strong></td>
            <td>{{ player.officialHandicapIndex?.toFixed(1) ?? '—' }}</td>
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
        <AppField input-id="add-player-name" label="Player name">
          <input id="add-player-name" v-model.trim="form.name" required autofocus />
        </AppField>
        <AppField input-id="add-player-category" label="Handicap category">
          <AppSelect id="add-player-category" v-model="form.category" :options="handicapCategories" />
        </AppField>
        <div class="form-actions"><button type="button" class="button ghost" @click="showAdd = false">Cancel</button><button class="button" :disabled="saving">Add player</button></div>
      </form>
    </AppModal>

    <AppModal v-if="editing" title="Edit player" @close="editing = null">
      <form class="form-stack" @submit.prevent="saveEdit">
        <AppField input-id="edit-player-name" label="Player name">
          <input id="edit-player-name" v-model.trim="form.name" required />
        </AppField>
        <AppField input-id="edit-player-category" label="Handicap category">
          <AppSelect id="edit-player-category" v-model="form.category" :options="handicapCategories" />
        </AppField>
        <div class="field-pair">
          <AppField input-id="edit-player-official-handicap" label="Official Handicap Index" hint="Reference only">
            <AppNumberField id="edit-player-official-handicap" v-model="form.official" :min="-10" :max="54" :step="0.1" />
          </AppField>
          <AppField input-id="edit-player-official-date" label="Official index date">
            <input id="edit-player-official-date" v-model="form.officialDate" type="date" :required="form.official !== undefined" />
          </AppField>
        </div>
        <div class="form-actions"><button type="button" class="button ghost" @click="editing = null">Cancel</button><button class="button" :disabled="saving">Save changes</button></div>
      </form>
    </AppModal>
  </div>
</template>

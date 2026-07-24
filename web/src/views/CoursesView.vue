<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ChevronDown, ChevronUp, Pencil, Plus, Trash2 } from '@lucide/vue'
import { api } from '@/api'
import type { Course, Tee } from '@/types'
import AppModal from '@/components/AppModal.vue'
import EmptyState from '@/components/EmptyState.vue'

interface TeeForm {
  name: string
  tee: string
  rating: number
  slope: number
  par: number[]
  strokeIndex: number[]
}

const courses = ref<Course[]>([])
const loading = ref(true)
const error = ref('')
const showEditor = ref(false)
const editingTeeId = ref<number | null>(null)
const editingCourse = ref<Course | null>(null)
const courseName = ref('')
const saving = ref(false)
const expanded = ref<number | null>(null)
const form = ref(newForm())
const validStrokeIndexes = computed(() => new Set(form.value.strokeIndex).size === 18 && form.value.strokeIndex.every((v) => v >= 1 && v <= 18))

function newForm(): TeeForm {
  return {
    name: '',
    tee: '',
    rating: 72,
    slope: 113,
    par: Array.from({ length: 18 }, (_, i) => ([3, 7, 11, 15].includes(i) ? 3 : [1, 5, 9, 13, 17].includes(i) ? 5 : 4)),
    strokeIndex: Array.from({ length: 18 }, (_, i) => i + 1),
  }
}

onMounted(load)
async function load() {
  loading.value = true
  try {
    courses.value = await api.courses()
    error.value = ''
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    loading.value = false
  }
}

function openAdd(courseName = '') {
  editingTeeId.value = null
  form.value = newForm()
  form.value.name = courseName
  showEditor.value = true
}

function openEditTee(course: Course, tee: Tee) {
  editingTeeId.value = tee.id
  form.value = {
    name: course.name,
    tee: tee.name,
    rating: tee.rating,
    slope: tee.slope,
    par: [...tee.par],
    strokeIndex: [...tee.strokeIndex],
  }
  showEditor.value = true
}

function openEditCourse(course: Course) {
  editingCourse.value = course
  courseName.value = course.name
}

async function save() {
  if (!validStrokeIndexes.value) {
    error.value = 'Stroke indexes must contain every number from 1 to 18 once.'
    return
  }
  saving.value = true
  try {
    if (editingTeeId.value === null) {
      await api.createTee(form.value)
    } else {
      await api.updateTee(editingTeeId.value, form.value)
    }
    showEditor.value = false
    await load()
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    saving.value = false
  }
}

async function saveCourse() {
  if (!editingCourse.value) return
  saving.value = true
  try {
    await api.updateCourse(editingCourse.value.id, { name: courseName.value })
    editingCourse.value = null
    await load()
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    saving.value = false
  }
}

async function removeTee(id: number, label: string) {
  if (!confirm(`Delete ${label}?`)) return
  try {
    await api.deleteTee(id)
    await load()
  } catch (err) {
    error.value = (err as Error).message
  }
}
</script>

<template>
  <div class="page">
    <header class="page-header">
      <div><p class="eyebrow">Course register</p><h1>Courses & tees</h1></div>
      <button class="button" @click="openAdd()"><Plus :size="18" /> Add tee</button>
    </header>
    <p v-if="error" class="alert error">{{ error }}</p>
    <div v-if="loading" class="loading">Loading courses...</div>
    <EmptyState v-else-if="courses.length === 0" title="No courses configured" detail="Add the first course and tee before recording a round.">
      <button class="button" @click="openAdd()">Add course</button>
    </EmptyState>
    <div v-else class="course-list">
      <section v-for="course in courses" :key="course.id" class="course-band">
        <header>
          <div><h2>{{ course.name }}</h2><span>{{ course.tees.length }} tee{{ course.tees.length === 1 ? '' : 's' }}</span></div>
          <div class="button-cluster">
            <button class="icon-button" title="Rename course" @click="openEditCourse(course)"><Pencil :size="17" /></button>
            <button class="button secondary compact" @click="openAdd(course.name)"><Plus :size="16" /> Tee</button>
          </div>
        </header>
        <div class="tee-table">
          <div v-for="tee in course.tees" :key="tee.id" class="tee-entry">
            <button class="tee-summary" @click="expanded = expanded === tee.id ? null : tee.id">
              <strong>{{ tee.name }}</strong>
              <span>Par {{ tee.totalPar }}</span><span>CR {{ tee.rating.toFixed(1) }}</span><span>Slope {{ tee.slope }}</span>
              <component :is="expanded === tee.id ? ChevronUp : ChevronDown" :size="18" />
            </button>
            <div v-if="expanded === tee.id" class="tee-detail">
              <div class="hole-reference">
                <div><span>Hole</span><b v-for="hole in 18" :key="hole">{{ hole }}</b></div>
                <div><span>Par</span><b v-for="(par, i) in tee.par" :key="i">{{ par }}</b></div>
                <div><span>SI</span><b v-for="(si, i) in tee.strokeIndex" :key="i">{{ si }}</b></div>
              </div>
              <div class="button-cluster tee-actions">
                <button class="button secondary compact" @click="openEditTee(course, tee)"><Pencil :size="16" /> Edit tee</button>
                <button class="button danger-text compact" @click="removeTee(tee.id, `${course.name} ${tee.name}`)"><Trash2 :size="16" /> Delete tee</button>
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>

    <AppModal v-if="showEditor" :title="editingTeeId === null ? 'Add course tee' : 'Edit tee'" wide @close="showEditor = false">
      <form class="form-stack" @submit.prevent="save">
        <div class="field-grid four">
          <label>Course name<input v-model.trim="form.name" :disabled="editingTeeId !== null" required /></label>
          <label>Tee name<input v-model.trim="form.tee" required /></label>
          <label>Course Rating<input v-model.number="form.rating" type="number" min="40" max="90" step="0.1" required /></label>
          <label>Slope Rating<input v-model.number="form.slope" type="number" min="55" max="155" required /></label>
        </div>
        <div class="course-editor">
          <div class="course-editor-header"><span>Hole</span><span>Par</span><span>Stroke index</span></div>
          <div v-for="hole in 18" :key="hole" class="course-editor-row">
            <strong>{{ hole }}</strong>
            <input v-model.number="form.par[hole - 1]" :aria-label="`Hole ${hole} par`" type="number" min="3" max="6" required />
            <input v-model.number="form.strokeIndex[hole - 1]" :aria-label="`Hole ${hole} stroke index`" type="number" min="1" max="18" required />
          </div>
        </div>
        <p v-if="!validStrokeIndexes" class="field-error">Stroke indexes must use each number from 1 to 18 once.</p>
        <div class="form-actions"><button type="button" class="button ghost" @click="showEditor = false">Cancel</button><button class="button" :disabled="saving || !validStrokeIndexes">{{ editingTeeId === null ? 'Save tee' : 'Save changes' }}</button></div>
      </form>
    </AppModal>

    <AppModal v-if="editingCourse" title="Rename course" @close="editingCourse = null">
      <form class="form-stack" @submit.prevent="saveCourse">
        <label>Course name<input v-model.trim="courseName" required autofocus /></label>
        <div class="form-actions"><button type="button" class="button ghost" @click="editingCourse = null">Cancel</button><button class="button" :disabled="saving">Save changes</button></div>
      </form>
    </AppModal>
  </div>
</template>

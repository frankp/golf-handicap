<script setup lang="ts">
import { ref } from 'vue'
import { KeyRound, LogIn } from '@lucide/vue'
import { useRoute, useRouter } from 'vue-router'
import { authState, login } from '@/auth'

const route = useRoute()
const router = useRouter()
const password = ref('')
const submitting = ref(false)
const error = ref('')

async function submit() {
  submitting.value = true
  error.value = ''
  try {
    await login(password.value)
    const redirect = typeof route.query.redirect === 'string' && route.query.redirect.startsWith('/') && !route.query.redirect.startsWith('//')
      ? route.query.redirect
      : '/'
    await router.replace(redirect)
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="page login-page">
    <section class="login-panel">
      <header><KeyRound :size="24" /><div><p class="eyebrow">Administration</p><h1>Sign in</h1></div></header>
      <p v-if="!authState.enabled" class="alert">Administrator access has not been configured.</p>
      <p v-if="error" class="alert error">{{ error }}</p>
      <form class="form-stack" @submit.prevent="submit">
        <label>Admin password<input v-model="password" type="password" required autofocus autocomplete="current-password" /></label>
        <button class="button full-width" :disabled="submitting || !authState.enabled">
          <LogIn :size="18" /> {{ submitting ? 'Signing in...' : 'Sign in' }}
        </button>
      </form>
    </section>
  </div>
</template>

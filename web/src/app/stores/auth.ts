import { defineStore } from 'pinia'
import { ref } from 'vue'

import { ApiError, fetchJSON } from '@/shared/composables/usePasskey'

export interface SessionUser {
  id: string
  username: string
}

export const useAuthStore = defineStore('auth', () => {
  // The server is the only source of truth: the session cookie is HttpOnly, so ask it.
  const user = ref<SessionUser | null>(null)
  let loading: Promise<SessionUser | null> | null = null

  // Memoized so navigations don't refetch; force after signing in.
  // Fails closed: any error means signed out. Only a 401 is cached, anything else is retried next time.
  function load(force = false) {
    if (force || !loading) {
      loading = fetchJSON<SessionUser>('/auth/session').then(
        (u) => (user.value = u),
        (e) => {
          if (!(e instanceof ApiError && e.status === 401)) loading = null
          return (user.value = null)
        },
      )
    }
    return loading
  }

  async function logout() {
    try {
      await fetchJSON('/auth/logout', { method: 'POST' })
    } finally {
      // If the request failed the next navigation asks the server again.
      user.value = null
      loading = null
    }
  }

  return { user, load, logout }
})

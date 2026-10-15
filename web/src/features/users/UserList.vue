<script setup lang="ts">
import { ref } from 'vue'

import { useAuthStore } from '@/app/stores/auth'
import { fetchJSON } from '@/shared/composables/usePasskey'
import { Badge } from '@/shared/ui/badge'
import { Button } from '@/shared/ui/button'
import {
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemGroup,
  ItemTitle,
} from '@/shared/ui/item'

interface User {
  id: string
  username: string
  disabled: boolean // awaiting approval or disabled
  createdAt: string
}

const auth = useAuthStore()
const users = ref<User[]>([])
const error = ref<string | null>(null)

// Runs the action, then reloads the list from the server.
async function refresh(action?: () => Promise<unknown>) {
  error.value = null
  try {
    await action?.()
    users.value = await fetchJSON<User[]>('/users')
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Request failed'
  }
}

const setStatus = (user: User, status: 'enable' | 'disable') =>
  refresh(() => fetchJSON(`/users/${user.id}/${status}`, { method: 'POST' }))

refresh()
</script>

<template>
  <div class="flex flex-col gap-4 p-4">
    <p v-if="error" class="text-destructive text-sm" role="alert">{{ error }}</p>
    <ItemGroup class="gap-2">
      <Item v-for="user in users" :key="user.id" variant="outline">
        <ItemContent>
          <ItemTitle>
            {{ user.username }}
            <Badge :variant="user.disabled ? 'secondary' : 'default'">
              {{ user.disabled ? 'Pending' : 'Active' }}
            </Badge>
          </ItemTitle>
          <ItemDescription
            >Joined {{ new Date(user.createdAt).toLocaleDateString() }}</ItemDescription
          >
        </ItemContent>
        <ItemActions>
          <Button v-if="user.disabled" size="sm" @click="setStatus(user, 'enable')">Approve</Button>
          <Button
            v-else-if="user.id !== auth.user?.id"
            size="sm"
            variant="outline"
            @click="setStatus(user, 'disable')"
          >
            Disable
          </Button>
        </ItemActions>
      </Item>
    </ItemGroup>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

import { type Role, useAuthStore } from '@/app/stores/auth'
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
  role: Role
  disabled: boolean // awaiting approval or disabled
  createdAt: string
}

interface UsersPage {
  users: User[]
  nextCursor?: string // absent on the last page
  previousCursor?: string // absent on the first page
}

const auth = useAuthStore()
const users = ref<User[]>([])
const error = ref<string | null>(null)
// Opaque cursors from the server; no cursor is the first page.
const cursor = ref<string>()
const nextCursor = ref<string>()
const previousCursor = ref<string>()

// Runs the action, then reloads the current page from the server.
async function refresh(action?: () => Promise<unknown>) {
  error.value = null
  try {
    await action?.()
    const page = await fetchJSON<UsersPage>(
      cursor.value ? `/users?cursor=${encodeURIComponent(cursor.value)}` : '/users',
    )
    // The page emptied out from under the cursor: start over instead of showing a blank page.
    if (!page.users.length && cursor.value) {
      cursor.value = undefined
      return refresh()
    }
    users.value = page.users
    nextCursor.value = page.nextCursor
    previousCursor.value = page.previousCursor
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Request failed'
  }
}

const setStatus = (user: User, status: 'enable' | 'disable') =>
  refresh(() => fetchJSON(`/users/${user.id}/${status}`, { method: 'POST' }))

const setRole = (user: User, role: Exclude<Role, 'owner'>) =>
  refresh(() => fetchJSON(`/users/${user.id}/role`, { method: 'POST', body: { role } }))

function goTo(target?: string) {
  cursor.value = target
  refresh()
}

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
            <Badge variant="outline" class="capitalize">{{ user.role }}</Badge>
          </ItemTitle>
          <ItemDescription
            >Joined {{ new Date(user.createdAt).toLocaleDateString() }}</ItemDescription
          >
        </ItemContent>
        <!-- Only the owner gets this list; they cannot manage themselves. -->
        <ItemActions v-if="user.id !== auth.user?.id">
          <Button v-if="user.disabled" size="sm" @click="setStatus(user, 'enable')">Approve</Button>
          <template v-else>
            <Button
              v-if="user.role === 'user'"
              size="sm"
              variant="outline"
              @click="setRole(user, 'admin')"
            >
              Make admin
            </Button>
            <Button
              v-else-if="user.role === 'admin'"
              size="sm"
              variant="outline"
              @click="setRole(user, 'user')"
            >
              Make user
            </Button>
            <Button size="sm" variant="outline" @click="setStatus(user, 'disable')">Disable</Button>
          </template>
        </ItemActions>
      </Item>
    </ItemGroup>
    <div v-if="previousCursor || nextCursor" class="flex justify-end gap-2">
      <Button size="sm" variant="outline" :disabled="!previousCursor" @click="goTo(previousCursor)">
        Previous
      </Button>
      <Button size="sm" variant="outline" :disabled="!nextCursor" @click="goTo(nextCursor)">
        Next
      </Button>
    </div>
  </div>
</template>

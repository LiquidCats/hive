<script setup lang="ts">
import { computed, ref } from 'vue'
import { GitBranch, Plus } from '@lucide/vue'

import { useAuthStore } from '@/app/stores/auth'
import { fetchJSON } from '@/shared/composables/usePasskey'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/shared/ui/alert-dialog'
import { Badge } from '@/shared/ui/badge'
import { Button } from '@/shared/ui/button'
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/shared/ui/empty'
import {
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemGroup,
  ItemTitle,
} from '@/shared/ui/item'

import RepositoryDrawer from './RepositoryDrawer.vue'
import { isSSH, type Repository } from './repository'

interface RepositoriesPage {
  repositories: Repository[]
  nextCursor?: string // absent on the last page
  previousCursor?: string // absent on the first page
}

const auth = useAuthStore()
const repositories = ref<Repository[]>([])
const error = ref<string | null>(null)
// Opaque cursors from the server; no cursor is the first page.
const cursor = ref<string>()
const nextCursor = ref<string>()
const previousCursor = ref<string>()
// Avoids flashing the empty state before the first response.
const loaded = ref(false)

// Writes need admin; the server enforces it, this only hides the buttons.
const canWrite = computed(() => auth.user?.role !== 'user')
const isEmpty = computed(() => loaded.value && !repositories.value.length && !cursor.value)

const drawerOpen = ref(false)
const editing = ref<Repository>()
const deleting = ref<Repository>()

// Runs the action, then reloads the current page from the server.
async function refresh(action?: () => Promise<unknown>) {
  error.value = null
  try {
    await action?.()
    const page = await fetchJSON<RepositoriesPage>(
      cursor.value ? `/repositories?cursor=${encodeURIComponent(cursor.value)}` : '/repositories',
    )
    // Deleted the last row of a later page: start over instead of showing a blank page.
    if (!page.repositories.length && cursor.value) {
      cursor.value = undefined
      return refresh()
    }
    repositories.value = page.repositories
    nextCursor.value = page.nextCursor
    previousCursor.value = page.previousCursor
    loaded.value = true
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Request failed'
  }
}

function openDrawer(repo?: Repository) {
  editing.value = repo
  drawerOpen.value = true
}

async function confirmDelete() {
  const repo = deleting.value
  if (!repo) return
  await refresh(() => fetchJSON(`/repositories/${repo.id}`, { method: 'DELETE' }))
}

function goTo(target?: string) {
  cursor.value = target
  refresh()
}

refresh()
</script>

<template>
  <div class="flex flex-col gap-4 p-4">
    <p v-if="error" class="text-destructive text-sm" role="alert">{{ error }}</p>

    <Empty v-if="isEmpty" class="border border-dashed">
      <EmptyHeader>
        <EmptyMedia variant="icon"><GitBranch /></EmptyMedia>
        <EmptyTitle>No repositories yet</EmptyTitle>
        <EmptyDescription>Add a Git repository to deploy apps from.</EmptyDescription>
      </EmptyHeader>
      <EmptyContent v-if="canWrite">
        <Button size="sm" @click="openDrawer()"><Plus />Add repository</Button>
      </EmptyContent>
    </Empty>

    <template v-else>
      <div v-if="canWrite" class="flex justify-end">
        <Button size="sm" @click="openDrawer()"><Plus />Add repository</Button>
      </div>
      <ItemGroup class="gap-2">
        <Item v-for="repo in repositories" :key="repo.id" variant="outline">
          <ItemContent>
            <ItemTitle>
              {{ repo.name || repo.url }}
              <Badge variant="outline">{{ isSSH(repo.url) ? 'SSH' : 'HTTPS' }}</Badge>
              <Badge v-if="repo.hasPassword" variant="secondary">Password</Badge>
              <Badge v-if="repo.hasSshPrivateKey" variant="secondary">SSH key</Badge>
              <Badge v-if="repo.insecure" variant="destructive">Insecure</Badge>
            </ItemTitle>
            <ItemDescription v-if="repo.name">{{ repo.url }}</ItemDescription>
          </ItemContent>
          <ItemActions v-if="canWrite">
            <Button size="sm" variant="outline" @click="openDrawer(repo)">Edit</Button>
            <Button size="sm" variant="outline" @click="deleting = repo">Delete</Button>
          </ItemActions>
        </Item>
      </ItemGroup>
      <div v-if="previousCursor || nextCursor" class="flex justify-end gap-2">
        <Button
          size="sm"
          variant="outline"
          :disabled="!previousCursor"
          @click="goTo(previousCursor)"
        >
          Previous
        </Button>
        <Button size="sm" variant="outline" :disabled="!nextCursor" @click="goTo(nextCursor)">
          Next
        </Button>
      </div>
    </template>

    <RepositoryDrawer v-model:open="drawerOpen" :repo="editing" @saved="refresh()" />

    <AlertDialog :open="!!deleting" @update:open="(o) => !o && (deleting = undefined)">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete repository?</AlertDialogTitle>
          <AlertDialogDescription>
            “{{ deleting?.name || deleting?.url }}” will be removed from Hive. This cannot be
            undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            class="bg-destructive text-white hover:bg-destructive/90"
            @click="confirmDelete"
          >
            Delete
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>

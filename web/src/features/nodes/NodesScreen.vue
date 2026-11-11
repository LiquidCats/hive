<script setup lang="ts">
import { computed, ref } from 'vue'
import { Crown, Ellipsis, Hexagon, Server } from '@lucide/vue'
import { useIntervalFn } from '@vueuse/core'

import { useAuthStore } from '@/app/stores/auth'
import { fetchJSON } from '@/shared/composables/usePasskey'
import AddNodeDrawer from './components/AddNodeDrawer.vue'
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
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/shared/ui/dropdown-menu'
import { Field, FieldLabel } from '@/shared/ui/field'
import {
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemGroup,
  ItemMedia,
  ItemTitle,
} from '@/shared/ui/item'
import { Switch } from '@/shared/ui/switch'

type Availability = 'active' | 'pause' | 'drain'

interface SwarmNode {
  id: string
  hostname: string
  role: 'manager' | 'worker'
  leader: boolean
  state: string // ready, down, disconnected or unknown
  availability: Availability
  addr: string
  engine: string
  os: string
  arch: string
  nanoCpus: number
  memoryBytes: number
}

const auth = useAuthStore()
const nodes = ref<SwarmNode[]>([])
const error = ref<string | null>(null)
// Avoids flashing the empty state before the first response.
const loaded = ref(false)

// Writes need admin; the server enforces it, this only hides the actions.
const canWrite = computed(() => auth.user?.role !== 'user')

const removing = ref<SwarmNode>()
const force = ref(false)

// Runs the action, then reloads the nodes from Docker.
async function refresh(action?: () => Promise<unknown>) {
  try {
    await action?.()
    nodes.value = (await fetchJSON<{ nodes: SwarmNode[] }>('/nodes')).nodes
    error.value = null
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Request failed'
  } finally {
    loaded.value = true
  }
}

const setAvailability = (node: SwarmNode, availability: Availability) =>
  refresh(() =>
    fetchJSON(`/nodes/${node.id}/availability`, { method: 'PUT', body: { availability } }),
  )

const toggleRole = (node: SwarmNode) =>
  refresh(() =>
    fetchJSON(`/nodes/${node.id}/role`, {
      method: 'PUT',
      body: { role: node.role === 'manager' ? 'worker' : 'manager' },
    }),
  )

function askRemove(node: SwarmNode) {
  force.value = false
  removing.value = node
}

async function confirmRemove() {
  const node = removing.value
  if (!node) return
  await refresh(() =>
    fetchJSON(`/nodes/${node.id}${force.value ? '?force=true' : ''}`, { method: 'DELETE' }),
  )
}

const dot = (state: string) =>
  ({ ready: 'bg-green-500', down: 'bg-red-500' })[state] ?? 'bg-amber-500'

const gb = (bytes: number) => Math.round(bytes / 2 ** 30)

const summary = computed(() => ({
  total: nodes.value.length,
  ready: nodes.value.filter((n) => n.state === 'ready').length,
  managers: nodes.value.filter((n) => n.role === 'manager').length,
  down: nodes.value.filter((n) => n.state !== 'ready').length,
}))

refresh()
// ponytail: polling, Docker has no push for node state here; switch to the events API if 10s lag matters.
useIntervalFn(() => refresh(), 10_000)
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between gap-4">
      <p class="text-muted-foreground text-sm">
        {{ summary.total }} nodes · {{ summary.ready }} ready · {{ summary.managers }} managers
        <template v-if="summary.down">
          · <span class="text-destructive">{{ summary.down }} down</span></template
        >
      </p>
      <AddNodeDrawer v-if="canWrite" />
    </div>

    <p v-if="error" class="text-destructive text-sm" role="alert">{{ error }}</p>

    <ItemGroup v-if="nodes.length" class="gap-2">
      <Item
        v-for="node in nodes"
        :key="node.id"
        variant="outline"
        :class="node.state !== 'ready' && 'opacity-70'"
      >
        <ItemMedia variant="icon" class="relative size-10">
          <Hexagon v-if="node.role === 'manager'" />
          <Server v-else />
          <span
            :class="[
              'absolute -top-1 -left-1 size-2.5 rounded-full ring-2 ring-background',
              dot(node.state),
            ]"
            :title="node.state"
          />
        </ItemMedia>

        <ItemContent>
          <ItemTitle>
            {{ node.hostname }}
            <Badge v-if="node.leader" variant="default"><Crown />Leader</Badge>
            <Badge v-else variant="secondary" class="capitalize">{{ node.role }}</Badge>
            <Badge v-if="node.availability !== 'active'" variant="outline" class="capitalize">
              {{ node.availability }}
            </Badge>
            <Badge v-if="node.state !== 'ready'" variant="destructive" class="capitalize">{{
              node.state
            }}</Badge>
          </ItemTitle>
          <ItemDescription class="font-mono text-xs">
            {{ node.addr }} · {{ node.os }}/{{ node.arch }} · {{ node.nanoCpus / 1e9 }} CPU ·
            {{ gb(node.memoryBytes) }} GB · Docker {{ node.engine }}
          </ItemDescription>
        </ItemContent>

        <ItemActions v-if="canWrite">
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="outline" size="icon-sm" aria-label="Node actions">
                <Ellipsis />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuLabel>Availability</DropdownMenuLabel>
              <DropdownMenuRadioGroup
                :model-value="node.availability"
                @update:model-value="(a) => setAvailability(node, a as Availability)"
              >
                <DropdownMenuRadioItem value="active">Active</DropdownMenuRadioItem>
                <DropdownMenuRadioItem value="pause">Pause</DropdownMenuRadioItem>
                <DropdownMenuRadioItem value="drain">Drain</DropdownMenuRadioItem>
              </DropdownMenuRadioGroup>
              <DropdownMenuSeparator />
              <DropdownMenuItem @select="toggleRole(node)">
                {{ node.role === 'manager' ? 'Demote to worker' : 'Promote to manager' }}
              </DropdownMenuItem>
              <DropdownMenuItem variant="destructive" @select="askRemove(node)">
                Remove
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </ItemActions>
      </Item>
    </ItemGroup>

    <p v-else-if="loaded && !error" class="text-muted-foreground py-12 text-center text-sm">
      No nodes in this swarm yet. Use “Add node” to connect one.
    </p>

    <AlertDialog :open="!!removing" @update:open="(o) => !o && (removing = undefined)">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Remove node?</AlertDialogTitle>
          <AlertDialogDescription>
            “{{ removing?.hostname }}” leaves the swarm; run <code>docker swarm leave</code> on it
            first, or force removal. Managers must be demoted before removal.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <Field orientation="horizontal">
          <Switch id="node-force" v-model="force" />
          <FieldLabel for="node-force">Force (node is not down)</FieldLabel>
        </Field>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            class="bg-destructive text-white hover:bg-destructive/90"
            @click="confirmRemove"
          >
            Remove
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>

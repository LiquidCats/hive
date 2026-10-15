<script setup lang="ts">
import { computed, ref } from 'vue'
import { Crown, Hexagon, Pause, Play, Server } from '@lucide/vue'

import AddNodeDrawer from './components/AddNodeDrawer.vue'
import { Badge } from '@/shared/ui/badge'
import { Button } from '@/shared/ui/button'
import {
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemGroup,
  ItemMedia,
  ItemTitle,
} from '@/shared/ui/item'

type State = 'ready' | 'down' | 'unknown'
type Availability = 'active' | 'pause' | 'drain'

interface SwarmNode {
  id: string
  hostname: string
  role: 'manager' | 'worker'
  leader?: boolean
  state: State
  availability: Availability
  addr: string
  engine: string
  os: string
  cpus: number
  memGb: number
}

// ponytail: static fixtures, BFF has no nodes endpoint yet; replace with fetch from `docker node ls` API.
const nodes = ref<SwarmNode[]>([
  {
    id: '1',
    hostname: 'hive-mgr-01',
    role: 'manager',
    leader: true,
    state: 'ready',
    availability: 'active',
    addr: '10.0.0.11',
    engine: '27.3.1',
    os: 'linux/amd64',
    cpus: 4,
    memGb: 8,
  },
  {
    id: '2',
    hostname: 'hive-wrk-01',
    role: 'worker',
    state: 'ready',
    availability: 'active',
    addr: '10.0.0.21',
    engine: '27.3.1',
    os: 'linux/amd64',
    cpus: 8,
    memGb: 32,
  },
  {
    id: '3',
    hostname: 'hive-wrk-02',
    role: 'worker',
    state: 'ready',
    availability: 'drain',
    addr: '10.0.0.22',
    engine: '27.3.1',
    os: 'linux/arm64',
    cpus: 8,
    memGb: 16,
  },
  {
    id: '4',
    hostname: 'hive-wrk-03',
    role: 'worker',
    state: 'down',
    availability: 'active',
    addr: '10.0.0.23',
    engine: '26.1.4',
    os: 'linux/amd64',
    cpus: 4,
    memGb: 16,
  },
])

const dot: Record<State, string> = {
  ready: 'bg-green-500',
  down: 'bg-red-500',
  unknown: 'bg-amber-500',
}

const managerAddr = computed(
  () =>
    (nodes.value.find((n) => n.leader) ?? nodes.value.find((n) => n.role === 'manager'))?.addr ??
    '<manager-ip>',
)

const summary = computed(() => ({
  total: nodes.value.length,
  ready: nodes.value.filter((n) => n.state === 'ready').length,
  managers: nodes.value.filter((n) => n.role === 'manager').length,
  down: nodes.value.filter((n) => n.state !== 'ready').length,
}))

function toggleAvailability(node: SwarmNode) {
  node.availability = node.availability === 'active' ? 'drain' : 'active'
}
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
      <AddNodeDrawer :manager-addr="managerAddr" />
    </div>

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
              dot[node.state],
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
            {{ node.addr }} · {{ node.os }} · {{ node.cpus }} CPU · {{ node.memGb }} GB · Docker
            {{ node.engine }}
          </ItemDescription>
        </ItemContent>

        <ItemActions>
          <Button
            variant="outline"
            size="sm"
            :disabled="node.state !== 'ready'"
            @click="toggleAvailability(node)"
          >
            <template v-if="node.availability === 'active'"><Pause />Drain</template>
            <template v-else><Play />Activate</template>
          </Button>
        </ItemActions>
      </Item>
    </ItemGroup>

    <p v-else class="text-muted-foreground py-12 text-center text-sm">
      No nodes in this swarm yet. Use “Add node” to connect one.
    </p>
  </div>
</template>

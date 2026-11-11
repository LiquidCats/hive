<script setup lang="ts">
import { computed, ref } from 'vue'
import { Check, Copy, Plus } from '@lucide/vue'
import { useClipboard } from '@vueuse/core'

import { fetchJSON } from '@/shared/composables/usePasskey'
import { Button } from '@/shared/ui/button'
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
  DrawerTrigger,
} from '@/shared/ui/drawer'
import { Tabs, TabsList, TabsTrigger } from '@/shared/ui/tabs'

interface JoinInfo {
  workerToken: string
  managerToken: string
  managerAddr: string // leader host:port
}

type Role = 'worker' | 'manager'
const role = ref<Role>('worker')
const join = ref<JoinInfo>()
const error = ref<string | null>(null)

// Fetched on every open: tokens are admin-only and may have been rotated.
async function load(open: boolean) {
  if (!open) return
  error.value = null
  try {
    join.value = await fetchJSON<JoinInfo>('/nodes/join')
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Request failed'
  }
}

const command = computed(() =>
  join.value
    ? `docker swarm join --token ${role.value === 'manager' ? join.value.managerToken : join.value.workerToken} ${join.value.managerAddr}`
    : '',
)
const managerHost = computed(() => join.value?.managerAddr.replace(/:\d+$/, '') ?? 'the manager')
const { copy, copied } = useClipboard({ source: command })
</script>

<template>
  <Drawer @update:open="load">
    <DrawerTrigger as-child>
      <Button size="sm"><Plus />Add node</Button>
    </DrawerTrigger>
    <DrawerContent class="max-w-xl mx-auto">
      <div class="mx-auto flex w-full max-w-md flex-col gap-6 p-6 pb-10">
        <DrawerHeader class="p-0">
          <DrawerTitle>Connect a node</DrawerTitle>
          <DrawerDescription>
            // Run the command below on the machine you want to join to this swarm.
          </DrawerDescription>
        </DrawerHeader>

        <Tabs v-model="role">
          <TabsList class="w-full">
            <TabsTrigger value="worker">Worker</TabsTrigger>
            <TabsTrigger value="manager">Manager</TabsTrigger>
          </TabsList>
        </Tabs>

        <p v-if="error" class="text-destructive text-sm" role="alert">{{ error }}</p>
        <div v-else class="flex flex-col gap-2">
          <pre
            class="bg-muted overflow-x-auto rounded-md border p-3 font-mono text-xs break-all whitespace-pre-wrap"
            >{{ command || 'Loading…' }}</pre>
          <Button variant="outline" size="sm" class="self-end" :disabled="!command" @click="copy()">
            <Check v-if="copied" /><Copy v-else />{{ copied ? 'Copied' : 'Copy command' }}
          </Button>
        </div>

        <ul class="text-muted-foreground list-disc space-y-1 pl-5 text-sm">
          <li>Docker must be installed and running on the new machine.</li>
          <li>
            Port 2377/tcp must be reachable on {{ managerHost }}; 7946 and 4789 between nodes.
          </li>
          <li v-if="role === 'manager'">
            Managers join the Raft quorum: keep an odd number (3 or 5).
          </li>
          <li>The node appears in the list once it joins.</li>
        </ul>
      </div>
    </DrawerContent>
  </Drawer>
</template>

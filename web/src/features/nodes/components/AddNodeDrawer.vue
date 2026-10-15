<script setup lang="ts">
import { computed, ref } from 'vue'
import { Check, Copy, Plus } from '@lucide/vue'
import { useClipboard } from '@vueuse/core'

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

const props = defineProps<{
  /** advertise address of a manager, host only */
  managerAddr: string
}>()

type Role = 'worker' | 'manager'
const role = ref<Role>('worker')

// ponytail: placeholder tokens, BFF has no endpoint yet; fetch `docker swarm join-token` per role.
const tokens: Record<Role, string> = {
  worker: 'SWMTKN-1-<worker-token>',
  manager: 'SWMTKN-1-<manager-token>',
}

const command = computed(
  () => `docker swarm join --token ${tokens[role.value]} ${props.managerAddr}:2377`,
)
const { copy, copied } = useClipboard({ source: command })
</script>

<template>
  <Drawer>
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

        <div class="flex flex-col gap-2">
          <pre
            class="bg-muted overflow-x-auto rounded-md border p-3 font-mono text-xs break-all whitespace-pre-wrap"
          >{{ command }}</pre>
          <Button variant="outline" size="sm" class="self-end" @click="copy()">
            <Check v-if="copied" /><Copy v-else />{{ copied ? 'Copied' : 'Copy command' }}
          </Button>
        </div>

        <ul class="text-muted-foreground list-disc space-y-1 pl-5 text-sm">
          <li>Docker must be installed and running on the new machine.</li>
          <li>Port 2377/tcp must be reachable on {{ managerAddr }}; 7946 and 4789 between nodes.</li>
          <li v-if="role === 'manager'">Managers join the Raft quorum: keep an odd number (3 or 5).</li>
          <li>The node appears in the list once it joins.</li>
        </ul>
      </div>
    </DrawerContent>
  </Drawer>
</template>

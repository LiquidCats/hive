<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { Hexagon } from '@lucide/vue'

import { RouteNames } from '@/app/router'
import ThemeToggle from '@/shared/components/ThemeToggle.vue'
import { Tabs, TabsList, TabsTrigger } from '@/shared/ui/tabs'

const route = useRoute()
const tab = computed(() => (route.name === RouteNames.AuthSignUp ? 'sign-up' : 'sign-in'))
</script>

<template>
  <div class="bg-muted relative flex min-h-svh flex-col items-center justify-center gap-6 p-6 md:p-10">
    <ThemeToggle class="absolute top-4 right-4" />
    <main class="flex w-full max-w-sm flex-col gap-6">
      <div class="flex items-center justify-center gap-2 self-center text-lg font-semibold">
        <div class="bg-primary text-primary-foreground flex size-8 items-center justify-center rounded-md">
          <Hexagon class="size-5" />
        </div>
        Hive
      </div>

      <Tabs :model-value="tab">
        <TabsList class="w-full">
          <TabsTrigger value="sign-in" as-child>
            <RouterLink :to="{ name: RouteNames.AuthSignIn }">Sign In</RouterLink>
          </TabsTrigger>
          <TabsTrigger value="sign-up" as-child>
            <RouterLink :to="{ name: RouteNames.AuthSignUp }">Sign Up</RouterLink>
          </TabsTrigger>
        </TabsList>
      </Tabs>

      <RouterView />

      <p class="text-muted-foreground px-6 text-center text-xs text-balance">
        Passkeys use your device's biometrics or PIN. No password to remember or leak.
      </p>
    </main>
  </div>
</template>

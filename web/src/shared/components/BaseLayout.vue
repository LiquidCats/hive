<script setup lang="ts">
import { RouteNames } from '@/app/router'
import AppSidebar from '@/shared/components/AppSidebar.vue'
import SiteHeader from '@/shared/components/SiteHeader.vue'
import { SidebarInset, SidebarProvider } from '@/shared/ui/sidebar'
import { useRoute, useRouter } from 'vue-router'

const router = useRouter()
const route = useRoute()
if (route.name == RouteNames.Hive) {
  router.replace({ name: RouteNames.HiveApps })
}
</script>

<template>
  <SidebarProvider
    :style="{
      '--sidebar-width': 'calc(var(--spacing) * 72)',
      '--header-height': 'calc(var(--spacing) * 12)',
    }"
  >
    <AppSidebar />
    <SidebarInset>
      <SiteHeader />
      <main class="flex flex-1 flex-col gap-4 p-4 pt-2">
        <RouterView />
      </main>
    </SidebarInset>
  </SidebarProvider>
</template>

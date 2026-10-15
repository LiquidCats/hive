<script setup lang="ts">
import type { SidebarProps } from '@/shared/ui/sidebar'
import {
  LayoutDashboard,
  HelpCircle,
  Settings,
  Hexagon,
  KeyRound,
  Workflow,
  UsersRound,
} from '@lucide/vue'

import { computed } from 'vue'

import { RouteNames } from '@/app/router'
import { useAuthStore } from '@/app/stores/auth'
import NavMain from '@/shared/components/NavMain.vue'
import NavSecondary from '@/shared/components/NavSecondary.vue'
import NavUser from '@/shared/components/NavUser.vue'

import {
  Sidebar,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
} from '@/shared/ui/sidebar'

const props = withDefaults(defineProps<SidebarProps>(), {
  variant: 'inset',
})

const auth = useAuthStore()

const data = {
  navMain: [
    {
      title: 'Applications',
      url: '/apps',
      route: RouteNames.HiveApps,
      icon: LayoutDashboard,
    },
    {
      title: 'Nodes',
      url: '/nodes',
      route: RouteNames.HiveNodes,
      icon: Workflow,
    },
    {
      title: 'Secrets',
      url: '/secrets',
      route: RouteNames.HiveSecrets,
      icon: KeyRound,
    },
    {
      title: 'Users',
      url: '/users',
      route: RouteNames.HiveUsers,
      icon: UsersRound,
    },
  ],
  navSecondary: [
    {
      title: 'Settings',
      url: '/settings',
      route: RouteNames.HiveSettings,
      icon: Settings,
    },
    {
      title: 'Get Help',
      url: '#',
      icon: HelpCircle,
    },
  ],
}

// User management is the owner's; the server refuses everyone else anyway.
const navMain = computed(() =>
  data.navMain.filter((item) => item.route !== RouteNames.HiveUsers || auth.user?.role === 'owner'),
)
</script>

<template>
  <Sidebar v-bind="props" collapsible="offcanvas">
    <SidebarHeader>
      <SidebarMenu>
        <SidebarMenuItem>
          <SidebarMenuButton as-child class="data-[slot=sidebar-menu-button]:p-1.5!">
            <RouterLink is="a" :to="{ name: RouteNames.HiveApps }">
              <div
                class="bg-primary text-primary-foreground flex size-8 items-center justify-center rounded-md"
              >
                <Hexagon class="size-5" />
              </div>
              <span class="text-base font-semibold">Hive</span>
            </RouterLink>
          </SidebarMenuButton>
        </SidebarMenuItem>
      </SidebarMenu>
    </SidebarHeader>
    <SidebarContent>
      <NavMain :items="navMain" />
      <NavSecondary :items="data.navSecondary" class="mt-auto" />
    </SidebarContent>
    <SidebarFooter>
      <NavUser />
    </SidebarFooter>
  </Sidebar>
</template>

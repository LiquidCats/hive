<script setup lang="ts">
import type { Component } from 'vue'

import { useRoute } from 'vue-router'

import {
  SidebarGroup,
  SidebarGroupContent,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from '@/shared/ui/sidebar'

interface NavItem {
  title: string
  url: string
  route?: string
  icon?: Component
}

const route = useRoute()

defineProps<{
  items: NavItem[]
}>()

function isActive(item: NavItem): boolean {
  return route.name == item.route || route.path == item.url
}
</script>

<template>
  <SidebarGroup>
    <SidebarGroupContent class="flex flex-col gap-2">
      <SidebarMenu>
        <SidebarMenuItem v-for="item in items" :key="item.title">
          <SidebarMenuButton as-child :tooltip="item.title" :is-active="isActive(item)">
            <RouterLink is="a" :to="item.route ? { name: item.route } : item.url">
              <component :is="item.icon" v-if="item.icon" />
              <span>{{ item.title }}</span>
            </RouterLink>
          </SidebarMenuButton>
        </SidebarMenuItem>
      </SidebarMenu>
    </SidebarGroupContent>
  </SidebarGroup>
</template>

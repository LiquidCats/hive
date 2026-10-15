<script setup lang="ts">
import type { Component } from 'vue'

import {
  SidebarGroup,
  SidebarGroupContent,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from '@/shared/ui/sidebar'

import { useRoute } from 'vue-router'

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
  return item.route ? route.name === item.route : false
}
</script>

<template>
  <SidebarGroup>
    <SidebarGroupContent>
      <SidebarMenu>
        <SidebarMenuItem v-for="item in items" :key="item.title">
          <SidebarMenuButton as-child :is-active="isActive(item)">
            <RouterLink is="a" :to="item.route ? { name: item.route } : item.url">
              <component :is="item.icon" v-if="item.icon" />
              {{ item.title }}
            </RouterLink>
          </SidebarMenuButton>
        </SidebarMenuItem>
      </SidebarMenu>
    </SidebarGroupContent>
  </SidebarGroup>
</template>

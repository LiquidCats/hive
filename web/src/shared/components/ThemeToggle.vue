<script setup lang="ts">
import { Moon, Sun } from '@lucide/vue'
import { useDark, useToggle } from '@vueuse/core'

import { Button } from '@/shared/ui/button'

const isDark = useDark()
const toggleDark = useToggle(isDark)
</script>

<template>
  <Button variant="ghost" size="icon" aria-label="Toggle theme" @click="toggleDark()">
    <Sun v-if="isDark" />
    <Moon v-else />
  </Button>
</template>

<script setup lang="ts">
import { useDark } from '@vueuse/core'

// apply saved/system theme on every route, not only where a toggle is mounted
useDark()
</script>

<template>
  <RouterView />
</template>

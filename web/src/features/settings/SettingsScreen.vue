<script setup lang="ts">
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/shared/ui/tabs'

import RepositoryList from './components/RepositoryList.vue'
</script>
<template>
  <Tabs default-value="repositories">
    <TabsList>
      <TabsTrigger value="repositories"> Repositories </TabsTrigger>
    </TabsList>
    <TabsContent value="repositories">
      <RepositoryList />
    </TabsContent>
  </Tabs>
</template>

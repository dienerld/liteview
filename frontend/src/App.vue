<script setup lang="ts">
import { onMounted } from "vue"
import { Toaster } from "@/components/ui/sonner"
import AppSidebar from "@/components/AppSidebar.vue"
import EmptyState from "@/components/EmptyState.vue"
import { useViewer } from "@/stores/viewer"

const store = useViewer()
onMounted(() => store.init())
</script>

<template>
  <div class="bg-background text-foreground flex h-screen">
    <EmptyState v-if="!store.db" class="flex-1" />
    <template v-else>
      <AppSidebar />
      <main class="min-w-0 flex-1 overflow-hidden">
        <p v-if="!store.current" class="text-muted-foreground p-8">Escolha uma tabela na barra lateral.</p>
        <p v-else class="p-8">{{ store.current.label }}</p>
      </main>
    </template>
    <Toaster />
  </div>
</template>

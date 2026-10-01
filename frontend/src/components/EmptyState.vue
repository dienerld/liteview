<script setup lang="ts">
import { Button } from "@/components/ui/button"
import { useViewer } from "@/stores/viewer"
const store = useViewer()
</script>

<template>
  <div class="flex h-full flex-col items-center justify-center gap-6 p-8">
    <div class="text-center">
      <h1 class="text-2xl font-semibold">SQLite Viewer</h1>
      <p class="text-muted-foreground mt-1">Abra um arquivo de banco para começar.</p>
    </div>
    <Button @click="store.openDialog()">Abrir banco…</Button>
    <div v-if="store.recents.length" class="w-full max-w-md">
      <p class="text-muted-foreground mb-2 text-sm">Recentes</p>
      <ul class="space-y-1">
        <li v-for="p in store.recents" :key="p">
          <button class="hover:bg-accent w-full truncate rounded px-2 py-1 text-left text-sm" :title="p" @click="store.openPath(p)">
            {{ p }}
          </button>
        </li>
      </ul>
    </div>
  </div>
</template>

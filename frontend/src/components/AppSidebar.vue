<script setup lang="ts">
import { computed, ref } from "vue"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { ScrollArea } from "@/components/ui/scroll-area"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { useViewer } from "@/stores/viewer"

const store = useViewer()
const search = ref("")
const visible = computed(() =>
  store.tables.filter((t) => t.name.toLowerCase().includes(search.value.toLowerCase())),
)
</script>

<template>
  <aside class="bg-muted/30 flex h-full w-64 shrink-0 flex-col border-r">
    <div class="flex items-center gap-2 border-b p-3">
      <div class="min-w-0 flex-1">
        <p class="truncate text-sm font-medium" :title="store.db?.path">{{ store.db?.name ?? "Nenhum banco" }}</p>
        <p v-if="store.db?.readOnly" class="text-muted-foreground text-xs">somente leitura</p>
      </div>
      <DropdownMenu>
        <DropdownMenuTrigger as-child><Button variant="outline" size="sm">Abrir</Button></DropdownMenuTrigger>
        <DropdownMenuContent align="end" class="w-72">
          <DropdownMenuItem @click="store.openDialog()">Abrir arquivo…</DropdownMenuItem>
          <DropdownMenuItem v-for="p in store.recents" :key="p" class="truncate" @click="store.openPath(p)">{{ p }}</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
    <div class="p-2"><Input v-model="search" placeholder="Buscar tabela…" /></div>
    <ScrollArea class="min-h-0 flex-1">
      <ul class="space-y-0.5 p-2">
        <li v-for="t in visible" :key="t.name">
          <button
            class="hover:bg-accent flex w-full items-center gap-2 rounded px-2 py-1 text-left text-sm"
            :class="{ 'bg-accent': store.trail[0]?.table === t.name }"
            @click="store.openTable(t.name)"
          >
            <span class="text-muted-foreground w-4 text-center text-xs">{{ t.kind === "view" ? "◇" : "▦" }}</span>
            <span class="truncate">{{ t.name }}</span>
          </button>
        </li>
      </ul>
    </ScrollArea>
  </aside>
</template>

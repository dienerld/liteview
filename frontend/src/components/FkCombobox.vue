<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from "vue"
import { ChevronsUpDownIcon } from "@lucide/vue"
import { ListboxFilter } from "reka-ui"
import { toast } from "vue-sonner"
import { Button } from "@/components/ui/button"
import { Command, CommandGroup, CommandItem, CommandList } from "@/components/ui/command"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { api } from "@/lib/api"
import { toAppError } from "@/lib/errors"
import { optionLabel } from "@/lib/fkOptions"
import type { ForeignKey, Row, TableSchema } from "@/lib/types"

const props = defineProps<{
  fk: ForeignKey
  modelValue: string
  disabled?: boolean
  id?: string
  invalid?: boolean
  describedby?: string
}>()
const emit = defineEmits<{ "update:modelValue": [v: string] }>()

const keyCol = props.fk.to[0]
const open = ref(false)
const search = ref("")
const options = ref<Row[]>([])
const loading = ref(false)
const target = ref<TableSchema | null>(null)
let seq = 0
let timer: ReturnType<typeof setTimeout> | undefined

async function load() {
  const mine = ++seq
  loading.value = true
  try {
    if (!target.value) target.value = await api.getTable(props.fk.table)
  } catch {
    // label hints are optional; the key alone is still usable
  }
  try {
    const page = await api.queryRows({
      table: props.fk.table, page: 1, pageSize: 20, orderBy: keyCol, desc: false, filter: search.value, where: [],
    })
    if (mine === seq) options.value = page?.rows ?? []
  } catch (e) {
    if (mine === seq) toast.error(toAppError(e).message)
  } finally {
    if (mine === seq) loading.value = false
  }
}

function onSearch(v: string | number) {
  search.value = String(v)
  clearTimeout(timer)
  timer = setTimeout(load, 200)
}

watch(open, (o) => {
  clearTimeout(timer)
  if (!o) { seq++; loading.value = false; return }
  search.value = ""
  load()
})
onBeforeUnmount(() => { clearTimeout(timer); seq++ })

const keyOf = (r: Row) => String(r.values[keyCol])

function pick(r: Row) {
  emit("update:modelValue", keyOf(r))
  open.value = false
}
</script>

<template>
  <Popover v-model:open="open">
    <PopoverTrigger as-child>
      <Button :id="id" type="button" variant="outline" role="combobox" class="w-full justify-between font-normal"
        :disabled="disabled" :aria-expanded="open" :aria-invalid="invalid ? true : undefined"
        :aria-describedby="describedby">
        <span class="truncate" :class="{ 'text-muted-foreground': !modelValue }">{{ modelValue || `Escolher em ${fk.table}…` }}</span>
        <ChevronsUpDownIcon data-icon="inline-end" class="opacity-50" />
      </Button>
    </PopoverTrigger>
    <PopoverContent class="w-80 p-0" align="start">
      <Command :model-value="modelValue">
        <div class="p-1 pb-0">
          <ListboxFilter :model-value="search" auto-focus placeholder="Buscar…" aria-label="Buscar"
            class="border-input/30 bg-input/30 h-8 w-full rounded-lg border px-2 text-sm outline-hidden"
            @update:model-value="onSearch" />
        </div>
        <CommandList>
          <p v-if="!options.length" class="text-muted-foreground py-6 text-center text-sm" role="status">
            {{ loading ? "Carregando…" : "Nada encontrado." }}
          </p>
          <CommandGroup>
            <CommandItem v-for="r in options" :key="keyOf(r)" :value="keyOf(r)" @select="pick(r)">
              {{ optionLabel(r, keyCol, target) }}
            </CommandItem>
          </CommandGroup>
        </CommandList>
      </Command>
    </PopoverContent>
  </Popover>
</template>

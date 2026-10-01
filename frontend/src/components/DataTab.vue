<script setup lang="ts">
import { computed, onBeforeUnmount, ref, toRaw, watch } from "vue"
import { toast } from "vue-sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import DataGrid from "./DataGrid.vue"
import RowSheet from "./RowSheet.vue"
import { api } from "@/lib/api"
import { toAppError } from "@/lib/errors"
import { outgoingTarget, type NavEntry } from "@/lib/nav"
import type { ForeignKey, Page, Row, TableSchema } from "@/lib/types"
import { useViewer } from "@/stores/viewer"

const props = defineProps<{ entry: NavEntry }>()
const store = useViewer()

const pageSize = 50
const schema = ref<TableSchema | null>(null)
const page = ref<Page | null>(null)
const pageNo = ref(1)
const orderBy = ref("")
const desc = ref(false)
const filter = ref("")
const selected = ref<Row | null>(null) // snapshot copy edited by the row sheet
const sheetOpen = ref(false)
const loading = ref(false)

const rows = computed(() => page.value?.rows ?? [])
const pages = computed(() => Math.max(1, Math.ceil((page.value?.total ?? 0) / pageSize)))
const readOnly = computed(() => !!schema.value?.readOnly || !!store.db?.readOnly)

let requestId = 0
let appliedFilter = "" // filter value used by the latest load; avoids redundant debounced reloads
async function load(clamped = false) {
  const id = ++requestId
  appliedFilter = filter.value
  loading.value = true
  try {
    const entry = props.entry
    let sch = schema.value
    if (!sch || sch.name !== entry.table) sch = await api.getTable(entry.table)
    const result = await api.queryRows({
      table: entry.table, page: pageNo.value, pageSize,
      orderBy: orderBy.value, desc: desc.value, filter: filter.value, where: entry.where,
    })
    if (id !== requestId) return // a newer load superseded this one
    schema.value = sch
    page.value = result
    const last = Math.max(1, Math.ceil(result.total / pageSize))
    if (!clamped && pageNo.value > last) { // e.g. deleted the last row of the last page
      pageNo.value = last
      await load(true)
      return
    }
  } catch (e) {
    if (id === requestId) toast.error(toAppError(e).message)
  } finally {
    if (id === requestId) loading.value = false
  }
}

let timer: ReturnType<typeof setTimeout> | undefined
watch(filter, () => {
  clearTimeout(timer)
  if (filter.value === appliedFilter) return
  timer = setTimeout(() => { pageNo.value = 1; load() }, 250)
})
watch(
  () => props.entry,
  () => {
    clearTimeout(timer)
    pageNo.value = 1
    orderBy.value = ""
    desc.value = false
    selected.value = null
    sheetOpen.value = false
    filter.value = ""
    load()
  },
  { immediate: true },
)
onBeforeUnmount(() => clearTimeout(timer))

function sort(column: string) {
  if (orderBy.value === column) desc.value = !desc.value
  else { orderBy.value = column; desc.value = false }
  pageNo.value = 1
  load()
}
function go(n: number) {
  pageNo.value = Math.min(pages.value, Math.max(1, n))
  load()
}
function select(row: Row) {
  selected.value = structuredClone(toRaw(row))
  sheetOpen.value = true
}
function create() {
  selected.value = null
  sheetOpen.value = true
}
function follow(fk: ForeignKey, row: Row) { store.follow(outgoingTarget(fk, row.values)) }

defineExpose({ reload: () => load(), selected, sheetOpen })
</script>

<template>
  <div v-if="schema && page" class="flex h-full flex-col">
    <div class="flex items-center gap-2 border-b p-3">
      <Input v-model="filter" placeholder="Filtrar texto…" class="max-w-xs" />
      <span class="text-muted-foreground text-sm">{{ page.total }} registros</span>
      <div class="flex-1" />
      <Button size="sm" variant="outline" :disabled="pageNo <= 1" aria-label="Página anterior" @click="go(pageNo - 1)">‹</Button>
      <span class="text-sm">{{ pageNo }} / {{ pages }}</span>
      <Button size="sm" variant="outline" :disabled="pageNo >= pages" aria-label="Próxima página" @click="go(pageNo + 1)">›</Button>
      <Button v-if="!readOnly" size="sm" @click="create">Novo</Button>
    </div>
    <div class="min-h-0 flex-1 overflow-auto" :class="{ 'opacity-60': loading }">
      <DataGrid :schema="schema" :rows="rows" :order-by="orderBy" :desc="desc"
        @sort="sort" @select="select" @follow="follow" />
    </div>
    <RowSheet v-model:open="sheetOpen" :table="schema" :row="selected" :read-only="readOnly"
      @saved="load()" @deleted="load()" />
  </div>
</template>

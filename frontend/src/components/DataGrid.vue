<script setup lang="ts">
import { computed, h } from "vue"
import { FlexRender, getCoreRowModel, useVueTable, type ColumnDef } from "@tanstack/vue-table"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import CellValue from "./CellValue.vue"
import { fkForColumn } from "@/lib/fk"
import type { ForeignKey, Row, TableSchema } from "@/lib/types"

const props = defineProps<{ schema: TableSchema; rows: Row[]; orderBy: string; desc: boolean }>()
const emit = defineEmits<{ sort: [column: string]; select: [row: Row]; follow: [fk: ForeignKey, row: Row] }>()

const schemaColumns = computed(() => props.schema.columns ?? [])

const columns = computed<ColumnDef<Row>[]>(() =>
  schemaColumns.value.map((c) => ({
    id: c.name,
    accessorFn: (r) => r.values[c.name],
    header: () =>
      h(
        "button",
        { type: "button", class: "flex items-center gap-1 font-medium", onClick: () => emit("sort", c.name) },
        [c.name, props.orderBy === c.name ? (props.desc ? " ↓" : " ↑") : ""],
      ),
    cell: ({ row }) =>
      h(CellValue, {
        value: row.original.values[c.name],
        fk: fkForColumn(props.schema.foreignKeys, c.name),
        onFollow: (fk: ForeignKey) => emit("follow", fk, row.original),
      }),
  })),
)

function ariaSort(column: string): "ascending" | "descending" | "none" {
  if (props.orderBy !== column) return "none"
  return props.desc ? "descending" : "ascending"
}

// Only react to keys pressed on the row itself, not on buttons inside it.
function onRowKey(e: KeyboardEvent, row: Row) {
  if (e.target !== e.currentTarget) return
  if (e.key === "Enter" || e.key === " ") {
    e.preventDefault()
    emit("select", row)
  }
}

const table = useVueTable({
  get data() { return props.rows },
  get columns() { return columns.value },
  getCoreRowModel: getCoreRowModel(),
  manualPagination: true,
  manualSorting: true,
})
</script>

<template>
  <Table>
    <TableHeader>
      <TableRow v-for="hg in table.getHeaderGroups()" :key="hg.id">
        <TableHead v-for="header in hg.headers" :key="header.id" :aria-sort="ariaSort(header.column.id)">
          <FlexRender v-if="!header.isPlaceholder" :render="header.column.columnDef.header" :props="header.getContext()" />
        </TableHead>
      </TableRow>
    </TableHeader>
    <TableBody>
      <TableRow v-for="row in table.getRowModel().rows" :key="row.id" tabindex="0"
        class="focus-visible:ring-ring cursor-pointer outline-none focus-visible:ring-2 focus-visible:ring-inset"
        @click="emit('select', row.original)" @keydown="onRowKey($event, row.original)">
        <TableCell v-for="cell in row.getVisibleCells()" :key="cell.id">
          <FlexRender :render="cell.column.columnDef.cell" :props="cell.getContext()" />
        </TableCell>
      </TableRow>
      <TableRow v-if="!rows.length">
        <TableCell :colspan="Math.max(1, schemaColumns.length)" class="text-muted-foreground py-8 text-center">Nenhum registro.</TableCell>
      </TableRow>
    </TableBody>
  </Table>
</template>

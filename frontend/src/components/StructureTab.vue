<script setup lang="ts">
import { ref, watch } from "vue"
import { toast } from "vue-sonner"
import { Badge } from "@/components/ui/badge"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { api } from "@/lib/api"
import { toAppError } from "@/lib/errors"
import type { NavEntry } from "@/lib/nav"
import type { TableSchema } from "@/lib/types"
import { useViewer } from "@/stores/viewer"

const props = defineProps<{ table: string }>()
const store = useViewer()
const schema = ref<TableSchema | null>(null)
watch(
  () => props.table,
  async (t) => {
    try {
      const s = await api.getTable(t)
      if (t === props.table) schema.value = s
    } catch (e) {
      toast.error(toAppError(e).message)
    }
  },
  { immediate: true },
)

const go = (table: string) => store.follow({ table, where: [], label: table } as NavEntry)
</script>

<template>
  <div v-if="schema" class="space-y-6 p-4">
    <section>
      <h3 class="mb-2 font-medium">Colunas</h3>
      <Table>
        <TableHeader><TableRow>
          <TableHead>Nome</TableHead><TableHead>Tipo</TableHead><TableHead>Obrigatório</TableHead><TableHead>Padrão</TableHead><TableHead />
        </TableRow></TableHeader>
        <TableBody>
          <TableRow v-for="c in schema.columns ?? []" :key="c.name">
            <TableCell class="font-mono">{{ c.name }}</TableCell>
            <TableCell>{{ c.type || "—" }}</TableCell>
            <TableCell>{{ c.notNull ? "sim" : "não" }}</TableCell>
            <TableCell class="font-mono">{{ c.default ?? "—" }}</TableCell>
            <TableCell class="space-x-1">
              <Badge v-if="c.pk" variant="secondary">PK</Badge>
              <Badge v-if="c.generated" variant="outline">gerada</Badge>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
      <p v-if="schema.usesRowId" class="text-muted-foreground mt-2 text-sm">Sem chave primária: linhas identificadas por rowid.</p>
      <p v-if="schema.readOnly" class="text-muted-foreground mt-2 text-sm">Somente leitura.</p>
    </section>

    <section>
      <h3 class="mb-2 font-medium">Referencia (saída)</h3>
      <p v-if="!(schema.foreignKeys ?? []).length" class="text-muted-foreground text-sm">Nenhuma chave estrangeira.</p>
      <ul class="space-y-1">
        <li v-for="(fk, i) in schema.foreignKeys ?? []" :key="i" class="text-sm">
          <span class="font-mono">{{ fk.from.join(", ") }}</span> →
          <button type="button" class="text-primary underline" @click="go(fk.table)">{{ fk.table }}</button>
          (<span class="font-mono">{{ fk.to.join(", ") }}</span>)
          <span class="text-muted-foreground"> · ON DELETE {{ fk.onDelete }}</span>
        </li>
      </ul>
    </section>

    <section>
      <h3 class="mb-2 font-medium">Referenciada por (entrada)</h3>
      <p v-if="!(schema.incoming ?? []).length" class="text-muted-foreground text-sm">Nenhuma tabela referencia esta.</p>
      <ul class="space-y-1">
        <li v-for="(fk, i) in schema.incoming ?? []" :key="i" class="text-sm">
          <button type="button" class="text-primary underline" @click="go(fk.table)">{{ fk.table }}</button>
          (<span class="font-mono">{{ fk.from.join(", ") }}</span>) → <span class="font-mono">{{ fk.to.join(", ") }}</span>
        </li>
      </ul>
    </section>
  </div>
</template>

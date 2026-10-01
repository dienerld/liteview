<script setup lang="ts">
import { computed, watch } from "vue"
import { toast } from "vue-sonner"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Sheet, SheetContent, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import { inputKind } from "@/lib/columnKind"
import { useRowForm } from "@/lib/useRowForm"
import { isBlob, type Column, type Row, type TableSchema } from "@/lib/types"

const props = defineProps<{ open: boolean; table: TableSchema; row: Row | null; readOnly: boolean }>()
const emit = defineEmits<{ "update:open": [v: boolean]; saved: []; deleted: [] }>()

const f = useRowForm(() => props.table, () => props.row)
watch(() => [props.open, props.table.name, props.row], () => { if (props.open) f.reset() }, { immediate: true })

const columns = computed(() => (props.table.columns ?? []).filter((c) => !c.generated))
const title = computed(() => (props.row ? "Editar registro" : "Novo registro"))

const blobSize = (c: Column): number | null => {
  const v = props.row?.values[c.name]
  return isBlob(v) ? v.$blob : null
}
const isNull = (c: Column) => f.form.value[c.name]?.mode === "null"
const isDisabled = (c: Column) => props.readOnly || isNull(c)
const errorId = (c: Column) => `f-${c.name}-err`
const describedBy = (c: Column) => (f.fieldErrors.value[c.name] ? errorId(c) : undefined)
const invalid = (c: Column) => (f.fieldErrors.value[c.name] ? true : undefined)

function placeholder(c: Column): string {
  const st = f.form.value[c.name]
  if (st?.mode !== "default") return ""
  if (c.default) return c.default
  const k = inputKind(c.type)
  if (k === "date") return "AAAA-MM-DD"
  if (k === "datetime") return "AAAA-MM-DD HH:MM:SS"
  return "padrão"
}
const inputMode = (c: Column) => {
  const k = inputKind(c.type)
  return k === "integer" ? "numeric" : k === "number" ? "decimal" : "text"
}
function touch(c: Column) {
  const st = f.form.value[c.name]
  if (st && st.mode !== "value") st.mode = "value"
}
function toggleNull(c: Column) {
  const st = f.form.value[c.name]
  if (st) st.mode = st.mode === "null" ? "value" : "null"
}
function setBool(c: Column, v: boolean | "indeterminate") {
  const st = f.form.value[c.name]
  if (!st) return
  st.bool = v === true
  touch(c)
}

async function save() {
  if (props.readOnly) return
  if (await f.submit()) { toast.success("Salvo"); emit("saved"); emit("update:open", false) }
}
async function del() {
  if (await f.remove()) { toast.success("Registro excluído"); emit("deleted"); emit("update:open", false) }
}
</script>

<template>
  <Sheet :open="open" @update:open="emit('update:open', $event)">
    <SheetContent class="flex w-[440px] flex-col gap-0 overflow-y-auto sm:max-w-[440px]">
      <SheetHeader><SheetTitle>{{ title }} — {{ table.name }}</SheetTitle></SheetHeader>

      <form class="flex-1 space-y-4 px-4 pb-4" @submit.prevent="save">
        <div v-for="c in columns" :key="c.name" class="space-y-1">
          <div class="flex items-center justify-between">
            <Label :for="`f-${c.name}`">
              {{ c.name }}
              <span class="text-muted-foreground text-xs">{{ c.type }}<template v-if="c.notNull"> · obrigatório</template><template v-if="c.pk"> · PK</template></span>
            </Label>
            <Button v-if="!c.notNull && !readOnly && blobSize(c) === null" type="button" variant="ghost" size="sm"
              :class="{ 'text-primary': isNull(c) }" :aria-pressed="isNull(c)" :aria-label="`Gravar ${c.name} como NULL`"
              @click="toggleNull(c)">NULL</Button>
          </div>

          <p v-if="blobSize(c) !== null" :id="`f-${c.name}`" class="text-muted-foreground text-sm">&lt;blob {{ blobSize(c) }} bytes&gt; (não editável)</p>
          <template v-else-if="f.form.value[c.name]">
            <div v-if="inputKind(c.type) === 'boolean'" class="flex items-center gap-2">
              <Checkbox :id="`f-${c.name}`" :model-value="f.form.value[c.name].bool" :disabled="isDisabled(c)"
                :aria-invalid="invalid(c)" :aria-describedby="describedBy(c)"
                @update:model-value="setBool(c, $event)" />
            </div>
            <Textarea v-else-if="inputKind(c.type) === 'textarea'" :id="`f-${c.name}`" v-model="f.form.value[c.name].text" rows="3"
              :disabled="isDisabled(c)" :placeholder="placeholder(c)"
              :aria-invalid="invalid(c)" :aria-describedby="describedBy(c)" @input="touch(c)" />
            <Input v-else :id="`f-${c.name}`" v-model="f.form.value[c.name].text" type="text" :inputmode="inputMode(c)"
              :disabled="isDisabled(c)" :placeholder="placeholder(c)"
              :aria-invalid="invalid(c)" :aria-describedby="describedBy(c)" @input="touch(c)" />
            <p v-if="isNull(c)" class="text-muted-foreground text-xs">Será gravado como NULL.</p>
          </template>
          <p v-if="f.fieldErrors.value[c.name]" :id="errorId(c)" class="text-destructive text-sm" role="alert">{{ f.fieldErrors.value[c.name] }}</p>
        </div>

        <p v-if="f.generalError.value" class="text-destructive text-sm" role="alert">{{ f.generalError.value }}</p>
        <slot name="extra" />
      </form>

      <SheetFooter class="flex-row justify-between border-t p-4">
        <AlertDialog v-if="row && !readOnly">
          <AlertDialogTrigger as-child><Button type="button" variant="destructive" :disabled="f.saving.value">Excluir</Button></AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Excluir este registro?</AlertDialogTitle>
              <AlertDialogDescription>Essa ação não pode ser desfeita.</AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancelar</AlertDialogCancel>
              <AlertDialogAction @click="del">Excluir</AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
        <span v-else />
        <div class="flex gap-2">
          <Button type="button" variant="outline" @click="emit('update:open', false)">{{ readOnly ? "Fechar" : "Cancelar" }}</Button>
          <Button v-if="!readOnly" type="button" :disabled="f.saving.value" @click="save">Salvar</Button>
        </div>
      </SheetFooter>
    </SheetContent>
  </Sheet>
</template>

<script setup lang="ts">
import { ref, watch } from "vue"
import { toast } from "vue-sonner"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { api } from "@/lib/api"
import { toAppError } from "@/lib/errors"
import { incomingTarget } from "@/lib/nav"
import type { IncomingFK, RefCount, Row } from "@/lib/types"
import { useViewer } from "@/stores/viewer"

const props = defineProps<{ table: string; row: Row; incoming: IncomingFK[] }>()
const emit = defineEmits<{ navigate: [] }>()
const store = useViewer()
const refs = ref<RefCount[]>([])
let seq = 0

watch(
  () => [props.table, props.row, props.incoming.length] as const,
  async () => {
    const mine = ++seq
    if (!props.incoming.length) { refs.value = []; return }
    refs.value = []
    try {
      const res = (await api.references(props.table, props.row.key)) ?? []
      if (mine === seq) refs.value = res
    } catch (e) {
      if (mine === seq) { refs.value = []; toast.error(toAppError(e).message) }
    }
  },
  { immediate: true },
)

const plural = (n: number) => `${n} ${n === 1 ? "registro" : "registros"}`

function go(r: RefCount) {
  store.follow(incomingTarget(r, props.row.values))
  emit("navigate")
}
</script>

<template>
  <section v-if="refs.length" class="flex flex-col gap-2" aria-labelledby="referenced-by-title">
    <Separator />
    <h4 id="referenced-by-title" class="text-sm font-medium">Referenciado por</h4>
    <ul class="flex flex-col gap-1">
      <li v-for="(r, i) in refs" :key="i" class="flex flex-wrap items-baseline gap-x-1 text-sm">
        <Button type="button" variant="link" size="sm" class="h-auto p-0" @click="go(r)">{{ r.table }}</Button>
        <span class="text-muted-foreground">· {{ plural(r.count) }} (via {{ (r.from ?? []).join(", ") }})</span>
      </li>
    </ul>
  </section>
</template>

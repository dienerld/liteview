<script setup lang="ts">
import { computed } from "vue"
import { isBlob, type ForeignKey } from "@/lib/types"

const props = defineProps<{ value: unknown; fk?: ForeignKey }>()
const emit = defineEmits<{ follow: [fk: ForeignKey] }>()
const text = computed(() => (props.value === "" ? "(vazio)" : String(props.value)))
</script>

<template>
  <span v-if="value === null || value === undefined" class="text-muted-foreground italic">NULL</span>
  <span v-else-if="isBlob(value)" class="text-muted-foreground">&lt;blob {{ value.$blob }} bytes&gt;</span>
  <button
    v-else-if="fk"
    type="button"
    class="text-primary max-w-xs truncate underline underline-offset-2"
    :title="`Ir para ${fk.table}`"
    @click.stop="emit('follow', fk)"
  >{{ text }}</button>
  <span v-else-if="value === ''" class="text-muted-foreground italic">(vazio)</span>
  <span v-else class="block max-w-xs truncate" :title="text">{{ text }}</span>
</template>

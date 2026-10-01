import { ref } from "vue"
import { api } from "@/lib/api"
import { toAppError } from "@/lib/errors"
import { initialState, splitErrors, toPayload, type FormState } from "@/lib/formValues"
import type { Row, TableSchema } from "@/lib/types"

export function useRowForm(getTable: () => TableSchema, getRow: () => Row | null) {
  const form = ref<FormState>({})
  const fieldErrors = ref<Record<string, string>>({})
  const generalError = ref("")
  const saving = ref(false)

  function reset() {
    form.value = initialState(getTable().columns, getRow())
    fieldErrors.value = {}
    generalError.value = ""
  }

  function fail(e: unknown) {
    const r = splitErrors(toAppError(e), getTable().columns)
    fieldErrors.value = r.fields
    generalError.value = r.general
  }

  async function submit(): Promise<boolean> {
    const table = getTable()
    const row = getRow()
    const { values, errors } = toPayload(table.columns, form.value, row?.values ?? null)
    fieldErrors.value = errors
    generalError.value = ""
    if (Object.keys(errors).length) return false
    if (row && Object.keys(values).length === 0) return true // nothing changed
    saving.value = true
    try {
      if (row) await api.updateRow(table.name, row.key, values)
      else await api.insertRow(table.name, values)
      return true
    } catch (e) {
      fail(e)
      return false
    } finally {
      saving.value = false
    }
  }

  async function remove(): Promise<boolean> {
    const row = getRow()
    if (!row) return false
    saving.value = true
    generalError.value = ""
    try {
      await api.deleteRow(getTable().name, row.key)
      return true
    } catch (e) {
      fail(e)
      return false
    } finally {
      saving.value = false
    }
  }

  return { form, fieldErrors, generalError, saving, reset, submit, remove }
}

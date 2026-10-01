import { inputKind } from "./columnKind"
import type { AppError } from "./errors"
import { isBlob, type Column, type Row, type Values } from "./types"

export type FieldMode = "value" | "null" | "default"
export interface FieldState { mode: FieldMode; text: string; bool: boolean }
export type FormState = Record<string, FieldState>

const editable = (c: Column) => !c.generated

export function initialState(cols: Column[], row: Row | null): FormState {
  const form: FormState = {}
  for (const c of cols.filter(editable)) {
    if (!row) { form[c.name] = { mode: "default", text: "", bool: false }; continue }
    const v = row.values[c.name]
    if (v === null || v === undefined) form[c.name] = { mode: "null", text: "", bool: false }
    else if (isBlob(v)) form[c.name] = { mode: "value", text: "", bool: false }
    else form[c.name] = { mode: "value", text: String(v), bool: v === 1 || v === true || v === "1" }
  }
  return form
}

function parse(c: Column, st: FieldState): { ok: true; value: unknown } | { ok: false; error: string } {
  const kind = inputKind(c.type)
  if (kind === "boolean") return { ok: true, value: st.bool ? 1 : 0 }
  if (kind === "integer") {
    const t = st.text.trim()
    if (!/^-?\d+$/.test(t)) return { ok: false, error: "informe um número inteiro" }
    const n = Number(t)
    return { ok: true, value: Number.isSafeInteger(n) ? n : t }
  }
  if (kind === "number") {
    const t = st.text.trim()
    const n = Number(t)
    if (t === "" || !Number.isFinite(n)) return { ok: false, error: "informe um número" }
    return { ok: true, value: n }
  }
  return { ok: true, value: st.text }
}

export function toPayload(cols: Column[], form: FormState, original: Values | null) {
  const values: Values = {}
  const errors: Record<string, string> = {}
  for (const c of cols.filter(editable)) {
    const st = form[c.name]
    if (!st) continue
    if (original && isBlob(original[c.name])) continue // blobs are never edited
    if (!original && st.mode === "default") continue
    let next: unknown
    if (st.mode === "null") next = null
    else {
      const p = parse(c, st)
      if (!p.ok) { errors[c.name] = p.error; continue }
      next = p.value
    }
    if (original) {
      const prev = original[c.name]
      const same = prev === next || (prev !== null && next !== null && String(prev) === String(next))
      if (same) continue
    }
    values[c.name] = next
  }
  return { values, errors }
}

export function splitErrors(err: AppError, cols: Column[]) {
  const fields: Record<string, string> = {}
  for (const name of err.columns ?? []) {
    if (cols.some((c) => c.name === name)) fields[name] = err.message
  }
  return { fields, general: Object.keys(fields).length ? "" : err.message }
}

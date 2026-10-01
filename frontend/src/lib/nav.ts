import type { Cond, ForeignKey, IncomingFK, Values } from "./types"

export interface NavEntry { table: string; where: Cond[]; label: string }

export const rootEntry = (table: string): NavEntry[] => [{ table, where: [], label: table }]
export const pushEntry = (trail: NavEntry[], e: NavEntry): NavEntry[] => [...trail, e]
export const popTo = (trail: NavEntry[], index: number): NavEntry[] => trail.slice(0, index + 1)

function describe(table: string, where: Cond[]): string {
  const f = where.map((c) => `${c.column} = ${c.value === null ? "NULL" : String(c.value)}`).join(", ")
  return f ? `${table} (${f})` : table
}

function entry(table: string, where: Cond[]): NavEntry {
  return { table, where, label: describe(table, where) }
}

/** Follow an FK from a row: referenced table filtered to the row the FK points at. */
export function outgoingTarget(fk: ForeignKey, values: Values): NavEntry {
  return entry(fk.table, fk.to.map((col, i) => ({ column: col, value: values[fk.from[i]] ?? null })))
}

/** Follow an incoming FK: referencing table filtered to rows pointing at this row. */
export function incomingTarget(inc: Pick<IncomingFK, "table" | "from" | "to">, values: Values): NavEntry {
  return entry(inc.table, inc.from.map((col, i) => ({ column: col, value: values[inc.to[i]] ?? null })))
}

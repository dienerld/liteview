import type { Column, ForeignKey, Row, TableSchema } from "./types"

/** The FK a column belongs to, only when it is single-column with a known target column. */
export function singleColumnFk(fks: ForeignKey[] | null | undefined, column: string): ForeignKey | undefined {
  return (fks ?? []).find((fk) => fk.from?.length === 1 && fk.from[0] === column && !!fk.to?.[0])
}

/** First text-like column of the referenced table (other than the key) used as a human hint. */
function hintColumn(keyCol: string, target: Pick<TableSchema, "columns"> | null): Column | undefined {
  return (target?.columns ?? []).find((c) => c.name !== keyCol && /CHAR|TEXT|CLOB/i.test(c.type))
}

/** "<key> — <first text column>" when available, otherwise just the key. */
export function optionLabel(row: Row, keyCol: string, target: Pick<TableSchema, "columns"> | null): string {
  const key = String(row.values[keyCol])
  const hint = hintColumn(keyCol, target)
  const extra = hint ? row.values[hint.name] : null
  return extra !== null && extra !== undefined && extra !== "" ? `${key} — ${String(extra)}` : key
}

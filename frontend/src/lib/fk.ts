import type { ForeignKey } from "./types"

/** The FK a column participates in (for composite FKs, every member column maps to the same FK). */
export function fkForColumn(fks: ForeignKey[] | null | undefined, column: string): ForeignKey | undefined {
  return (fks ?? []).find((fk) => fk.from.includes(column))
}

/** True when the column is part of the table's primary key (single or composite). */
export function isPkColumn(primaryKey: string[] | null | undefined, column: string): boolean {
  return (primaryKey ?? []).includes(column)
}

/** Tooltip for the FK mark: where this column points to ("tabela.coluna", or just the table when unknown). */
export function fkTitle(fk: ForeignKey, column: string): string {
  const target = fk.to?.[fk.from.indexOf(column)]
  return `Chave estrangeira → ${fk.table}${target ? `.${target}` : ""}`
}

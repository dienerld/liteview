import type { ForeignKey } from "./types"

/** The FK a column participates in (for composite FKs, every member column maps to the same FK). */
export function fkForColumn(fks: ForeignKey[] | null | undefined, column: string): ForeignKey | undefined {
  return (fks ?? []).find((fk) => fk.from.includes(column))
}

export interface Column { name: string; type: string; notNull: boolean; default: string | null; pk: number; generated: boolean }
export interface ForeignKey { table: string; from: string[]; to: string[]; onUpdate: string; onDelete: string }
export interface IncomingFK { table: string; from: string[]; to: string[] }
export interface TableInfo { name: string; kind: "table" | "view" }
export interface TableSchema {
  name: string; kind: "table" | "view"; readOnly: boolean
  columns: Column[]; primaryKey: string[]; usesRowId: boolean; keyColumns: string[]
  foreignKeys: ForeignKey[]; incoming: IncomingFK[]
}
export type Values = Record<string, unknown>
export interface Row { key: Values; values: Values }
export interface Page { rows: Row[]; total: number; page: number; pageSize: number }
export interface Cond { column: string; value: unknown }
export interface RowQuery { table: string; page: number; pageSize: number; orderBy: string; desc: boolean; filter: string; where: Cond[] }
export interface RefCount { table: string; from: string[]; to: string[]; count: number }
export interface DbInfo { path: string; name: string; readOnly: boolean }

export const isBlob = (v: unknown): v is { $blob: number } =>
  typeof v === "object" && v !== null && "$blob" in v

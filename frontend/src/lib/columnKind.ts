export type InputKind = "boolean" | "integer" | "number" | "date" | "datetime" | "textarea" | "text"

export function inputKind(decl: string): InputKind {
  const d = decl.toUpperCase()
  if (d.includes("BOOL")) return "boolean"
  if (d.includes("INT")) return "integer"
  if (/REAL|FLOA|DOUB|NUMERIC|DECIMAL/.test(d)) return "number"
  if (d.includes("DATETIME") || d.includes("TIMESTAMP")) return "datetime"
  if (d.includes("DATE")) return "date"
  if (/TEXT|CLOB/.test(d)) return "textarea"
  return "text"
}

/** True when the DECLARED type contains BLOB (an empty declared type is not a BLOB column). */
export const isBlobColumn = (c: { type: string }): boolean => c.type.toUpperCase().includes("BLOB")

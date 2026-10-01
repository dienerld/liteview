import { describe, expect, it } from "vitest"
import { optionLabel, singleColumnFk } from "./fkOptions"
import type { Column, ForeignKey, Row, TableSchema } from "./types"

const col = (name: string, type = "TEXT"): Column => ({ name, type, notNull: false, default: null, pk: 0, generated: false })
const fk = (from: string[], to: string[], table = "users"): ForeignKey => ({ table, from, to, onUpdate: "", onDelete: "" })
const users: Pick<TableSchema, "columns"> = { columns: [col("id", "INTEGER"), col("name", "VARCHAR(80)"), col("bio", "TEXT")] }
const row = (values: Record<string, unknown>): Row => ({ key: {}, values })

describe("singleColumnFk", () => {
  it("returns the FK for a single-column foreign key", () => {
    const f = fk(["user_id"], ["id"])
    expect(singleColumnFk([f], "user_id")).toBe(f)
  })
  it("ignores composite FKs, other columns, missing target and null lists", () => {
    expect(singleColumnFk([fk(["a", "b"], ["x", "y"])], "a")).toBeUndefined()
    expect(singleColumnFk([fk(["user_id"], ["id"])], "other")).toBeUndefined()
    expect(singleColumnFk([fk(["user_id"], [""])], "user_id")).toBeUndefined()
    expect(singleColumnFk([fk(["user_id"], [])], "user_id")).toBeUndefined()
    expect(singleColumnFk(null, "user_id")).toBeUndefined()
  })
})

describe("optionLabel", () => {
  it("joins key and the first text column", () => {
    expect(optionLabel(row({ id: 1, name: "Ana", bio: "x" }), "id", users)).toBe("1 — Ana")
  })
  it("falls back to the key when the text value is empty or null", () => {
    expect(optionLabel(row({ id: 2, name: null }), "id", users)).toBe("2")
    expect(optionLabel(row({ id: 3, name: "" }), "id", users)).toBe("3")
  })
  it("uses only the key without a target schema or text column", () => {
    expect(optionLabel(row({ id: 4, name: "Z" }), "id", null)).toBe("4")
    expect(optionLabel(row({ id: 5 }), "id", { columns: [col("id", "INTEGER")] })).toBe("5")
  })
  it("tolerates a null columns list and never picks the key column", () => {
    expect(optionLabel(row({ id: 6 }), "id", { columns: null as unknown as Column[] })).toBe("6")
    expect(optionLabel(row({ code: "A1", name: "Q" }), "code", { columns: [col("code"), col("name")] })).toBe("A1 — Q")
  })
})

import { describe, expect, it } from "vitest"
import { fkForColumn, fkTitle, isPkColumn } from "./fk"
import type { ForeignKey } from "./types"

const single: ForeignKey = { table: "users", from: ["user_id"], to: ["id"], onUpdate: "", onDelete: "" }
const composite: ForeignKey = { table: "t", from: ["a", "b"], to: ["x", "y"], onUpdate: "", onDelete: "" }

describe("fkForColumn", () => {
  it("finds the FK containing the column", () => {
    expect(fkForColumn([single, composite], "user_id")).toBe(single)
  })
  it("returns the same FK for every column of a composite FK", () => {
    expect(fkForColumn([single, composite], "a")).toBe(composite)
    expect(fkForColumn([single, composite], "b")).toBe(composite)
  })
  it("returns undefined for no match or null/undefined list", () => {
    expect(fkForColumn([single], "other")).toBeUndefined()
    expect(fkForColumn(null, "a")).toBeUndefined()
    expect(fkForColumn(undefined, "a")).toBeUndefined()
  })
})

describe("isPkColumn", () => {
  it("is true only for primary key columns (including composite keys)", () => {
    expect(isPkColumn(["id"], "id")).toBe(true)
    expect(isPkColumn(["a", "b"], "b")).toBe(true)
    expect(isPkColumn(["id"], "name")).toBe(false)
  })
  it("tolerates null/undefined lists", () => {
    expect(isPkColumn(null, "id")).toBe(false)
    expect(isPkColumn(undefined, "id")).toBe(false)
  })
})

describe("fkTitle", () => {
  it("describes a single-column FK target", () => {
    expect(fkTitle(single, "user_id")).toBe("Chave estrangeira → users.id")
  })
  it("points a composite FK member at its own target column", () => {
    expect(fkTitle(composite, "b")).toBe("Chave estrangeira → t.y")
  })
  it("falls back to the table when the target column is unknown", () => {
    expect(fkTitle({ ...single, to: [""] }, "user_id")).toBe("Chave estrangeira → users")
  })
})

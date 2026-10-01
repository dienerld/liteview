import { describe, expect, it } from "vitest"
import { fkForColumn } from "./fk"
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

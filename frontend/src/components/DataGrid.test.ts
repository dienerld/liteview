import { describe, expect, it } from "vitest"
import { mount } from "@vue/test-utils"
import DataGrid from "./DataGrid.vue"
import type { Row, TableSchema } from "@/lib/types"

const col = (name: string) => ({ name, type: "", notNull: false, default: null, pk: 0, generated: false })
const schema: TableSchema = {
  name: "posts", kind: "table", readOnly: false,
  columns: [col("id"), col("user_id")], primaryKey: ["id"], usesRowId: false, keyColumns: ["id"],
  foreignKeys: [{ table: "users", from: ["user_id"], to: ["id"], onUpdate: "", onDelete: "" }],
  incoming: [],
}
const rows: Row[] = [
  { key: { id: 1 }, values: { id: 1, user_id: 10 } },
  { key: { id: 2 }, values: { id: 2, user_id: 20 } },
]
const mountGrid = (orderBy = "", desc = false) =>
  mount(DataGrid, { props: { schema, rows, orderBy, desc } })

describe("DataGrid accessibility and events", () => {
  it("makes data rows focusable", () => {
    const trs = mountGrid().findAll("tbody tr")
    expect(trs).toHaveLength(2)
    trs.forEach((tr) => expect(tr.attributes("tabindex")).toBe("0"))
  })
  it("emits select on Enter and Space (Space is default-prevented)", async () => {
    const w = mountGrid()
    const tr = w.findAll("tbody tr")[1]
    await tr.trigger("keydown", { key: "Enter" })
    const ev = new KeyboardEvent("keydown", { key: " ", cancelable: true, bubbles: true })
    tr.element.dispatchEvent(ev)
    expect(ev.defaultPrevented).toBe(true)
    expect(w.emitted("select")).toEqual([[rows[1]], [rows[1]]])
  })
  it("row click still emits select", async () => {
    const w = mountGrid()
    await w.findAll("tbody tr")[0].trigger("click")
    expect(w.emitted("select")).toEqual([[rows[0]]])
  })
  it("FK link click emits follow and not select", async () => {
    const w = mountGrid()
    await w.findAll("tbody tr")[0].find("button").trigger("click")
    expect(w.emitted("follow")).toEqual([[schema.foreignKeys[0], rows[0]]])
    expect(w.emitted("select")).toBeUndefined()
  })
  it("keys on the FK link do not trigger row select", async () => {
    const w = mountGrid()
    const btn = w.findAll("tbody tr")[0].find("button")
    await btn.trigger("keydown", { key: "Enter" })
    await btn.trigger("keydown", { key: " " })
    expect(w.emitted("select")).toBeUndefined()
  })
  it("sort button emits sort, and keys on it do not emit select", async () => {
    const w = mountGrid()
    const btn = w.find("thead button")
    await btn.trigger("keydown", { key: "Enter" })
    await btn.trigger("click")
    expect(w.emitted("sort")).toEqual([["id"]])
    expect(w.emitted("select")).toBeUndefined()
  })
  it("exposes aria-sort on headers", () => {
    const attr = (w: ReturnType<typeof mountGrid>) => w.findAll("thead th").map((h) => h.attributes("aria-sort"))
    expect(attr(mountGrid())).toEqual(["none", "none"])
    expect(attr(mountGrid("user_id", false))).toEqual(["none", "ascending"])
    expect(attr(mountGrid("id", true))).toEqual(["descending", "none"])
  })
})

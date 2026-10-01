import { beforeEach, describe, expect, it, vi } from "vitest"
import { flushPromises, mount } from "@vue/test-utils"
import { createPinia, setActivePinia } from "pinia"
import type { TableSchema } from "@/lib/types"

const col = (name: string, extra = {}) => ({ name, type: "INTEGER", notNull: false, default: null, pk: 0, generated: false, ...extra })
const schema: TableSchema = {
  name: "posts", kind: "table", readOnly: false, primaryKey: ["id"], usesRowId: false, keyColumns: ["id"],
  columns: [col("id", { pk: 1 }), col("user_id", { notNull: true })],
  foreignKeys: [{ table: "users", from: ["user_id"], to: ["id"], onUpdate: "", onDelete: "CASCADE" }],
  incoming: [{ table: "comments", from: ["post_id"], to: ["id"] }],
}

vi.mock("@/lib/api", () => ({ api: { getTable: vi.fn(async () => schema) } }))

import StructureTab from "./StructureTab.vue"
import { useViewer } from "@/stores/viewer"

beforeEach(() => setActivePinia(createPinia()))

describe("StructureTab", () => {
  it("renders column rows and FK / incoming links", async () => {
    const w = mount(StructureTab, { props: { table: "posts" } })
    await flushPromises()
    expect(w.findAll("tbody tr")).toHaveLength(2)
    expect(w.text()).toContain("user_id")
    const links = w.findAll("button").map((b) => b.text())
    expect(links).toEqual(["users", "comments"])
  })

  it("follows a link through the store", async () => {
    const store = useViewer()
    const spy = vi.spyOn(store, "follow")
    const w = mount(StructureTab, { props: { table: "posts" } })
    await flushPromises()
    await w.findAll("button")[0].trigger("click")
    expect(spy).toHaveBeenCalledWith({ table: "users", where: [], label: "users" })
    await w.findAll("button")[1].trigger("click")
    expect(spy).toHaveBeenLastCalledWith({ table: "comments", where: [], label: "comments" })
  })
})

import { beforeEach, describe, expect, it, vi } from "vitest"
import { flushPromises, mount } from "@vue/test-utils"
import { createPinia, setActivePinia } from "pinia"
import type { IncomingFK, RefCount, Row } from "@/lib/types"

const references = vi.fn<(t: string, k: unknown) => Promise<RefCount[] | null>>()
vi.mock("@/lib/api", () => ({ api: { references: (t: string, k: unknown) => references(t, k) } }))
const toastError = vi.fn()
vi.mock("vue-sonner", () => ({ toast: { error: (m: string) => toastError(m) } }))

import ReferencedBy from "./ReferencedBy.vue"
import { useViewer } from "@/stores/viewer"

const incoming: IncomingFK[] = [{ table: "posts", from: ["user_id"], to: ["id"] }]
const row = (id: number): Row => ({ key: { id }, values: { id, name: "Ana" } })
const refs: RefCount[] = [
  { table: "posts", from: ["user_id"], to: ["id"], count: 2 },
  { table: "tag_notes", from: ["a", "b"], to: ["x", "y"], count: 1 },
]

beforeEach(() => { setActivePinia(createPinia()); references.mockReset(); toastError.mockReset() })

describe("ReferencedBy", () => {
  it("renders entries with pt-BR pluralisation", async () => {
    references.mockResolvedValue(refs)
    const w = mount(ReferencedBy, { props: { table: "users", row: row(1), incoming } })
    await flushPromises()
    expect(references).toHaveBeenCalledWith("users", { id: 1 })
    expect(w.text()).toContain("Referenciado por")
    expect(w.text()).toMatch(/posts\s*· 2 registros \(via user_id\)/)
    expect(w.text()).toMatch(/tag_notes\s*· 1 registro \(via a, b\)/)
  })

  it("is hidden without incoming FKs (and does not call the API) or without references", async () => {
    const a = mount(ReferencedBy, { props: { table: "users", row: row(1), incoming: [] } })
    await flushPromises()
    expect(references).not.toHaveBeenCalled()
    expect(a.find("section").exists()).toBe(false)
    references.mockResolvedValue(null)
    const b = mount(ReferencedBy, { props: { table: "users", row: row(1), incoming } })
    await flushPromises()
    expect(b.find("section").exists()).toBe(false)
  })

  it("follows the link through the store and emits navigate", async () => {
    references.mockResolvedValue(refs)
    const store = useViewer()
    const spy = vi.spyOn(store, "follow")
    const w = mount(ReferencedBy, { props: { table: "users", row: row(1), incoming } })
    await flushPromises()
    const buttons = w.findAll("button")
    expect(buttons).toHaveLength(2)
    await buttons[0].trigger("click")
    expect(spy).toHaveBeenCalledWith({ table: "posts", where: [{ column: "user_id", value: 1 }], label: "posts (user_id = 1)" })
    expect(w.emitted("navigate")).toHaveLength(1)
  })

  it("ignores a stale response when the row changes", async () => {
    let resolveFirst!: (v: RefCount[]) => void
    references.mockImplementationOnce(() => new Promise((r) => { resolveFirst = r }))
    references.mockResolvedValueOnce([{ table: "posts", from: ["user_id"], to: ["id"], count: 5 }])
    const w = mount(ReferencedBy, { props: { table: "users", row: row(1), incoming } })
    await w.setProps({ row: row(2) })
    await flushPromises()
    resolveFirst([{ table: "posts", from: ["user_id"], to: ["id"], count: 99 }])
    await flushPromises()
    expect(w.text()).toContain("5 registros")
    expect(w.text()).not.toContain("99")
  })

  it("shows a toast and stays hidden on error", async () => {
    references.mockRejectedValue(new Error("boom"))
    const w = mount(ReferencedBy, { props: { table: "users", row: row(1), incoming } })
    await flushPromises()
    expect(toastError).toHaveBeenCalledWith("boom")
    expect(w.find("section").exists()).toBe(false)
  })
})

import { beforeEach, describe, expect, it, vi } from "vitest"
import { createPinia, setActivePinia } from "pinia"

vi.mock("vue-sonner", () => ({ toast: { error: vi.fn() } }))
vi.mock("@/lib/api", () => ({
  api: {
    initialDb: vi.fn(),
    openDialog: vi.fn(),
    openPath: vi.fn(),
    recents: vi.fn(),
    forgetRecent: vi.fn(),
    listTables: vi.fn(),
  },
}))

import { toast } from "vue-sonner"
import { api } from "@/lib/api"
import { useViewer } from "./viewer"

const m = vi.mocked(api)
const info = { path: "/x/a.db", name: "a.db", readOnly: false }

beforeEach(() => {
  setActivePinia(createPinia())
  vi.resetAllMocks()
  m.recents.mockResolvedValue(["/x/a.db"])
  m.listTables.mockResolvedValue([{ name: "users", kind: "table" }, { name: "v", kind: "view" }])
})

describe("viewer store", () => {
  it("init without an initial db keeps db null and loads recents", async () => {
    m.initialDb.mockResolvedValue(null)
    const s = useViewer()
    await s.init()
    expect(s.db).toBeNull()
    expect(s.recents).toEqual(["/x/a.db"])
    expect(m.listTables).not.toHaveBeenCalled()
  })

  it("init with a db loads tables and an empty trail", async () => {
    m.initialDb.mockResolvedValue(info)
    const s = useViewer()
    await s.init()
    expect(s.db).toEqual(info)
    expect(s.tables).toHaveLength(2)
    expect(s.trail).toEqual([])
    expect(s.current).toBeNull()
  })

  it("init does not throw when recents fails", async () => {
    m.recents.mockRejectedValue(new Error("boom"))
    m.initialDb.mockResolvedValue(null)
    const s = useViewer()
    await expect(s.init()).resolves.toBeUndefined()
    expect(toast.error).toHaveBeenCalled()
  })

  it("normalises null arrays from the bindings", async () => {
    m.recents.mockResolvedValue(null as unknown as string[])
    m.listTables.mockResolvedValue(null as unknown as [])
    m.initialDb.mockResolvedValue(info)
    const s = useViewer()
    await s.init()
    expect(s.recents).toEqual([])
    expect(s.tables).toEqual([])
  })

  it("openTable, follow and backTo move through the trail", async () => {
    const s = useViewer()
    s.openTable("posts")
    expect(s.current?.table).toBe("posts")
    s.follow({ table: "users", where: [{ column: "id", value: 1 }], label: "users (id = 1)" })
    s.follow({ table: "posts", where: [], label: "posts" })
    expect(s.trail).toHaveLength(3)
    s.backTo(1)
    expect(s.current?.table).toBe("users")
    s.openTable("v")
    expect(s.trail.map((e) => e.table)).toEqual(["v"])
  })

  it("adopt resets the trail and reloads tables", async () => {
    const s = useViewer()
    s.openTable("posts")
    await s.adopt(info)
    expect(s.trail).toEqual([])
    expect(m.listTables).toHaveBeenCalledTimes(1)
  })

  it("openPath failure toasts and forgets the dead recent", async () => {
    m.openPath.mockRejectedValue(new Error("gone"))
    m.forgetRecent.mockResolvedValue(undefined)
    m.recents.mockResolvedValue([])
    const s = useViewer()
    await s.openPath("/x/dead.db")
    expect(toast.error).toHaveBeenCalled()
    expect(m.forgetRecent).toHaveBeenCalledWith("/x/dead.db")
    expect(s.recents).toEqual([])
    expect(s.db).toBeNull()
  })
})

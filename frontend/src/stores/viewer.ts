import { defineStore } from "pinia"
import { toast } from "vue-sonner"
import { api } from "@/lib/api"
import { toAppError } from "@/lib/errors"
import { popTo, pushEntry, rootEntry, type NavEntry } from "@/lib/nav"
import type { DbInfo, TableInfo } from "@/lib/types"

export const useViewer = defineStore("viewer", {
  state: () => ({
    db: null as DbInfo | null,
    tables: [] as TableInfo[],
    recents: [] as string[],
    trail: [] as NavEntry[],
  }),
  getters: {
    current: (s): NavEntry | null => s.trail[s.trail.length - 1] ?? null,
  },
  actions: {
    async refreshRecents() {
      try {
        this.recents = (await api.recents()) ?? []
      } catch (e) {
        toast.error(toAppError(e).message)
      }
    },
    async init() {
      await this.refreshRecents()
      try {
        await this.adopt(await api.initialDb())
      } catch (e) {
        toast.error(toAppError(e).message)
      }
    },
    async adopt(info: DbInfo | null) {
      if (!info) return
      this.tables = (await api.listTables()) ?? []
      this.db = info
      this.trail = []
      await this.refreshRecents()
    },
    async openDialog() {
      try {
        await this.adopt(await api.openDialog())
      } catch (e) {
        toast.error(toAppError(e).message)
      }
    },
    async openPath(path: string) {
      try {
        await this.adopt(await api.openPath(path))
      } catch (e) {
        toast.error(toAppError(e).message)
        await this.forget(path) // dead entry: drop it from the list
      }
    },
    async forget(path: string) {
      try {
        await api.forgetRecent(path)
      } catch (e) {
        toast.error(toAppError(e).message)
      }
      await this.refreshRecents()
    },
    openTable(name: string) { this.trail = rootEntry(name) },
    follow(e: NavEntry) { this.trail = pushEntry(this.trail, e) },
    backTo(index: number) { this.trail = popTo(this.trail, index) },
  },
})

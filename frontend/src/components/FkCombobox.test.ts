import { describe, expect, it, vi } from "vitest"
import { mount } from "@vue/test-utils"
import type { ForeignKey } from "@/lib/types"

vi.mock("@/lib/api", () => ({ api: { getTable: vi.fn(), queryRows: vi.fn() } }))
vi.mock("vue-sonner", () => ({ toast: { error: vi.fn() } }))

import FkCombobox from "./FkCombobox.vue"

const fk: ForeignKey = { table: "users", from: ["user_id"], to: ["id"], onUpdate: "", onDelete: "" }

describe("FkCombobox trigger", () => {
  it("shows the current value", () => {
    const w = mount(FkCombobox, { props: { fk, modelValue: "7", id: "f-user_id" } })
    const b = w.get("button")
    expect(b.text()).toBe("7")
    expect(b.attributes("id")).toBe("f-user_id")
    expect(b.attributes("role")).toBe("combobox")
  })
  it("shows the placeholder when empty and exposes invalid/describedby", () => {
    const w = mount(FkCombobox, { props: { fk, modelValue: "", invalid: true, describedby: "err" } })
    const b = w.get("button")
    expect(b.text()).toBe("Escolher em users…")
    expect(b.attributes("aria-invalid")).toBe("true")
    expect(b.attributes("aria-describedby")).toBe("err")
  })
  it("is disabled when requested", () => {
    expect(mount(FkCombobox, { props: { fk, modelValue: "1", disabled: true } }).get("button").attributes("disabled")).toBeDefined()
  })
})

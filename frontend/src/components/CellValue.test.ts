import { describe, expect, it } from "vitest"
import { mount } from "@vue/test-utils"
import CellValue from "./CellValue.vue"
import type { ForeignKey } from "@/lib/types"

const fk: ForeignKey = { table: "users", from: ["user_id"], to: ["id"], onUpdate: "", onDelete: "" }

describe("CellValue", () => {
  it("renders null as italic NULL", () => {
    const w = mount(CellValue, { props: { value: null } })
    expect(w.text()).toBe("NULL")
    expect(w.find("span").classes()).toContain("italic")
  })
  it("renders empty string as italic (vazio)", () => {
    const w = mount(CellValue, { props: { value: "" } })
    expect(w.text()).toBe("(vazio)")
    expect(w.find("span").classes()).toContain("italic")
  })
  it("renders blobs with size", () => {
    expect(mount(CellValue, { props: { value: { $blob: 4 } } }).text()).toBe("<blob 4 bytes>")
  })
  it("renders plain text and big integers as strings", () => {
    expect(mount(CellValue, { props: { value: "olá" } }).text()).toBe("olá")
    expect(mount(CellValue, { props: { value: "9007199254740993" } }).text()).toBe("9007199254740993")
  })
  it("renders a non-null FK value as a link that emits follow", async () => {
    const w = mount(CellValue, { props: { value: 1, fk } })
    const btn = w.find("button")
    expect(btn.text()).toBe("1")
    await btn.trigger("click")
    expect(w.emitted("follow")).toEqual([[fk]])
  })
  it("does not render a link for a NULL FK value", () => {
    const w = mount(CellValue, { props: { value: null, fk } })
    expect(w.find("button").exists()).toBe(false)
    expect(w.text()).toBe("NULL")
  })
  it("stops click propagation on the FK link", async () => {
    let parentClicks = 0
    const w = mount({ components: { CellValue }, template: `<div @click="n()"><CellValue :value="1" :fk="fk" /></div>`, setup: () => ({ fk, n: () => parentClicks++ }) })
    await w.find("button").trigger("click")
    expect(parentClicks).toBe(0)
  })
})

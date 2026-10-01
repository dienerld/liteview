import { describe, expect, it } from "vitest";
import { initialState, splitErrors, toPayload } from "./formValues";
import type { Column } from "./types";

const col = (name: string, type: string, extra: Partial<Column> = {}): Column =>
  ({ name, type, notNull: false, default: null, pk: 0, generated: false, ...extra });

const cols = [col("id", "INTEGER", { pk: 1 }), col("name", "TEXT"), col("age", "INTEGER"), col("score", "REAL"), col("ok", "BOOLEAN")];

describe("initialState", () => {
  it("insert: every field starts in default mode", () => {
    const f = initialState(cols, null);
    expect(Object.values(f).every((s) => s.mode === "default")).toBe(true);
  });
  it("edit: NULL becomes null mode, values become value mode", () => {
    const f = initialState(cols, { key: { id: 1 }, values: { id: 1, name: null, age: 30, score: 1.5, ok: 1 } });
    expect(f.name.mode).toBe("null");
    expect(f.age).toMatchObject({ mode: "value", text: "30" });
    expect(f.ok.bool).toBe(true);
  });
  it("skips generated columns", () => {
    expect("g" in initialState([col("g", "INTEGER", { generated: true })], null)).toBe(false);
  });
});

describe("toPayload — insert", () => {
  it("omits default-mode fields", () => {
    const f = initialState(cols, null);
    f.name = { mode: "value", text: "Ana", bool: false };
    expect(toPayload(cols, f, null)).toEqual({ values: { name: "Ana" }, errors: {} });
  });
  it("distinguishes NULL from empty string", () => {
    const f = initialState(cols, null);
    f.name = { mode: "null", text: "", bool: false };
    expect(toPayload(cols, f, null).values).toEqual({ name: null });
    f.name = { mode: "value", text: "", bool: false };
    expect(toPayload(cols, f, null).values).toEqual({ name: "" });
  });
  it("parses numbers and booleans", () => {
    const f = initialState(cols, null);
    f.age = { mode: "value", text: "42", bool: false };
    f.score = { mode: "value", text: "2.5", bool: false };
    f.ok = { mode: "value", text: "", bool: true };
    expect(toPayload(cols, f, null).values).toEqual({ age: 42, score: 2.5, ok: 1 });
  });
  it("reports empty or invalid numeric input as a field error", () => {
    const f = initialState(cols, null);
    f.age = { mode: "value", text: "", bool: false };
    f.score = { mode: "value", text: "abc", bool: false };
    const { errors, values } = toPayload(cols, f, null);
    expect(Object.keys(errors).sort()).toEqual(["age", "score"]);
    expect(values).toEqual({});
  });
  it("keeps integers beyond 2^53 as exact strings", () => {
    const f = initialState(cols, null);
    f.age = { mode: "value", text: "9007199254740993", bool: false };
    expect(toPayload(cols, f, null).values.age).toBe("9007199254740993");
  });
});

describe("toPayload — edit", () => {
  const row = { key: { id: 1 }, values: { id: 1, name: "Ana", age: 30, score: 1.5, ok: 0 } };
  it("sends only changed fields", () => {
    const f = initialState(cols, row);
    expect(toPayload(cols, f, row.values).values).toEqual({});
    f.name.text = "Ana Maria";
    expect(toPayload(cols, f, row.values).values).toEqual({ name: "Ana Maria" });
  });
  it("can change a value to NULL", () => {
    const f = initialState(cols, row);
    f.name.mode = "null";
    expect(toPayload(cols, f, row.values).values).toEqual({ name: null });
  });
  it("never sends blob columns", () => {
    const blobCols = [col("id", "INTEGER", { pk: 1 }), col("data", "BLOB")];
    const r = { key: { id: 1 }, values: { id: 1, data: { $blob: 4 } } };
    const f = initialState(blobCols, r);
    expect(toPayload(blobCols, f, r.values).values).toEqual({});
  });
});

describe("splitErrors", () => {
  it("assigns constraint errors to the matching fields", () => {
    const r = splitErrors({ message: "email é obrigatório", constraint: "NOT NULL", columns: ["name"] }, cols);
    expect(r.fields).toEqual({ name: "email é obrigatório" });
    expect(r.general).toBe("");
  });
  it("uses a general message when no column matches", () => {
    const r = splitErrors({ message: "FK falhou", constraint: "FOREIGN KEY", columns: [] }, cols);
    expect(r.fields).toEqual({});
    expect(r.general).toBe("FK falhou");
  });
});

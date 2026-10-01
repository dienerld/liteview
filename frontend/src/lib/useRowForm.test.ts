import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/api", () => ({
  api: { insertRow: vi.fn(), updateRow: vi.fn(), deleteRow: vi.fn() },
}));

import { api } from "@/lib/api";
import { useRowForm } from "./useRowForm";
import type { TableSchema } from "./types";

const table: TableSchema = {
  name: "users", kind: "table", readOnly: false, primaryKey: ["id"], usesRowId: false, keyColumns: ["id"],
  foreignKeys: [], incoming: [],
  columns: [
    { name: "id", type: "INTEGER", notNull: false, default: null, pk: 1, generated: false },
    { name: "email", type: "TEXT", notNull: true, default: null, pk: 0, generated: false },
  ],
};
const row = { key: { id: 1 }, values: { id: 1, email: "a@x.com" } };

beforeEach(() => vi.clearAllMocks());

describe("useRowForm", () => {
  it("insert calls insertRow with only provided fields", async () => {
    (api.insertRow as any).mockResolvedValue({ id: 2 });
    const f = useRowForm(() => table, () => null);
    f.reset();
    f.form.value.email = { mode: "value", text: "b@x.com", bool: false };
    expect(await f.submit()).toBe(true);
    expect(api.insertRow).toHaveBeenCalledWith("users", { email: "b@x.com" });
  });

  it("edit with no changes does not call the backend", async () => {
    const f = useRowForm(() => table, () => row);
    f.reset();
    expect(await f.submit()).toBe(true);
    expect(api.updateRow).not.toHaveBeenCalled();
  });

  it("edit sends the key and changed fields", async () => {
    (api.updateRow as any).mockResolvedValue(undefined);
    const f = useRowForm(() => table, () => row);
    f.reset();
    f.form.value.email.text = "z@x.com";
    await f.submit();
    expect(api.updateRow).toHaveBeenCalledWith("users", { id: 1 }, { email: "z@x.com" });
  });

  it("maps a constraint error onto the right field and keeps the form open", async () => {
    (api.insertRow as any).mockRejectedValue(
      Object.assign(new Error("x"), { cause: { type: "constraint", kind: "UNIQUE", columns: ["email"], message: "já existe" } }),
    );
    const f = useRowForm(() => table, () => null);
    f.reset();
    f.form.value.email = { mode: "value", text: "a@x.com", bool: false };
    expect(await f.submit()).toBe(false);
    expect(f.fieldErrors.value).toEqual({ email: "já existe" });
    expect(f.saving.value).toBe(false);
  });

  it("client-side validation errors never reach the backend", async () => {
    const t2 = { ...table, columns: [...table.columns, { name: "age", type: "INTEGER", notNull: false, default: null, pk: 0, generated: false }] };
    const f = useRowForm(() => t2, () => null);
    f.reset();
    f.form.value.age = { mode: "value", text: "abc", bool: false };
    expect(await f.submit()).toBe(false);
    expect(api.insertRow).not.toHaveBeenCalled();
    expect(f.fieldErrors.value.age).toBeTruthy();
  });

  it("remove deletes by key and reports general errors", async () => {
    (api.deleteRow as any).mockRejectedValue(
      Object.assign(new Error("x"), { cause: { type: "constraint", kind: "FOREIGN KEY", columns: [], message: "há dependentes" } }),
    );
    const f = useRowForm(() => table, () => row);
    f.reset();
    expect(await f.remove()).toBe(false);
    expect(api.deleteRow).toHaveBeenCalledWith("users", { id: 1 });
    expect(f.generalError.value).toBe("há dependentes");
  });
});

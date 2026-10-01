import { describe, expect, it } from "vitest";
import { inputKind } from "./columnKind";

describe("inputKind", () => {
  it.each([
    ["INTEGER", "integer"],
    ["BIGINT", "integer"],
    ["REAL", "number"],
    ["NUMERIC(10,2)", "number"],
    ["BOOLEAN", "boolean"],
    ["DATETIME", "datetime"],
    ["TIMESTAMP", "datetime"],
    ["DATE", "date"],
    ["TEXT", "textarea"],
    ["VARCHAR(80)", "text"],
    ["", "text"],
    ["BLOB", "text"],
  ])("%s -> %s", (decl, kind) => {
    expect(inputKind(decl)).toBe(kind);
  });
});

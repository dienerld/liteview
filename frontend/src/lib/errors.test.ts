import { describe, expect, it } from "vitest";
import { toAppError } from "./errors";

describe("toAppError", () => {
  it("reads a structured constraint cause (object)", () => {
    const e = Object.assign(new Error("x"), {
      cause: { type: "constraint", kind: "NOT NULL", columns: ["email"], message: "email é obrigatório" },
    });
    expect(toAppError(e)).toEqual({ message: "email é obrigatório", constraint: "NOT NULL", columns: ["email"] });
  });

  it("reads a structured constraint cause (JSON string)", () => {
    const cause = JSON.stringify({ type: "constraint", kind: "UNIQUE", columns: ["a", "b"], message: "dup" });
    expect(toAppError(Object.assign(new Error("x"), { cause })).columns).toEqual(["a", "b"]);
  });

  it("falls back to the error message", () => {
    expect(toAppError(new Error("boom"))).toEqual({ message: "boom", columns: [] });
    expect(toAppError("texto").message).toBe("texto");
  });

  it("ignores a cause that is not a constraint payload", () => {
    const e = Object.assign(new Error("boom"), { cause: "not json" });
    expect(toAppError(e)).toEqual({ message: "boom", columns: [] });
  });
});

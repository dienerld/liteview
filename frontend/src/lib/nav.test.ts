import { describe, expect, it } from "vitest";
import { incomingTarget, outgoingTarget, popTo, pushEntry, rootEntry } from "./nav";

describe("trail", () => {
  it("push and popTo", () => {
    let t = rootEntry("posts");
    t = pushEntry(t, { table: "users", where: [{ column: "id", value: 1 }], label: "users" });
    t = pushEntry(t, { table: "posts", where: [], label: "posts" });
    expect(t.map((e) => e.table)).toEqual(["posts", "users", "posts"]);
    expect(popTo(t, 0).map((e) => e.table)).toEqual(["posts"]);
    expect(popTo(t, 1).length).toBe(2);
  });
});

describe("targets", () => {
  it("outgoing FK filters the referenced table by the cell values", () => {
    const e = outgoingTarget({ table: "users", from: ["user_id"], to: ["id"], onUpdate: "", onDelete: "" }, { user_id: 7 });
    expect(e.table).toBe("users");
    expect(e.where).toEqual([{ column: "id", value: 7 }]);
  });

  it("outgoing composite FK keeps all column pairs", () => {
    const e = outgoingTarget({ table: "tags", from: ["a", "b"], to: ["x", "y"], onUpdate: "", onDelete: "" }, { a: 1, b: 2 });
    expect(e.where).toEqual([{ column: "x", value: 1 }, { column: "y", value: 2 }]);
  });

  it("incoming FK filters the referencing table by this row's values", () => {
    const e = incomingTarget({ table: "posts", from: ["user_id"], to: ["id"] }, { id: 3, email: "x" });
    expect(e.table).toBe("posts");
    expect(e.where).toEqual([{ column: "user_id", value: 3 }]);
  });

  it("labels mention the filter", () => {
    const e = incomingTarget({ table: "posts", from: ["user_id"], to: ["id"] }, { id: 3 });
    expect(e.label).toBe("posts (user_id = 3)");
  });
});

import * as ViewerService from "../../bindings/liteview/internal/viewer/service";
import type { Cond, DbInfo, Page, RefCount, RowQuery, TableInfo, TableSchema, Values } from "./types";

// Generated models are structurally identical to ./types (modulo nullable slices/maps);
// cast at this boundary only. This is the only file that imports the bindings.
const call = <T>(p: Promise<unknown>) => p as Promise<T>;

export const api = {
  initialDb: () => call<DbInfo | null>(ViewerService.InitialDB()),
  openDialog: () => call<DbInfo | null>(ViewerService.OpenDialog()),
  openPath: (path: string) => call<DbInfo>(ViewerService.OpenPath(path)),
  recents: () => call<string[]>(ViewerService.Recents()),
  forgetRecent: (path: string) => call<void>(ViewerService.ForgetRecent(path)),
  listTables: () => call<TableInfo[]>(ViewerService.ListTables()),
  getTable: (name: string) => call<TableSchema>(ViewerService.GetTable(name)),
  queryRows: (q: RowQuery) => call<Page>(ViewerService.QueryRows(q)),
  insertRow: (table: string, values: Values) => call<Values>(ViewerService.InsertRow(table, values)),
  updateRow: (table: string, key: Values, values: Values) => call<void>(ViewerService.UpdateRow(table, key, values)),
  deleteRow: (table: string, key: Values) => call<void>(ViewerService.DeleteRow(table, key)),
  references: (table: string, key: Values) => call<RefCount[]>(ViewerService.References(table, key)),
};

export type { Cond };

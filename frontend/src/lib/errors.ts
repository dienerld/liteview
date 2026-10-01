export interface AppError { message: string; constraint?: string; columns: string[] }

/** Normalizes anything thrown by a binding call; understands the backend's constraint payload in `cause`. */
export function toAppError(e: unknown): AppError {
  let cause: any = (e as any)?.cause
  if (typeof cause === "string") {
    try { cause = JSON.parse(cause) } catch { cause = undefined }
  }
  if (cause && cause.type === "constraint") {
    return { message: cause.message, constraint: cause.kind, columns: cause.columns ?? [] }
  }
  return { message: e instanceof Error ? e.message : String(e), columns: [] }
}

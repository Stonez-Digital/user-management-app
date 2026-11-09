export interface ApiErrorOptions {
  status?: number;
  code?: string;
  details?: unknown;
}

export class ApiError extends Error {
  status: number;
  code: string;
  details: unknown;
  constructor(message: string, options?: ApiErrorOptions);
}

// Endpoint payloads are currently heterogeneous; keep the migration-compatible
// response shape until each API route has an explicit response interface.
export function apiRequest(path: string, options?: RequestInit): Promise<any>;

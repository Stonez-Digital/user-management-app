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

export function apiRequest<T = unknown>(path: string, options?: RequestInit): Promise<T>;

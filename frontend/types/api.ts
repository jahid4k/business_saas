// types/api.ts
// Mirrors the Go backend's response envelope exactly.
// Every API call returns one of these shapes.

// ----------------------------------------------------------
// Response envelope
// ----------------------------------------------------------

export interface ApiResponse<T = unknown> {
  success: boolean;
  data?: T;
  message?: string;
  request_id?: string;
  error?: {
    code: string;
    message: string;
    fields?: Record<string, string>;
  };
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  limit: number;
  has_more: boolean;
}

// ----------------------------------------------------------
// API Error — thrown by lib/api.ts on non-2xx responses
// ----------------------------------------------------------

export class ApiError extends Error {
  code: string;
  status: number;
  fields?: Record<string, string>;
  requestId?: string;

  constructor(params: {
    code: string;
    message: string;
    status: number;
    fields?: Record<string, string>;
    requestId?: string;
  }) {
    super(params.message);
    this.name = "ApiError";
    this.code = params.code;
    this.status = params.status;
    this.fields = params.fields;
    this.requestId = params.requestId;
  }

  /** True for errors where the user must log in again */
  get isAuthError(): boolean {
    return this.status === 401;
  }

  /** True when a specific field caused the error */
  get hasFieldErrors(): boolean {
    return Boolean(this.fields && Object.keys(this.fields).length > 0);
  }
}

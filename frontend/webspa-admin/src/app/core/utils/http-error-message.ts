import { HttpErrorResponse } from '@angular/common/http';

function stringValue(value: unknown): string | null {
  if (typeof value !== 'string') {
    return null;
  }

  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : null;
}

export function extractHttpErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof HttpErrorResponse) {
    const payload = error.error;

    if (typeof payload === 'string') {
      return stringValue(payload) ?? fallback;
    }

    if (payload && typeof payload === 'object') {
      const errorObject = payload as Record<string, unknown>;
      return (
        stringValue(errorObject['message']) ??
        stringValue(errorObject['error']) ??
        fallback
      );
    }

    return stringValue(error.message) ?? fallback;
  }

  if (error && typeof error === 'object') {
    const errorObject = error as Record<string, unknown>;
    return stringValue(errorObject['message']) ?? stringValue(errorObject['error']) ?? fallback;
  }

  return stringValue(error) ?? fallback;
}

// Error codes per docs/dayline-app-api-contract.md "Conventions" section.
const ERROR_MESSAGES: Record<string, string> = {
  INVALID_MSISDN: "That doesn't look like a valid phone number.",
  OTP_INVALID: "That code isn't right. Check and try again.",
  OTP_EXPIRED: 'This code has expired. Request a new one.',
  RATE_LIMITED: 'Too many attempts. Please wait a bit before trying again.',
  UNAUTHORIZED: 'Your session expired. Please sign in again.',
  NOT_FOUND: "We couldn't find that.",
  CONFLICT: "That action can't be completed right now.",
  PROVIDER_ERROR: 'Something went wrong with your network provider. Try again shortly.',
  VALIDATION: 'Please check your details and try again.',
  NETWORK_ERROR: 'Could not reach Dayline. Check your connection and try again.',
  CONFIG_ERROR: 'Dayline is not configured correctly. Please try again later.',
  UNKNOWN: 'Something went wrong. Please try again.',
};

export class ApiError extends Error {
  code: string;
  status: number;

  constructor(code: string, message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.status = status;
  }
}

export function messageForCode(code: string, fallback?: string): string {
  return ERROR_MESSAGES[code] ?? fallback ?? ERROR_MESSAGES.UNKNOWN;
}

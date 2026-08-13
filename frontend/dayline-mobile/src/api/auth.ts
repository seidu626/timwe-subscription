import { apiRequest } from './client';
import type { OtpVerifyResponse } from './types';

export function requestOtp(msisdn: string, tenant: string): Promise<void> {
  return apiRequest<void>('/v1/app/auth/otp/request', {
    method: 'POST',
    body: { msisdn, tenant },
    auth: false,
  });
}

export function verifyOtp(msisdn: string, tenant: string, code: string): Promise<OtpVerifyResponse> {
  return apiRequest<OtpVerifyResponse>('/v1/app/auth/otp/verify', {
    method: 'POST',
    body: { msisdn, tenant, code },
    auth: false,
  });
}

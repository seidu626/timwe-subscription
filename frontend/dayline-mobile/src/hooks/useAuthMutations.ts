import { useMutation } from '@tanstack/react-query';

import { requestOtp, verifyOtp } from '@/api/auth';

export function useRequestOtp() {
  return useMutation({
    mutationFn: ({ msisdn, tenant }: { msisdn: string; tenant: string }) => requestOtp(msisdn, tenant),
  });
}

export function useVerifyOtp() {
  return useMutation({
    mutationFn: ({ msisdn, tenant, code }: { msisdn: string; tenant: string; code: string }) =>
      verifyOtp(msisdn, tenant, code),
  });
}

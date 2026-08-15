import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  cancelSubscription,
  confirmSubscription,
  createSubscription,
  listSubscriptions,
} from '@/api/subscriptions';
import { queryKeys } from './queryKeys';

export function useSubscriptions() {
  return useQuery({
    queryKey: queryKeys.subscriptions,
    queryFn: () => listSubscriptions(),
    select: (data) => data.subscriptions,
  });
}

export function useCreateSubscription() {
  return useMutation({
    mutationFn: ({ campaignSlug, tenant }: { campaignSlug: string; tenant: string }) =>
      createSubscription(campaignSlug, tenant),
  });
}

export function useConfirmSubscription() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ ref, pin }: { ref: string; pin?: string }) => confirmSubscription(ref, pin),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.subscriptions });
    },
  });
}

export function useCancelSubscription() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (ref: string) => cancelSubscription(ref),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.subscriptions });
    },
  });
}

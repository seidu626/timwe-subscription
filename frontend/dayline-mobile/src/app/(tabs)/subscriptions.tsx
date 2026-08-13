import { useState } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import { Link } from 'expo-router';
import { Alert, Pressable, StyleSheet, Text, View } from 'react-native';


import { Button } from '@/components/Button';
import { Card } from '@/components/Card';
import { EmptyState, ErrorState, LoadingState } from '@/components/AsyncState';
import { ScreenContainer } from '@/components/ScreenContainer';
import { useCancelSubscription, useSubscriptions } from '@/hooks/useSubscriptions';
import { colors, spacing, typography } from '@/theme/tokens';
import { formatBillingCycle, formatCurrency } from '@/utils/format';
import type { Subscription } from '@/api/types';

export default function SubscriptionsScreen() {
  const subscriptions = useSubscriptions();
  const cancelSubscription = useCancelSubscription();
  const [cancelingRef, setCancelingRef] = useState<string | null>(null);

  function confirmCancel(subscription: Subscription) {
    Alert.alert(
      `Cancel ${subscription.product_name}?`,
      "You'll lose access at the end of the current billing period.",
      [
        { text: 'Keep subscription', style: 'cancel' },
        {
          text: 'Cancel subscription',
          style: 'destructive',
          onPress: async () => {
            setCancelingRef(subscription.ref);
            try {
              await cancelSubscription.mutateAsync(subscription.ref);
            } catch (err) {
              Alert.alert('Could not cancel', err instanceof Error ? err.message : 'Please try again.');
            } finally {
              setCancelingRef(null);
            }
          },
        },
      ],
    );
  }

  return (
    <ScreenContainer scroll withTabBarClearance>
      <Text style={styles.pageTitle}>My Subscriptions</Text>
      <Text style={styles.pageSubtitle}>Manage your active plans and billing.</Text>

      {subscriptions.isPending ? <LoadingState label="Loading subscriptions…" /> : null}

      {subscriptions.isError ? (
        <ErrorState
          title="Couldn't load your subscriptions"
          message={subscriptions.error instanceof Error ? subscriptions.error.message : undefined}
          onRetry={() => subscriptions.refetch()}
        />
      ) : null}

      {subscriptions.isSuccess && subscriptions.data.length === 0 ? (
        <EmptyState
          icon="stars"
          title="No subscriptions yet"
          message="Browse Discover to find your first daily content subscription."
        />
      ) : null}

      <View style={styles.list}>
        {subscriptions.data?.map((subscription) => (
          <Card key={subscription.ref} style={styles.card}>
            <View style={styles.cardHeader}>
              <Text style={styles.productName}>{subscription.product_name}</Text>
              <StatusPill status={subscription.status} />
            </View>
            <Text style={styles.priceLine}>
              {subscription.next_charge_hint ? `${subscription.next_charge_hint} • ` : ''}
              {formatCurrency(subscription.price, subscription.currency)}
              {formatBillingCycle(subscription.billing_cycle)}
            </Text>
            {subscription.status === 'ACTIVE' ? (
              <Button
                label="Cancel subscription"
                variant="secondary"
                onPress={() => confirmCancel(subscription)}
                loading={cancelingRef === subscription.ref}
              />
            ) : null}
          </Card>
        ))}
      </View>

      <Link href="/(tabs)/discover" asChild>
        <Pressable accessibilityRole="button" style={styles.browseMore}>
          <MaterialIcons name="add" size={20} color={colors.primary} />
          <Text style={styles.browseMoreText}>Browse More Subscriptions</Text>
        </Pressable>
      </Link>

      <View style={styles.footnote}>
        <MaterialIcons name="info" size={16} color={colors.onSurfaceVariant} />
        <Text style={styles.footnoteText}>
          Charges appear on your mobile network airtime or wallet. Text STOP to cancel any subscription by SMS at any
          time.
        </Text>
      </View>
    </ScreenContainer>
  );
}

function StatusPill({ status }: { status: Subscription['status'] }) {
  const label = status === 'ACTIVE' ? 'Active' : status === 'PENDING' ? 'Pending' : 'Failed';
  return (
    <View style={[styles.statusPill, statusStyles[status]]}>
      <Text style={[styles.statusPillText, statusTextStyles[status]]}>{label}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  pageTitle: {
    ...typography.headlineLgMobile,
    color: colors.primary,
  },
  pageSubtitle: {
    ...typography.bodyMd,
    color: colors.onSurfaceVariant,
    marginTop: spacing.stackSm,
    marginBottom: spacing.sectionGap - spacing.stackLg,
  },
  list: {
    gap: spacing.stackMd,
  },
  card: {
    gap: spacing.stackMd,
  },
  cardHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  productName: {
    ...typography.headlineMd,
    fontSize: 20,
    color: colors.onSurface,
  },
  priceLine: {
    ...typography.bodyMd,
    color: colors.onSurfaceVariant,
  },
  statusPill: {
    borderRadius: 9999,
    paddingHorizontal: 10,
    paddingVertical: 4,
  },
  statusPillText: {
    ...typography.labelSm,
  },
  browseMore: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: spacing.stackSm,
    paddingVertical: spacing.stackMd,
    marginTop: spacing.stackLg,
  },
  browseMoreText: {
    ...typography.labelMd,
    color: colors.primary,
  },
  footnote: {
    flexDirection: 'row',
    gap: spacing.stackSm,
    marginTop: spacing.sectionGap - spacing.stackLg,
    paddingTop: spacing.stackLg,
    borderTopWidth: StyleSheet.hairlineWidth,
    borderTopColor: colors.outlineVariant,
  },
  footnoteText: {
    ...typography.labelSm,
    color: colors.onSurfaceVariant,
    flex: 1,
    lineHeight: 18,
  },
});

const statusStyles = {
  ACTIVE: { backgroundColor: 'rgba(15,110,82,0.14)' },
  PENDING: { backgroundColor: 'rgba(253,183,65,0.2)' },
  FAILED: { backgroundColor: 'rgba(186,26,26,0.12)' },
} as const;

const statusTextStyles = {
  ACTIVE: { color: colors.primary },
  PENDING: { color: colors.secondary },
  FAILED: { color: colors.error },
} as const;

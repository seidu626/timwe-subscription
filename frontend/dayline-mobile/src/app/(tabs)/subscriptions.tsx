import { useState } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import { Link } from 'expo-router';
import { Alert, Platform, Pressable, StyleSheet, Text, View } from 'react-native';


import { Button } from '@/components/Button';
import { Card } from '@/components/Card';
import { EmptyState, ErrorState, LoadingState } from '@/components/AsyncState';
import { ScreenContainer } from '@/components/ScreenContainer';
import { useCancelSubscription, useSubscriptions } from '@/hooks/useSubscriptions';
import { colors, spacing, typography } from '@/theme/tokens';
import { formatBillingCycle, formatCurrency, pluralize } from '@/utils/format';
import type { Subscription } from '@/api/types';

const PAST_STATUSES: Subscription['status'][] = ['CANCELLED', 'FAILED'];

export default function SubscriptionsScreen() {
  const subscriptions = useSubscriptions();
  const cancelSubscription = useCancelSubscription();
  const [cancelingRef, setCancelingRef] = useState<string | null>(null);
  const [pastExpanded, setPastExpanded] = useState(false);
  // Low-signal CANCELLED/FAILED records collapse into "Past subscriptions" so
  // the list stays scannable as history accumulates; ACTIVE/PENDING keep
  // rendering as cards exactly as before.
  const activeSubscriptions = subscriptions.data?.filter((s) => !PAST_STATUSES.includes(s.status)) ?? [];
  const pastSubscriptions = subscriptions.data?.filter((s) => PAST_STATUSES.includes(s.status)) ?? [];

  async function runCancel(subscription: Subscription) {
    setCancelingRef(subscription.ref);
    try {
      await cancelSubscription.mutateAsync(subscription.ref);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Please try again.';
      if (Platform.OS === 'web') {
        window.alert(`Could not cancel: ${message}`);
      } else {
        Alert.alert('Could not cancel', message);
      }
    } finally {
      setCancelingRef(null);
    }
  }

  function confirmCancel(subscription: Subscription) {
    const title = `Cancel ${subscription.product_name}?`;
    const body = "You'll lose access at the end of the current billing period.";
    // Alert.alert with buttons is a no-op on react-native-web, so the web
    // build must confirm through the browser dialog instead.
    if (Platform.OS === 'web') {
      if (window.confirm(`${title}\n${body}`)) {
        void runCancel(subscription);
      }
      return;
    }
    Alert.alert(title, body, [
      { text: 'Keep subscription', style: 'cancel' },
      {
        text: 'Cancel subscription',
        style: 'destructive',
        onPress: () => {
          void runCancel(subscription);
        },
      },
    ]);
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

      {activeSubscriptions.length > 0 ? (
        <View style={styles.list}>
          {activeSubscriptions.map((subscription) => (
            <SubscriptionCard
              key={subscription.ref}
              subscription={subscription}
              cancelingRef={cancelingRef}
              onCancel={confirmCancel}
            />
          ))}
        </View>
      ) : null}

      {pastSubscriptions.length > 0 ? (
        <View style={styles.pastGroup}>
          <Pressable
            onPress={() => setPastExpanded((value) => !value)}
            accessibilityRole="button"
            accessibilityState={{ expanded: pastExpanded }}
            style={styles.pastToggle}
          >
            <Text style={styles.pastToggleText}>{pluralize(pastSubscriptions.length, 'past subscription')}</Text>
            <MaterialIcons
              name={pastExpanded ? 'expand-less' : 'expand-more'}
              size={22}
              color={colors.onSurfaceVariant}
            />
          </Pressable>
          {pastExpanded ? (
            <View style={styles.list}>
              {pastSubscriptions.map((subscription) => (
                <SubscriptionCard
                  key={subscription.ref}
                  subscription={subscription}
                  cancelingRef={cancelingRef}
                  onCancel={confirmCancel}
                />
              ))}
            </View>
          ) : null}
        </View>
      ) : null}

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

function SubscriptionCard({
  subscription,
  cancelingRef,
  onCancel,
}: {
  subscription: Subscription;
  cancelingRef: string | null;
  onCancel: (subscription: Subscription) => void;
}) {
  return (
    <Card style={styles.card}>
      <View style={styles.cardHeader}>
        <Text style={styles.productName} numberOfLines={2} ellipsizeMode="tail">
          {subscription.product_name}
        </Text>
        <StatusPill status={subscription.status} />
      </View>
      {subscription.tenant_name ? (
        <Text style={styles.providerLine} numberOfLines={1} ellipsizeMode="tail">
          by {subscription.tenant_name}
        </Text>
      ) : null}
      <Text style={styles.priceLine}>
        {subscription.next_charge_hint ? `${subscription.next_charge_hint} • ` : ''}
        {formatCurrency(subscription.price, subscription.currency)}
        {formatBillingCycle(subscription.billing_cycle)}
      </Text>
      {subscription.status === 'ACTIVE' ? (
        <Button
          label="Cancel subscription"
          variant="secondary"
          onPress={() => onCancel(subscription)}
          loading={cancelingRef === subscription.ref}
        />
      ) : null}
    </Card>
  );
}

function StatusPill({ status }: { status: Subscription['status'] }) {
  const label =
    status === 'ACTIVE' ? 'Active' : status === 'PENDING' ? 'Pending' : status === 'CANCELLED' ? 'Cancelled' : 'Failed';
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
    alignItems: 'flex-start',
    gap: spacing.stackSm,
  },
  productName: {
    ...typography.headlineMd,
    fontSize: 20,
    color: colors.onSurface,
    flex: 1,
    minWidth: 0,
  },
  providerLine: {
    ...typography.labelSm,
    color: colors.onSurfaceVariant,
  },
  priceLine: {
    ...typography.bodyMd,
    color: colors.onSurfaceVariant,
  },
  statusPill: {
    borderRadius: 9999,
    paddingHorizontal: 10,
    paddingVertical: 4,
    flexShrink: 0,
  },
  pastGroup: {
    gap: spacing.stackMd,
    marginTop: spacing.stackLg,
  },
  pastToggle: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingVertical: spacing.stackSm,
  },
  pastToggleText: {
    ...typography.labelMd,
    color: colors.onSurfaceVariant,
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
  CANCELLED: { backgroundColor: 'rgba(73,69,79,0.1)' },
  FAILED: { backgroundColor: 'rgba(186,26,26,0.12)' },
} as const;

const statusTextStyles = {
  ACTIVE: { color: colors.primary },
  PENDING: { color: colors.secondary },
  CANCELLED: { color: colors.onSurfaceVariant },
  FAILED: { color: colors.error },
} as const;

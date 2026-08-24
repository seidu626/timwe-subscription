import { useState, useMemo } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import { Link } from 'expo-router';
import { Alert, Platform, Pressable, StyleSheet, Text, View } from 'react-native';

import { Button } from '@/components/Button';
import { Card } from '@/components/Card';
import { EmptyState, ErrorState, LoadingState } from '@/components/AsyncState';
import { ScreenContainer } from '@/components/ScreenContainer';
import { AnimatedPressable } from '@/components/AnimatedPressable';
import { useCancelSubscription, useSubscriptions } from '@/hooks/useSubscriptions';
import { radii, spacing, typography, type ThemeColors } from '@/theme/tokens';
import { useTheme } from '@/theme/ThemeContext';
import { formatBillingCycle, formatCurrency, formatProductName, pluralize } from '@/utils/format';
import type { Subscription } from '@/api/types';

const PAST_STATUSES: Subscription['status'][] = ['CANCELLED', 'FAILED'];

export default function SubscriptionsScreen() {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  const subscriptions = useSubscriptions();
  const cancelSubscription = useCancelSubscription();
  const [cancelingRef, setCancelingRef] = useState<string | null>(null);
  const [pastExpanded, setPastExpanded] = useState(false);
  
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
    const title = `Cancel ${formatProductName(subscription.product_name)}?`;
    const body = "You'll lose access at the end of the current billing period.";
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
    <ScreenContainer
      scroll
      withTabBarClearance
      refreshing={subscriptions.isRefetching}
      onRefresh={() => void subscriptions.refetch()}
    >
      <View style={styles.header}>
        <Text style={styles.eyebrow}>BILLING & MEMBERSHIPS</Text>
        <Text style={styles.pageTitle}>My Subscriptions</Text>
        <Text style={styles.pageSubtitle}>Manage your active channels and carrier billing.</Text>
      </View>

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
          title="No active subscriptions yet"
          message="Browse Discover to subscribe to daily briefings with your mobile airtime."
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
            <View style={styles.pastToggleLeft}>
              <MaterialIcons name="history" size={18} color={colors.onSurfaceVariant} />
              <Text style={styles.pastToggleText}>{pluralize(pastSubscriptions.length, 'past subscription')}</Text>
            </View>
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
        <AnimatedPressable accessibilityRole="button" style={styles.browseMore}>
          <MaterialIcons name="add-circle-outline" size={18} color={colors.primary} />
          <Text style={styles.browseMoreText}>Explore More Channels</Text>
        </AnimatedPressable>
      </Link>

      <View style={styles.footnote}>
        <MaterialIcons name="verified-user" size={16} color={colors.outline} />
        <Text style={styles.footnoteText}>
          Charges are billed directly to your MTN or Telecel airtime/wallet. You can text STOP to cancel any channel by SMS at any time.
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
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  return (
    <Card style={styles.card} padded={false}>
      <View style={styles.cardInner}>
        <View style={styles.cardHeader}>
          <View style={styles.cardHeaderLeft}>
            <Text style={styles.productName} numberOfLines={2} ellipsizeMode="tail">
              {formatProductName(subscription.product_name)}
            </Text>
            {subscription.tenant_name ? (
              <Text style={styles.providerLine} numberOfLines={1} ellipsizeMode="tail">
                Published by {subscription.tenant_name}
              </Text>
            ) : null}
          </View>
          <StatusPill status={subscription.status} />
        </View>

        <View style={styles.priceRow}>
          <Text style={styles.priceText}>
            {formatCurrency(subscription.price, subscription.currency)}
            <Text style={styles.priceCycle}> {formatBillingCycle(subscription.billing_cycle)}</Text>
          </Text>
          {subscription.next_charge_hint ? (
            <Text style={styles.nextCharge}>{subscription.next_charge_hint}</Text>
          ) : null}
        </View>

        {subscription.status === 'ACTIVE' || subscription.status === 'PENDING' ? (
          <Button
            label={subscription.status === 'PENDING' ? 'Cancel Request' : 'Cancel Subscription'}
            variant="secondary"
            onPress={() => onCancel(subscription)}
            loading={cancelingRef === subscription.ref}
            style={styles.cancelButton}
          />
        ) : null}
      </View>
    </Card>
  );
}

function StatusPill({ status }: { status: Subscription['status'] }) {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  const statusStyles = useMemo(() => createStatusStyles(colors), [colors]);
  const statusTextStyles = useMemo(() => createStatusTextStyles(colors), [colors]);
  const label =
    status === 'ACTIVE' ? 'Active' : status === 'PENDING' ? 'Pending' : status === 'CANCELLED' ? 'Cancelled' : 'Failed';
  return (
    <View style={[styles.statusPill, statusStyles[status]]}>
      <Text style={[styles.statusPillText, statusTextStyles[status]]}>{label}</Text>
    </View>
  );
}

const createStyles = (colors: ThemeColors) => StyleSheet.create({
  header: {
    marginBottom: spacing.stackLg,
  },
  eyebrow: {
    ...typography.labelSm,
    fontSize: 11,
    fontWeight: '700',
    letterSpacing: 0.8,
    color: colors.primary,
    marginBottom: 2,
  },
  pageTitle: {
    ...typography.headlineLgMobile,
    fontSize: 30,
    fontWeight: '800',
    letterSpacing: -0.5,
    color: colors.onSurface,
  },
  pageSubtitle: {
    ...typography.bodyMd,
    fontSize: 13,
    color: colors.onSurfaceVariant,
    marginTop: 4,
  },
  list: {
    gap: 12,
  },
  card: {
    backgroundColor: colors.surfaceContainerLowest,
    overflow: 'hidden',
  },
  cardInner: {
    padding: 16,
    gap: 12,
  },
  cardHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'flex-start',
    gap: spacing.stackSm,
  },
  cardHeaderLeft: {
    flex: 1,
    minWidth: 0,
    gap: 2,
  },
  productName: {
    ...typography.headlineMd,
    fontSize: 17,
    fontWeight: '700',
    color: colors.onSurface,
  },
  providerLine: {
    ...typography.labelSm,
    fontSize: 12,
    color: colors.onSurfaceVariant,
  },
  priceRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingVertical: 8,
    borderTopWidth: 1,
    borderTopColor: colors.cardBorder,
  },
  priceText: {
    ...typography.headlineMd,
    fontSize: 16,
    fontWeight: '700',
    color: colors.onSurface,
  },
  priceCycle: {
    ...typography.bodyMd,
    fontSize: 13,
    fontWeight: '400',
    color: colors.onSurfaceVariant,
  },
  nextCharge: {
    ...typography.labelSm,
    fontSize: 12,
    color: colors.outline,
  },
  cancelButton: {
    width: '100%',
  },
  statusPill: {
    borderRadius: radii.full,
    paddingHorizontal: 9,
    paddingVertical: 3,
    borderWidth: 1,
    flexShrink: 0,
  },
  statusPillText: {
    ...typography.labelSm,
    fontSize: 11,
    fontWeight: '700',
    letterSpacing: 0.4,
  },
  pastGroup: {
    gap: 12,
    marginTop: 20,
  },
  pastToggle: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingVertical: 10,
    paddingHorizontal: 14,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceContainerLow,
    borderWidth: 1,
    borderColor: colors.cardBorder,
  },
  pastToggleLeft: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  pastToggleText: {
    ...typography.labelMd,
    fontSize: 13,
    fontWeight: '600',
    color: colors.onSurfaceVariant,
  },
  browseMore: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 6,
    paddingVertical: 12,
    borderRadius: radii.md,
    borderWidth: 1,
    borderColor: colors.cardBorder,
    backgroundColor: colors.surfaceContainerLowest,
    marginTop: 20,
  },
  browseMoreText: {
    ...typography.labelMd,
    fontSize: 13,
    fontWeight: '600',
    color: colors.primary,
  },
  footnote: {
    flexDirection: 'row',
    gap: 8,
    marginTop: 24,
    paddingTop: 16,
    borderTopWidth: 1,
    borderTopColor: colors.cardBorder,
  },
  footnoteText: {
    ...typography.labelSm,
    fontSize: 11,
    lineHeight: 16,
    color: colors.outline,
    flex: 1,
  },
});

const createStatusStyles = (colors: ThemeColors) => ({
  ACTIVE: {
    backgroundColor: colors.primarySoft,
    borderColor: 'rgba(52, 211, 153, 0.3)',
  },
  PENDING: {
    backgroundColor: colors.secondaryContainer,
    borderColor: 'rgba(245, 158, 11, 0.3)',
  },
  CANCELLED: {
    backgroundColor: colors.surfaceContainerLow,
    borderColor: colors.cardBorder,
  },
  FAILED: {
    backgroundColor: colors.errorContainer,
    borderColor: 'rgba(239, 68, 68, 0.3)',
  },
} as const);

const createStatusTextStyles = (colors: ThemeColors) => ({
  ACTIVE: { color: colors.primary },
  PENDING: { color: colors.secondary },
  CANCELLED: { color: colors.onSurfaceVariant },
  FAILED: { color: colors.error },
} as const);

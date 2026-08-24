import { useState, useMemo } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import { router, useLocalSearchParams } from 'expo-router';
import { Pressable, StyleSheet, Text, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import * as Haptics from 'expo-haptics';

import { Button } from '@/components/Button';
import { Card } from '@/components/Card';
import { ErrorState, LoadingState } from '@/components/AsyncState';
import { useCatalogProduct } from '@/hooks/useCatalog';
import { useConfirmSubscription, useCreateSubscription } from '@/hooks/useSubscriptions';
import { radii, spacing, typography, type ThemeColors } from '@/theme/tokens';
import { useTheme } from '@/theme/ThemeContext';
import { formatBillingCycle, formatCurrency, formatProductName } from '@/utils/format';
import type { SubscriptionStatus } from '@/api/types';

type Step = 'review' | 'awaiting_pin' | 'error' | 'pending';

export default function ConfirmSubscriptionScreen() {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  const { slug } = useLocalSearchParams<{ slug: string }>();
  const { isPending, isError, product } = useCatalogProduct(slug);
  const createSubscription = useCreateSubscription();
  const confirmSubscription = useConfirmSubscription();
  const isSubmitting = createSubscription.isPending || confirmSubscription.isPending;

  const [step, setStep] = useState<Step>('review');
  const [subscriptionRef, setSubscriptionRef] = useState<string | null>(null);
  const [pin, setPin] = useState('');
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  function handleStatus(status: SubscriptionStatus) {
    if (status === 'ACTIVE') {
      Haptics.notificationAsync(Haptics.NotificationFeedbackType.Success);
      router.replace({ pathname: '/product/[slug]/success', params: { slug: slug! } });
    } else if (status === 'FAILED') {
      Haptics.notificationAsync(Haptics.NotificationFeedbackType.Error);
      setStep('error');
      setErrorMessage('The mobile network could not confirm this subscription. Please try again.');
    } else {
      setStep('pending');
    }
  }

  async function startSubscription() {
    if (!product) return;
    Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Medium);
    setErrorMessage(null);
    try {
      const result = await createSubscription.mutateAsync({
        campaignSlug: product.slug,
        tenant: product.tenant,
      });
      setSubscriptionRef(result.subscription_ref);
      if (result.next_action === 'SUBSCRIBED') {
        Haptics.notificationAsync(Haptics.NotificationFeedbackType.Success);
        router.replace({ pathname: '/product/[slug]/success', params: { slug: product.slug } });
      } else if (result.next_action === 'OTP') {
        setStep('awaiting_pin');
      } else {
        const confirmed = await confirmSubscription.mutateAsync({ ref: result.subscription_ref });
        handleStatus(confirmed.status);
      }
    } catch (err) {
      setStep('error');
      setErrorMessage(err instanceof Error ? err.message : 'Something went wrong. Please try again.');
    }
  }

  async function submitPin() {
    if (!subscriptionRef) return;
    Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Medium);
    setErrorMessage(null);
    try {
      const confirmed = await confirmSubscription.mutateAsync({ ref: subscriptionRef, pin });
      handleStatus(confirmed.status);
    } catch (err) {
      setStep('awaiting_pin');
      setErrorMessage(err instanceof Error ? err.message : 'Incorrect code. Please try again.');
    }
  }

  return (
    <SafeAreaView style={styles.root} edges={['bottom', 'left', 'right']}>
      <View style={styles.handle} />

      {isPending ? <LoadingState label="Preparing subscription…" /> : null}
      {isError || (!isPending && !product) ? (
        <ErrorState title="Couldn't load this channel" message="Please close and try again." />
      ) : null}

      {product ? (
        <View style={styles.content}>
          <Text style={styles.title}>Confirm Subscription</Text>

          {/* Product Summary Card */}
          <Card style={styles.summaryCard} padded={false}>
            <View style={styles.summaryCardInner}>
              <View style={styles.productLeft}>
                <View style={styles.productIcon}>
                  <MaterialIcons name="newspaper" size={24} color={colors.primary} />
                </View>
                <View style={styles.productTextGroup}>
                  <Text style={styles.productName} numberOfLines={2}>
                    {formatProductName(product.name)}
                  </Text>
                  <Text style={styles.productTagline} numberOfLines={1}>
                    {product.tagline}
                  </Text>
                </View>
              </View>

              <View style={styles.priceContainer}>
                <Text style={styles.price}>
                  {formatCurrency(product.price, product.currency)}
                </Text>
                <Text style={styles.priceCycle}>{formatBillingCycle(product.billing_cycle)}</Text>
              </View>
            </View>
          </Card>

          {/* Carrier Billing Trust Notice */}
          <View style={styles.disclosureBox}>
            <MaterialIcons name="verified-user" size={18} color={colors.primary} />
            <Text style={styles.disclosureText}>
              Billed directly to your MTN or Telecel airtime. No credit card needed. Auto-renews daily and you can cancel anytime.
            </Text>
          </View>

          {step === 'awaiting_pin' ? (
            <View style={styles.pinGroup}>
              <Text style={styles.pinLabel}>Enter the verification PIN sent via SMS</Text>
              <TextInput
                value={pin}
                onChangeText={setPin}
                keyboardType="number-pad"
                maxLength={6}
                placeholder="• • • •"
                placeholderTextColor={colors.outline}
                style={styles.pinInput}
                autoFocus
              />
            </View>
          ) : null}

          {step === 'pending' ? (
            <View style={styles.disclosureBox}>
              <MaterialIcons name="hourglass-top" size={18} color={colors.secondary} />
              <Text style={styles.disclosureText}>
                Your subscription is being activated by your carrier. Check the Subscriptions tab shortly.
              </Text>
            </View>
          ) : null}

          {errorMessage ? <Text style={styles.errorText}>{errorMessage}</Text> : null}

          {step === 'pending' ? (
            <Button label="Done" onPress={() => router.back()} style={styles.cta} />
          ) : step === 'awaiting_pin' ? (
            <Button
              label="Confirm PIN"
              onPress={submitPin}
              disabled={pin.length < 4}
              loading={isSubmitting}
              style={styles.cta}
            />
          ) : (
            <Button
              label="Authorize & Subscribe"
              onPress={startSubscription}
              loading={isSubmitting}
              icon={<MaterialIcons name="check" size={18} color={colors.onPrimary} />}
              style={styles.cta}
            />
          )}

          <Pressable onPress={() => router.back()} accessibilityRole="button" style={styles.cancelLink}>
            <Text style={styles.cancelLinkText}>Cancel and return</Text>
          </Pressable>
        </View>
      ) : null}
    </SafeAreaView>
  );
}

const createStyles = (colors: ThemeColors) => StyleSheet.create({
  root: {
    flex: 1,
    backgroundColor: colors.surfaceContainerLowest,
  },
  handle: {
    alignSelf: 'center',
    width: 44,
    height: 5,
    borderRadius: radii.full,
    backgroundColor: colors.outlineVariant,
    marginTop: 10,
    marginBottom: 4,
  },
  content: {
    paddingHorizontal: spacing.containerMargin,
    paddingTop: 16,
    paddingBottom: spacing.sectionGap,
    gap: 16,
  },
  title: {
    ...typography.headlineMd,
    fontSize: 22,
    fontWeight: '800',
    color: colors.onSurface,
  },
  summaryCard: {
    backgroundColor: colors.surfaceContainerLow,
    borderColor: colors.cardBorder,
    overflow: 'hidden',
  },
  summaryCardInner: {
    padding: 16,
    gap: 14,
  },
  productLeft: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 12,
  },
  productIcon: {
    width: 48,
    height: 48,
    borderRadius: radii.md,
    backgroundColor: colors.primarySoft,
    borderWidth: 1,
    borderColor: colors.cardBorder,
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: 0,
  },
  productTextGroup: {
    flex: 1,
    minWidth: 0,
    gap: 2,
  },
  productName: {
    ...typography.headlineMd,
    fontSize: 16,
    fontWeight: '700',
    color: colors.onSurface,
  },
  productTagline: {
    ...typography.bodyMd,
    fontSize: 13,
    color: colors.onSurfaceVariant,
  },
  priceContainer: {
    flexDirection: 'row',
    alignItems: 'baseline',
    gap: 4,
    paddingTop: 10,
    borderTopWidth: 1,
    borderTopColor: colors.cardBorder,
  },
  price: {
    ...typography.headlineMd,
    fontSize: 24,
    fontWeight: '800',
    color: colors.onSurface,
  },
  priceCycle: {
    ...typography.bodyMd,
    fontSize: 14,
    color: colors.onSurfaceVariant,
  },
  disclosureBox: {
    flexDirection: 'row',
    gap: 10,
    alignItems: 'center',
    backgroundColor: colors.surfaceContainerLow,
    borderWidth: 1,
    borderColor: colors.cardBorder,
    borderRadius: radii.md,
    padding: 14,
  },
  disclosureText: {
    ...typography.labelSm,
    color: colors.onSurfaceVariant,
    flex: 1,
    lineHeight: 18,
  },
  pinGroup: {
    gap: 8,
  },
  pinLabel: {
    ...typography.labelMd,
    fontSize: 13,
    fontWeight: '600',
    color: colors.onSurface,
  },
  pinInput: {
    borderWidth: 1,
    borderColor: colors.primary,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceContainerLowest,
    paddingHorizontal: 16,
    paddingVertical: 14,
    ...typography.headlineMd,
    fontSize: 20,
    textAlign: 'center',
    color: colors.onSurface,
    letterSpacing: 8,
  },
  errorText: {
    ...typography.labelSm,
    color: colors.error,
    fontWeight: '600',
  },
  cta: {
    width: '100%',
    marginTop: 4,
  },
  cancelLink: {
    alignItems: 'center',
    paddingVertical: 8,
  },
  cancelLinkText: {
    ...typography.labelMd,
    fontSize: 13,
    fontWeight: '600',
    color: colors.outline,
  },
});

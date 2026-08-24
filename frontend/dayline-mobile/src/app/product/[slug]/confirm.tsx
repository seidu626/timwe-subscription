import { useState , useMemo } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import { router, useLocalSearchParams } from 'expo-router';
import { Pressable, StyleSheet, Text, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Button } from '@/components/Button';
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
      router.replace({ pathname: '/product/[slug]/success', params: { slug: slug! } });
    } else if (status === 'FAILED') {
      setStep('error');
      setErrorMessage('The provider could not confirm this subscription. Please try again.');
    } else {
      setStep('pending');
    }
  }

  async function startSubscription() {
    if (!product) return;
    setErrorMessage(null);
    try {
      const result = await createSubscription.mutateAsync({
        campaignSlug: product.slug,
        tenant: product.tenant,
      });
      setSubscriptionRef(result.subscription_ref);
      if (result.next_action === 'SUBSCRIBED') {
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

      {isPending ? <LoadingState label="Loading…" /> : null}
      {isError || (!isPending && !product) ? (
        <ErrorState title="Couldn't load this product" message="Please close and try again." />
      ) : null}

      {product ? (
        <View style={styles.content}>
          <Text style={styles.title}>Confirm your subscription</Text>
          <Text style={styles.productName}>{formatProductName(product.name)}</Text>
          <Text style={styles.productTagline}>{product.tagline}</Text>

          <View style={styles.priceRow}>
            <Text style={styles.price}>
              {formatCurrency(product.price, product.currency)}
              <Text style={styles.priceCycle}> {formatBillingCycle(product.billing_cycle)}</Text>
            </Text>
          </View>

          <View style={styles.disclosureBox}>
            <MaterialIcons name="info-outline" size={18} color={colors.onSurfaceVariant} />
            <Text style={styles.disclosureText}>
              You will be billed {formatCurrency(product.price, product.currency)}{' '}
              {formatBillingCycle(product.billing_cycle)} via your mobile network. This subscription auto-renews and
              you can cancel anytime.
            </Text>
          </View>

          {step === 'awaiting_pin' ? (
            <View style={styles.pinGroup}>
              <Text style={styles.pinLabel}>Enter the confirmation PIN sent to your phone</Text>
              <TextInput
                value={pin}
                onChangeText={setPin}
                keyboardType="number-pad"
                maxLength={6}
                placeholder="PIN"
                placeholderTextColor={colors.onSurfaceVariant}
                style={styles.pinInput}
              />
            </View>
          ) : null}

          {step === 'pending' ? (
            <View style={styles.disclosureBox}>
              <MaterialIcons name="hourglass-top" size={18} color={colors.secondary} />
              <Text style={styles.disclosureText}>
                Your subscription is being processed. Check the Subscriptions tab shortly for the final status.
              </Text>
            </View>
          ) : null}

          {errorMessage ? <Text style={styles.errorText}>{errorMessage}</Text> : null}

          {step === 'pending' ? (
            <Button label="Done" onPress={() => router.back()} style={styles.cta} />
          ) : step === 'awaiting_pin' ? (
            <Button
              label="Confirm"
              onPress={submitPin}
              disabled={pin.length < 4}
              loading={isSubmitting}
              style={styles.cta}
            />
          ) : (
            <Button
              label="Confirm & Subscribe"
              onPress={startSubscription}
              loading={isSubmitting}
              style={styles.cta}
            />
          )}

          <Pressable onPress={() => router.back()} accessibilityRole="button" style={styles.cancelLink}>
            <Text style={styles.cancelLinkText}>Cancel</Text>
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
    width: 40,
    height: 4,
    borderRadius: radii.full,
    backgroundColor: colors.outlineVariant,
    marginTop: spacing.stackSm,
  },
  content: {
    paddingHorizontal: spacing.containerMargin,
    paddingTop: spacing.stackLg,
    paddingBottom: spacing.sectionGap,
    gap: spacing.stackMd,
  },
  title: {
    ...typography.headlineMd,
    fontSize: 20,
    color: colors.onSurface,
  },
  productName: {
    ...typography.headlineLgMobile,
    color: colors.primary,
  },
  productTagline: {
    ...typography.bodyMd,
    color: colors.onSurfaceVariant,
  },
  priceRow: {
    marginTop: spacing.stackSm,
  },
  price: {
    ...typography.headlineMd,
    fontSize: 22,
    color: colors.onSurface,
  },
  priceCycle: {
    ...typography.bodyMd,
    color: colors.onSurfaceVariant,
  },
  disclosureBox: {
    flexDirection: 'row',
    gap: spacing.stackSm,
    backgroundColor: colors.surfaceVariant,
    borderRadius: radii.default,
    padding: spacing.stackMd,
  },
  disclosureText: {
    ...typography.labelSm,
    color: colors.onSurfaceVariant,
    flex: 1,
    lineHeight: 18,
  },
  pinGroup: {
    gap: spacing.stackSm,
  },
  pinLabel: {
    ...typography.labelMd,
    color: colors.onSurface,
  },
  pinInput: {
    borderWidth: 1,
    borderColor: colors.outlineVariant,
    borderRadius: radii.default,
    paddingHorizontal: spacing.stackMd,
    paddingVertical: 12,
    ...typography.bodyLg,
    color: colors.onSurface,
    letterSpacing: 4,
  },
  errorText: {
    ...typography.labelSm,
    color: colors.error,
  },
  cta: {
    width: '100%',
    marginTop: spacing.stackSm,
  },
  cancelLink: {
    alignItems: 'center',
    paddingVertical: spacing.stackSm,
  },
  cancelLinkText: {
    ...typography.labelMd,
    color: colors.onSurfaceVariant,
  },
});

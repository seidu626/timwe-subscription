import { useState } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import { router } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { Button } from '@/components/Button';
import { PhoneInput } from '@/components/PhoneInput';
import { ScreenContainer } from '@/components/ScreenContainer';
import { useAuth } from '@/context/AuthContext';
import { useRequestOtp } from '@/hooks/useAuthMutations';
import { colors, spacing, typography } from '@/theme/tokens';
import { isCompleteGhanaLocalNumber, toE164Ghana } from '@/utils/phone';

export default function PhoneEntryScreen() {
  const { tenant } = useAuth();
  const [localNumber, setLocalNumber] = useState('');
  const [error, setError] = useState<string | null>(null);
  const requestOtp = useRequestOtp();

  const canSubmit = isCompleteGhanaLocalNumber(localNumber) && !requestOtp.isPending;

  async function handleSubmit() {
    setError(null);
    const msisdn = toE164Ghana(localNumber);
    try {
      await requestOtp.mutateAsync({ msisdn, tenant });
      router.push({ pathname: '/(auth)/otp-verify', params: { msisdn } });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong. Please try again.');
    }
  }

  return (
    <ScreenContainer>
      <Pressable onPress={() => router.back()} style={styles.back} accessibilityRole="button" accessibilityLabel="Go back">
        <MaterialIcons name="arrow-back" size={24} color={colors.primary} />
      </Pressable>

      <View style={styles.body}>
        <View style={styles.headerText}>
          <Text style={styles.title}>What&apos;s your number?</Text>
          <Text style={styles.subtitle}>We&apos;ll text you a 6-digit code to verify your account.</Text>
        </View>

        <View style={styles.form}>
          <PhoneInput value={localNumber} onChange={setLocalNumber} autoFocus />
          <View style={styles.helperRow}>
            <MaterialIcons name="check-circle" size={16} color={colors.primary} />
            <Text style={styles.helperText}>Works on AirtelTigo, MTN, Telecel</Text>
          </View>

          {error ? <Text style={styles.error}>{error}</Text> : null}

          <Button
            label="Send Code"
            onPress={handleSubmit}
            disabled={!canSubmit}
            loading={requestOtp.isPending}
            icon={<MaterialIcons name="arrow-forward" size={20} color={colors.onPrimary} />}
          />
        </View>

        <Text style={styles.terms}>
          By entering your number, you agree to our Terms of Service and acknowledge our Privacy Policy.
        </Text>
      </View>
    </ScreenContainer>
  );
}

const styles = StyleSheet.create({
  back: {
    width: 40,
    height: 40,
    borderRadius: 20,
    alignItems: 'center',
    justifyContent: 'center',
    marginTop: spacing.stackSm,
  },
  body: {
    flex: 1,
    justifyContent: 'center',
    gap: spacing.sectionGap,
    paddingBottom: spacing.sectionGap,
  },
  headerText: {
    gap: spacing.stackMd,
  },
  title: {
    ...typography.headlineLgMobile,
    color: colors.primary,
  },
  subtitle: {
    ...typography.bodyMd,
    color: colors.onSurfaceVariant,
  },
  form: {
    gap: spacing.stackLg,
  },
  helperRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.stackSm,
  },
  helperText: {
    ...typography.labelSm,
    color: colors.onSurfaceVariant,
  },
  error: {
    ...typography.labelSm,
    color: colors.error,
  },
  terms: {
    ...typography.labelSm,
    color: colors.outline,
    textAlign: 'center',
    lineHeight: 18,
  },
});

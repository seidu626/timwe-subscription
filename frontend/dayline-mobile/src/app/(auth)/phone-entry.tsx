import { useState, useMemo } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import { router } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { Button } from '@/components/Button';
import { PhoneInput } from '@/components/PhoneInput';
import { ScreenContainer } from '@/components/ScreenContainer';
import { useAuth } from '@/context/AuthContext';
import { useRequestOtp } from '@/hooks/useAuthMutations';
import { spacing, typography, type ThemeColors } from '@/theme/tokens';
import { useTheme } from '@/theme/ThemeContext';
import { isCompleteGhanaLocalNumber, toE164Ghana } from '@/utils/phone';

export default function PhoneEntryScreen() {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
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
        <View style={styles.iconCircle}>
          <MaterialIcons name="arrow-back" size={20} color={colors.onSurface} />
        </View>
      </Pressable>

      <View style={styles.body}>
        <View style={styles.headerText}>
          <Text style={styles.title}>What&apos;s your phone number?</Text>
          <Text style={styles.subtitle}>We&apos;ll send a 6-digit verification code to sign into your Dayline account.</Text>
        </View>

        <View style={styles.form}>
          <PhoneInput value={localNumber} onChange={setLocalNumber} autoFocus />
          <View style={styles.helperRow}>
            <MaterialIcons name="verified-user" size={16} color={colors.primary} />
            <Text style={styles.helperText}>Works on MTN, Telecel, and AT Ghana networks</Text>
          </View>

          {error ? <Text style={styles.error}>{error}</Text> : null}

          <Button
            label="Send Verification Code"
            onPress={handleSubmit}
            disabled={!canSubmit}
            loading={requestOtp.isPending}
            icon={<MaterialIcons name="arrow-forward" size={18} color={colors.onPrimary} />}
          />
        </View>

        <Text style={styles.terms}>
          By entering your number, you agree to Dayline Terms of Service and acknowledge our Privacy Policy.
        </Text>
      </View>
    </ScreenContainer>
  );
}

const createStyles = (colors: ThemeColors) => StyleSheet.create({
  back: {
    width: 40,
    height: 40,
    alignItems: 'center',
    justifyContent: 'center',
    marginTop: spacing.stackSm,
  },
  iconCircle: {
    width: 38,
    height: 38,
    borderRadius: 19,
    backgroundColor: colors.surfaceContainerLowest,
    borderWidth: 1,
    borderColor: colors.cardBorder,
    alignItems: 'center',
    justifyContent: 'center',
  },
  body: {
    flex: 1,
    justifyContent: 'center',
    gap: spacing.sectionGap,
    paddingBottom: spacing.sectionGap,
  },
  headerText: {
    gap: spacing.stackSm,
  },
  title: {
    ...typography.headlineLgMobile,
    fontSize: 28,
    fontWeight: '800',
    letterSpacing: -0.5,
    color: colors.onSurface,
  },
  subtitle: {
    ...typography.bodyMd,
    fontSize: 15,
    lineHeight: 22,
    color: colors.onSurfaceVariant,
  },
  form: {
    gap: spacing.stackLg,
  },
  helperRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
    paddingHorizontal: 4,
  },
  helperText: {
    ...typography.labelSm,
    fontSize: 12,
    color: colors.onSurfaceVariant,
  },
  error: {
    ...typography.labelSm,
    color: colors.error,
    fontWeight: '600',
  },
  terms: {
    ...typography.labelSm,
    fontSize: 12,
    color: colors.outline,
    textAlign: 'center',
    lineHeight: 18,
    paddingHorizontal: 8,
  },
});

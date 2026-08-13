import { useEffect, useState } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import { Link, router, useLocalSearchParams } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { Button } from '@/components/Button';
import { OtpInput } from '@/components/OtpInput';
import { ScreenContainer } from '@/components/ScreenContainer';
import { useAuth } from '@/context/AuthContext';
import { useRequestOtp, useVerifyOtp } from '@/hooks/useAuthMutations';
import { colors, spacing, typography } from '@/theme/tokens';
import { formatMsisdnForDisplay } from '@/utils/phone';

const RESEND_COOLDOWN_SECONDS = 30;

export default function OtpVerifyScreen() {
  const { msisdn } = useLocalSearchParams<{ msisdn: string }>();
  const { tenant, signIn } = useAuth();
  const [code, setCode] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [cooldown, setCooldown] = useState(RESEND_COOLDOWN_SECONDS);
  const verifyOtp = useVerifyOtp();
  const requestOtp = useRequestOtp();

  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = setInterval(() => setCooldown((value) => Math.max(0, value - 1)), 1000);
    return () => clearInterval(timer);
  }, [cooldown]);

  async function handleVerify() {
    if (!msisdn || code.length !== 6) return;
    setError(null);
    try {
      const result = await verifyOtp.mutateAsync({ msisdn, tenant, code });
      await signIn({ token: result.token, msisdn, expiresInSeconds: result.expires_in });
      router.replace('/(tabs)/today');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong. Please try again.');
    }
  }

  async function handleResend() {
    if (!msisdn || cooldown > 0) return;
    setError(null);
    try {
      await requestOtp.mutateAsync({ msisdn, tenant });
      setCooldown(RESEND_COOLDOWN_SECONDS);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong. Please try again.');
    }
  }

  const minutes = Math.floor(cooldown / 60);
  const seconds = String(cooldown % 60).padStart(2, '0');

  return (
    <ScreenContainer>
      <Pressable onPress={() => router.back()} style={styles.back} accessibilityRole="button" accessibilityLabel="Go back">
        <MaterialIcons name="arrow-back" size={24} color={colors.onSurfaceVariant} />
      </Pressable>

      <View style={styles.body}>
        <View style={styles.headerText}>
          <Text style={styles.title}>Enter the code</Text>
          <Text style={styles.subtitle}>
            Sent to {msisdn ? formatMsisdnForDisplay(msisdn) : ''} —{' '}
            <Link href="/(auth)/phone-entry" style={styles.editLink}>
              Edit
            </Link>
          </Text>
        </View>

        <OtpInput value={code} onChange={setCode} autoFocus />

        <View style={styles.countdownRow}>
          {cooldown > 0 ? (
            <Text style={styles.countdownText}>
              Resend in <Text style={styles.countdownValue}>{minutes}:{seconds}</Text>
            </Text>
          ) : (
            <Pressable onPress={handleResend} accessibilityRole="button">
              <Text style={styles.resendText}>{requestOtp.isPending ? 'Sending…' : 'Resend code'}</Text>
            </Pressable>
          )}
        </View>

        {error ? <Text style={styles.error}>{error}</Text> : null}
      </View>

      <Button
        label="Verify"
        onPress={handleVerify}
        disabled={code.length !== 6}
        loading={verifyOtp.isPending}
      />
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
    marginLeft: -8,
    marginTop: spacing.stackSm,
  },
  body: {
    flex: 1,
    gap: spacing.sectionGap,
    paddingTop: spacing.stackLg,
  },
  headerText: {
    gap: spacing.stackSm,
  },
  title: {
    ...typography.headlineLgMobile,
    color: colors.primary,
  },
  subtitle: {
    ...typography.bodyMd,
    color: colors.onSurfaceVariant,
  },
  editLink: {
    ...typography.labelMd,
    color: colors.primary,
  },
  countdownRow: {
    alignItems: 'center',
  },
  countdownText: {
    ...typography.bodyMd,
    color: colors.onSurfaceVariant,
  },
  countdownValue: {
    ...typography.labelMd,
    color: colors.onSurface,
  },
  resendText: {
    ...typography.labelMd,
    color: colors.primary,
  },
  error: {
    ...typography.labelSm,
    color: colors.error,
    textAlign: 'center',
  },
});

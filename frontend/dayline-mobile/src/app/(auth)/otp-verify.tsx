import { useCallback, useEffect, useRef, useState } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import * as Clipboard from 'expo-clipboard';
import { Link, router, useLocalSearchParams } from 'expo-router';
import { AppState, Pressable, StyleSheet, Text, View } from 'react-native';

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
  const [clipboardCode, setClipboardCode] = useState<string | null>(null);
  const lastSubmittedRef = useRef<string | null>(null);
  const verifyOtp = useVerifyOtp();
  const requestOtp = useRequestOtp();

  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = setInterval(() => setCooldown((value) => Math.max(0, value - 1)), 1000);
    return () => clearInterval(timer);
  }, [cooldown]);

  // Truecaller/Messages "Copy OTP" assist: whenever this screen is in the
  // foreground, surface a one-tap fill chip if the clipboard holds a
  // six-digit code.
  const checkClipboard = useCallback(async () => {
    try {
      const text = await Clipboard.getStringAsync();
      const match = (text ?? '').match(/(?<!\d)(\d{6})(?!\d)/);
      setClipboardCode(match ? match[1] : null);
    } catch {
      // Clipboard access can be denied in the background; ignore.
    }
  }, []);

  useEffect(() => {
    const initialCheck = setTimeout(checkClipboard, 0);
    const subscription = AppState.addEventListener('change', (state) => {
      if (state === 'active') checkClipboard();
    });
    return () => {
      clearTimeout(initialCheck);
      subscription.remove();
    };
  }, [checkClipboard]);

  const handleVerify = useCallback(
    async (candidate: string) => {
      if (!msisdn || candidate.length !== 6) return;
      setError(null);
      try {
        const result = await verifyOtp.mutateAsync({ msisdn, tenant, code: candidate });
        await signIn({ token: result.token, msisdn, expiresInSeconds: result.expires_in });
        router.replace('/(tabs)/today');
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Something went wrong. Please try again.');
        setCode('');
        setClipboardCode((current) => (current === candidate ? null : current));
      }
    },
    [msisdn, tenant, verifyOtp, signIn],
  );

  // Auto-submit as soon as six digits are present, once per distinct code.
  useEffect(() => {
    if (code.length === 6 && code !== lastSubmittedRef.current && !verifyOtp.isPending) {
      lastSubmittedRef.current = code;
      handleVerify(code);
    }
  }, [code, verifyOtp.isPending, handleVerify]);

  async function handleResend() {
    if (!msisdn || cooldown > 0) return;
    setError(null);
    try {
      await requestOtp.mutateAsync({ msisdn, tenant });
      setCooldown(RESEND_COOLDOWN_SECONDS);
      lastSubmittedRef.current = null;
      setCode('');
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

        {clipboardCode && clipboardCode !== code ? (
          <Pressable
            onPress={() => setCode(clipboardCode)}
            style={styles.clipboardChip}
            accessibilityRole="button"
            accessibilityLabel={`Use copied code ${clipboardCode}`}
          >
            <MaterialIcons name="content-paste" size={16} color={colors.onPrimaryFixedVariant} />
            <Text style={styles.clipboardChipText}>
              Use code {clipboardCode.slice(0, 3)} {clipboardCode.slice(3)}
            </Text>
          </Pressable>
        ) : null}

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
        onPress={() => handleVerify(code)}
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
  clipboardChip: {
    flexDirection: 'row',
    alignSelf: 'center',
    alignItems: 'center',
    gap: spacing.stackSm,
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderRadius: 999,
    backgroundColor: colors.primaryFixed,
  },
  clipboardChipText: {
    ...typography.labelMd,
    color: colors.onPrimaryFixedVariant,
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

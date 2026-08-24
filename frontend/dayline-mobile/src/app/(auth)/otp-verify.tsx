import { useCallback, useEffect, useRef, useState, useMemo } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import * as Clipboard from 'expo-clipboard';
import { Link, router, useLocalSearchParams } from 'expo-router';
import { AppState, Pressable, StyleSheet, Text, View } from 'react-native';

import { Button } from '@/components/Button';
import { OtpInput } from '@/components/OtpInput';
import { ScreenContainer } from '@/components/ScreenContainer';
import { useAuth } from '@/context/AuthContext';
import { useRequestOtp, useVerifyOtp } from '@/hooks/useAuthMutations';
import { radii, spacing, typography, type ThemeColors } from '@/theme/tokens';
import { useTheme } from '@/theme/ThemeContext';
import { formatMsisdnForDisplay } from '@/utils/phone';

const RESEND_COOLDOWN_SECONDS = 30;

export default function OtpVerifyScreen() {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
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

  const checkClipboard = useCallback(async () => {
    try {
      const text = await Clipboard.getStringAsync();
      const match = (text ?? '').match(/(?<!\d)(\d{6})(?!\d)/);
      setClipboardCode(match ? match[1] : null);
    } catch {
      // Clipboard access can be denied in background
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
        <View style={styles.iconCircle}>
          <MaterialIcons name="arrow-back" size={20} color={colors.onSurface} />
        </View>
      </Pressable>

      <View style={styles.body}>
        <View style={styles.headerText}>
          <Text style={styles.title}>Enter verification code</Text>
          <Text style={styles.subtitle}>
            Sent via SMS to {msisdn ? formatMsisdnForDisplay(msisdn) : ''} —{' '}
            <Link href="/(auth)/phone-entry" style={styles.editLink}>
              Change
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
            <MaterialIcons name="content-paste" size={16} color={colors.primary} />
            <Text style={styles.clipboardChipText}>
              Paste code {clipboardCode.slice(0, 3)} {clipboardCode.slice(3)}
            </Text>
          </Pressable>
        ) : null}

        <View style={styles.countdownRow}>
          {cooldown > 0 ? (
            <Text style={styles.countdownText}>
              Resend code in <Text style={styles.countdownValue}>{minutes}:{seconds}</Text>
            </Text>
          ) : (
            <Pressable onPress={handleResend} accessibilityRole="button">
              <Text style={styles.resendText}>{requestOtp.isPending ? 'Sending…' : 'Resend SMS Code'}</Text>
            </Pressable>
          )}
        </View>

        {error ? <Text style={styles.error}>{error}</Text> : null}
      </View>

      <Button
        label="Verify & Continue"
        onPress={() => handleVerify(code)}
        disabled={code.length !== 6}
        loading={verifyOtp.isPending}
        icon={<MaterialIcons name="check" size={18} color={colors.onPrimary} />}
      />
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
    gap: spacing.sectionGap,
    paddingTop: spacing.stackLg,
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
  editLink: {
    ...typography.labelMd,
    fontSize: 14,
    fontWeight: '700',
    color: colors.primary,
  },
  clipboardChip: {
    flexDirection: 'row',
    alignSelf: 'center',
    alignItems: 'center',
    gap: 8,
    paddingHorizontal: 16,
    paddingVertical: 10,
    borderRadius: radii.full,
    backgroundColor: colors.primarySoft,
    borderWidth: 1,
    borderColor: 'rgba(52, 211, 153, 0.3)',
  },
  clipboardChipText: {
    ...typography.labelMd,
    fontSize: 13,
    fontWeight: '600',
    color: colors.primary,
  },
  countdownRow: {
    alignItems: 'center',
  },
  countdownText: {
    ...typography.bodyMd,
    fontSize: 14,
    color: colors.onSurfaceVariant,
  },
  countdownValue: {
    ...typography.labelMd,
    fontSize: 14,
    fontWeight: '700',
    color: colors.primary,
  },
  resendText: {
    ...typography.labelMd,
    fontSize: 14,
    fontWeight: '700',
    color: colors.primary,
  },
  error: {
    ...typography.labelSm,
    color: colors.error,
    fontWeight: '600',
    textAlign: 'center',
  },
});

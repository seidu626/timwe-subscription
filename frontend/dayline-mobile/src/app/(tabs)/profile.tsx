import { useMemo } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import { Alert, Linking, Platform, Pressable, StyleSheet, Switch, Text, View } from 'react-native';
import * as Haptics from 'expo-haptics';

import { Card } from '@/components/Card';
import { Divider } from '@/components/Divider';
import { ScreenContainer } from '@/components/ScreenContainer';
import { AnimatedPressable } from '@/components/AnimatedPressable';
import { PRIVACY_URL, SUPPORT_URL, TERMS_URL } from '@/config';
import { useAuth } from '@/context/AuthContext';
import { useSettings } from '@/context/SettingsContext';
import { radii, spacing, typography, type ThemeColors } from '@/theme/tokens';
import { useTheme, type ThemePreference } from '@/theme/ThemeContext';
import { formatMsisdnForDisplay } from '@/utils/phone';

const APPEARANCE_OPTIONS: { value: ThemePreference; label: string; icon: keyof typeof MaterialIcons.glyphMap }[] = [
  { value: 'system', label: 'System', icon: 'settings-brightness' },
  { value: 'light', label: 'Light', icon: 'light-mode' },
  { value: 'dark', label: 'Dark', icon: 'dark-mode' },
];

export default function ProfileScreen() {
  const { colors, preference, setPreference } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  const { msisdn, signOut } = useAuth();
  const { dataSaverEnabled, setDataSaverEnabled } = useSettings();

  function handleSignOut() {
    const title = 'Sign out?';
    const body = 'You can sign back in anytime with your phone number.';
    if (Platform.OS === 'web') {
      if (window.confirm(`${title}\n${body}`)) {
        signOut();
      }
      return;
    }
    Alert.alert(title, body, [
      { text: 'Cancel', style: 'cancel' },
      { text: 'Sign out', style: 'destructive', onPress: () => signOut() },
    ]);
  }

  function handleAppearanceChange(pref: ThemePreference) {
    Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Light);
    setPreference(pref);
  }

  function openLink(url: string, label: string) {
    if (!url) {
      Alert.alert(label, 'This will be available soon.');
      return;
    }
    Linking.openURL(url).catch(() => Alert.alert('Could not open link', 'Please try again later.'));
  }

  return (
    <ScreenContainer scroll withTabBarClearance>
      <View style={styles.header}>
        <Text style={styles.eyebrow}>ACCOUNT & SETTINGS</Text>
        <Text style={styles.pageTitle}>Profile</Text>
      </View>

      {/* Identity Card */}
      <Card style={styles.identityCard} padded={false}>
        <View style={styles.identityInner}>
          <View style={styles.avatar}>
            <MaterialIcons name="person" size={28} color={colors.primary} />
          </View>
          <View style={styles.identityTextGroup}>
            <Text style={styles.msisdn}>{msisdn ? formatMsisdnForDisplay(msisdn) : 'Dayline Member'}</Text>
            <View style={styles.memberBadge}>
              <MaterialIcons name="verified" size={14} color={colors.primary} />
              <Text style={styles.memberBadgeText}>Verified Account</Text>
            </View>
          </View>
        </View>
      </Card>

      {/* Settings Options Card */}
      <Text style={styles.sectionHeader}>Preferences</Text>
      <Card padded={false} style={styles.settingsCard}>
        <View style={styles.row}>
          <View style={styles.iconCircle}>
            <MaterialIcons name="data-usage" size={20} color={colors.primary} />
          </View>
          <View style={styles.rowTextGroup}>
            <Text style={styles.rowLabel}>Data Saver</Text>
            <Text style={styles.rowHint}>Skip loading high-res artwork images</Text>
          </View>
          <Switch
            value={dataSaverEnabled}
            onValueChange={setDataSaverEnabled}
            trackColor={{ true: colors.primary, false: colors.surfaceContainerLow }}
            thumbColor={colors.onPrimary}
          />
        </View>

        <Divider />

        <View style={styles.row}>
          <View style={styles.iconCircle}>
            <MaterialIcons name="palette" size={20} color={colors.secondary} />
          </View>
          <View style={styles.rowTextGroup}>
            <Text style={styles.rowLabel}>Theme Appearance</Text>
            <View style={styles.appearanceRow}>
              {APPEARANCE_OPTIONS.map((option) => {
                const active = option.value === preference;
                return (
                  <Pressable
                    key={option.value}
                    onPress={() => handleAppearanceChange(option.value)}
                    accessibilityRole="button"
                    accessibilityState={{ selected: active }}
                    style={[styles.appearancePill, active && styles.appearancePillActive]}
                  >
                    <MaterialIcons 
                      name={option.icon} 
                      size={14} 
                      color={active ? colors.onPrimary : colors.onSurfaceVariant} 
                    />
                    <Text style={[styles.appearancePillText, active && styles.appearancePillTextActive]}>
                      {option.label}
                    </Text>
                  </Pressable>
                );
              })}
            </View>
          </View>
        </View>
      </Card>

      <Text style={styles.sectionHeader}>Support & Legal</Text>
      <Card padded={false} style={styles.settingsCard}>
        <Pressable
          style={styles.row}
          accessibilityRole="button"
          onPress={() => openLink(SUPPORT_URL, 'Help and support')}
        >
          <View style={styles.iconCircle}>
            <MaterialIcons name="help-outline" size={20} color={colors.primary} />
          </View>
          <Text style={[styles.rowLabel, styles.rowLabelFlex]}>Help & Support</Text>
          <MaterialIcons name="chevron-right" size={20} color={colors.outline} />
        </Pressable>

        <Divider />

        <Pressable
          style={styles.row}
          accessibilityRole="button"
          onPress={() => openLink(TERMS_URL || PRIVACY_URL, 'Terms and privacy')}
        >
          <View style={styles.iconCircle}>
            <MaterialIcons name="security" size={20} color={colors.primary} />
          </View>
          <Text style={[styles.rowLabel, styles.rowLabelFlex]}>Terms & Privacy Policy</Text>
          <MaterialIcons name="chevron-right" size={20} color={colors.outline} />
        </Pressable>
      </Card>

      <AnimatedPressable style={styles.signOutRow} accessibilityRole="button" onPress={handleSignOut}>
        <MaterialIcons name="logout" size={20} color={colors.error} />
        <Text style={styles.signOutText}>Sign Out of Dayline</Text>
      </AnimatedPressable>
    </ScreenContainer>
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
  identityCard: {
    marginBottom: 24,
    backgroundColor: colors.surfaceContainerLowest,
    overflow: 'hidden',
  },
  identityInner: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 16,
    padding: 18,
  },
  avatar: {
    width: 56,
    height: 56,
    borderRadius: radii.md,
    backgroundColor: colors.primarySoft,
    borderWidth: 1,
    borderColor: 'rgba(52, 211, 153, 0.3)',
    alignItems: 'center',
    justifyContent: 'center',
  },
  identityTextGroup: {
    flex: 1,
    gap: 4,
  },
  msisdn: {
    ...typography.headlineMd,
    fontSize: 19,
    fontWeight: '800',
    color: colors.onSurface,
  },
  memberBadge: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 4,
  },
  memberBadgeText: {
    ...typography.labelSm,
    fontSize: 12,
    fontWeight: '600',
    color: colors.primary,
  },
  sectionHeader: {
    ...typography.labelSm,
    fontSize: 12,
    fontWeight: '700',
    letterSpacing: 0.6,
    color: colors.outline,
    marginBottom: 10,
    marginLeft: 4,
  },
  settingsCard: {
    marginBottom: 24,
    backgroundColor: colors.surfaceContainerLowest,
    overflow: 'hidden',
  },
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 14,
    paddingHorizontal: 16,
    paddingVertical: 16,
  },
  iconCircle: {
    width: 36,
    height: 36,
    borderRadius: 18,
    backgroundColor: colors.surfaceContainerLow,
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: 0,
  },
  rowTextGroup: {
    flex: 1,
    gap: 2,
  },
  rowLabel: {
    ...typography.bodyMd,
    fontSize: 15,
    fontWeight: '600',
    color: colors.onSurface,
  },
  rowLabelFlex: {
    flex: 1,
  },
  rowHint: {
    ...typography.labelSm,
    fontSize: 12,
    color: colors.onSurfaceVariant,
  },
  appearanceRow: {
    flexDirection: 'row',
    gap: 8,
    marginTop: 8,
  },
  appearancePill: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
    paddingHorizontal: 12,
    paddingVertical: 7,
    borderRadius: radii.full,
    borderWidth: 1,
    borderColor: colors.cardBorder,
    backgroundColor: colors.surfaceContainerLow,
  },
  appearancePillActive: {
    backgroundColor: colors.primary,
    borderColor: colors.primary,
  },
  appearancePillText: {
    ...typography.labelSm,
    fontSize: 12,
    fontWeight: '600',
    color: colors.onSurfaceVariant,
  },
  appearancePillTextActive: {
    color: colors.onPrimary,
    fontWeight: '700',
  },
  signOutRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 8,
    paddingVertical: 14,
    borderRadius: radii.md,
    borderWidth: 1,
    borderColor: 'rgba(239, 68, 68, 0.25)',
    backgroundColor: 'rgba(239, 68, 68, 0.08)',
    marginTop: 8,
  },
  signOutText: {
    ...typography.labelMd,
    fontSize: 14,
    fontWeight: '700',
    color: colors.error,
  },
});

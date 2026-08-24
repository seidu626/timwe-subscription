import { useMemo } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import { Alert, Linking, Platform, Pressable, StyleSheet, Switch, Text, View } from 'react-native';

import { Card } from '@/components/Card';
import { Divider } from '@/components/Divider';
import { ScreenContainer } from '@/components/ScreenContainer';
import { PRIVACY_URL, SUPPORT_URL, TERMS_URL } from '@/config';
import { useAuth } from '@/context/AuthContext';
import { useSettings } from '@/context/SettingsContext';
import { radii, spacing, typography, type ThemeColors } from '@/theme/tokens';
import { useTheme, type ThemePreference } from '@/theme/ThemeContext';
import { formatMsisdnForDisplay } from '@/utils/phone';

const APPEARANCE_OPTIONS: { value: ThemePreference; label: string }[] = [
  { value: 'system', label: 'System' },
  { value: 'light', label: 'Light' },
  { value: 'dark', label: 'Dark' },
];

export default function ProfileScreen() {
  const { colors, preference, setPreference } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  const { msisdn, signOut } = useAuth();
  const { dataSaverEnabled, setDataSaverEnabled } = useSettings();

  function handleSignOut() {
    const title = 'Sign out?';
    const body = 'You can sign back in anytime with your phone number.';
    // Alert.alert with buttons is a no-op on react-native-web, so the web
    // build must confirm through the browser dialog instead.
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

  function openLink(url: string, label: string) {
    if (!url) {
      Alert.alert(label, 'This will be available soon.');
      return;
    }
    Linking.openURL(url).catch(() => Alert.alert('Could not open link', 'Please try again later.'));
  }

  return (
    <ScreenContainer scroll withTabBarClearance>
      <Text style={styles.pageTitle}>Profile</Text>

      <View style={styles.identity}>
        <View style={styles.avatar}>
          <MaterialIcons name="person" size={32} color={colors.primary} />
        </View>
        <View>
          <Text style={styles.msisdn}>{msisdn ? formatMsisdnForDisplay(msisdn) : ''}</Text>
          <Text style={styles.identitySubtitle}>Dayline account</Text>
        </View>
      </View>

      <Card padded={false} style={styles.settingsCard}>
        <View style={styles.row}>
          <MaterialIcons name="data-usage" size={22} color={colors.onSurfaceVariant} />
          <View style={styles.rowTextGroup}>
            <Text style={styles.rowLabel}>Data saver</Text>
            <Text style={styles.rowHint}>Skip loading product artwork images</Text>
          </View>
          <Switch
            value={dataSaverEnabled}
            onValueChange={setDataSaverEnabled}
            trackColor={{ true: colors.primary, false: colors.surfaceVariant }}
          />
        </View>
        <Divider />
        <View style={styles.row}>
          <MaterialIcons name="dark-mode" size={22} color={colors.onSurfaceVariant} />
          <View style={styles.rowTextGroup}>
            <Text style={styles.rowLabel}>Appearance</Text>
            <View style={styles.appearanceRow}>
              {APPEARANCE_OPTIONS.map((option) => {
                const active = option.value === preference;
                return (
                  <Pressable
                    key={option.value}
                    onPress={() => setPreference(option.value)}
                    accessibilityRole="button"
                    accessibilityState={{ selected: active }}
                    style={[styles.appearancePill, active && styles.appearancePillActive]}
                  >
                    <Text style={[styles.appearancePillText, active && styles.appearancePillTextActive]}>
                      {option.label}
                    </Text>
                  </Pressable>
                );
              })}
            </View>
          </View>
        </View>
        <Divider />
        <Pressable
          style={styles.row}
          accessibilityRole="button"
          onPress={() => openLink(SUPPORT_URL, 'Help and support')}
        >
          <MaterialIcons name="help-outline" size={22} color={colors.onSurfaceVariant} />
          <Text style={[styles.rowLabel, styles.rowLabelFlex]}>Help and support</Text>
          <MaterialIcons name="chevron-right" size={20} color={colors.onSurfaceVariant} />
        </Pressable>
        <Divider />
        <Pressable
          style={styles.row}
          accessibilityRole="button"
          onPress={() => openLink(TERMS_URL || PRIVACY_URL, 'Terms and privacy')}
        >
          <MaterialIcons name="privacy-tip" size={22} color={colors.onSurfaceVariant} />
          <Text style={[styles.rowLabel, styles.rowLabelFlex]}>Terms and privacy</Text>
          <MaterialIcons name="chevron-right" size={20} color={colors.onSurfaceVariant} />
        </Pressable>
      </Card>

      <Pressable style={styles.signOutRow} accessibilityRole="button" onPress={handleSignOut}>
        <MaterialIcons name="logout" size={22} color={colors.error} />
        <Text style={styles.signOutText}>Sign out</Text>
      </Pressable>
    </ScreenContainer>
  );
}

const createStyles = (colors: ThemeColors) => StyleSheet.create({
  pageTitle: {
    ...typography.headlineLgMobile,
    color: colors.primary,
    marginBottom: spacing.sectionGap - spacing.stackLg,
  },
  identity: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.stackMd,
    marginBottom: spacing.sectionGap - spacing.stackLg,
  },
  avatar: {
    width: 64,
    height: 64,
    borderRadius: 32,
    backgroundColor: colors.surfaceContainerHigh,
    alignItems: 'center',
    justifyContent: 'center',
  },
  msisdn: {
    ...typography.headlineMd,
    fontSize: 20,
    color: colors.onSurface,
  },
  identitySubtitle: {
    ...typography.bodyMd,
    color: colors.onSurfaceVariant,
  },
  settingsCard: {
    marginBottom: spacing.sectionGap - spacing.stackLg,
  },
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.stackMd,
    paddingHorizontal: spacing.containerMargin,
    paddingVertical: spacing.stackLg,
  },
  rowTextGroup: {
    flex: 1,
    gap: 2,
  },
  rowLabel: {
    ...typography.bodyMd,
    color: colors.onSurface,
  },
  rowLabelFlex: {
    flex: 1,
  },
  rowHint: {
    ...typography.labelSm,
    color: colors.onSurfaceVariant,
  },
  appearanceRow: {
    flexDirection: 'row',
    gap: spacing.stackSm,
    marginTop: spacing.stackSm,
  },
  appearancePill: {
    paddingHorizontal: spacing.stackMd,
    paddingVertical: 6,
    borderRadius: radii.full,
    borderWidth: 1,
    borderColor: colors.outlineVariant,
  },
  appearancePillActive: {
    backgroundColor: colors.primary,
    borderColor: colors.primary,
  },
  appearancePillText: {
    ...typography.labelSm,
    color: colors.onSurfaceVariant,
  },
  appearancePillTextActive: {
    color: colors.onPrimary,
  },
  signOutRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: spacing.stackSm,
    paddingVertical: spacing.stackLg,
  },
  signOutText: {
    ...typography.labelMd,
    color: colors.error,
  },
});

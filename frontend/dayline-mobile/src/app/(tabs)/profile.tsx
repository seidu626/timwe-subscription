import { MaterialIcons } from '@expo/vector-icons';
import { Alert, Linking, Pressable, StyleSheet, Switch, Text, View } from 'react-native';

import { Card } from '@/components/Card';
import { Divider } from '@/components/Divider';
import { ScreenContainer } from '@/components/ScreenContainer';
import { PRIVACY_URL, SUPPORT_URL, TERMS_URL } from '@/config';
import { useAuth } from '@/context/AuthContext';
import { useSettings } from '@/context/SettingsContext';
import { colors, spacing, typography } from '@/theme/tokens';
import { formatMsisdnForDisplay } from '@/utils/phone';

export default function ProfileScreen() {
  const { msisdn, signOut } = useAuth();
  const { dataSaverEnabled, setDataSaverEnabled } = useSettings();

  function handleSignOut() {
    Alert.alert('Sign out?', 'You can sign back in anytime with your phone number.', [
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

const styles = StyleSheet.create({
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

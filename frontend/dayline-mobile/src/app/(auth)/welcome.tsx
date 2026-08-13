import { MaterialIcons } from '@expo/vector-icons';
import { Link } from 'expo-router';
import { StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Button } from '@/components/Button';
import { colors, spacing, typography } from '@/theme/tokens';

export default function WelcomeScreen() {
  return (
    <View style={styles.root}>
      <SafeAreaView style={styles.safeArea}>
        <View style={styles.header}>
          <Text style={styles.title}>Dayline</Text>
          <Text style={styles.tagline}>Fresh content. Every day.</Text>
        </View>

        <View style={styles.illustration}>
          <View style={styles.sunGlow}>
            <MaterialIcons name="wb-sunny" size={120} color={colors.onPrimary} />
          </View>
        </View>

        <View style={styles.actions}>
          <Link href="/(auth)/phone-entry" asChild>
            <Button
              label="Get Started"
              variant="inverse"
              onPress={() => undefined}
              icon={<MaterialIcons name="arrow-forward" size={20} color={colors.primaryContainer} />}
            />
          </Link>
          <Text style={styles.terms}>By continuing, you agree to our Terms</Text>
        </View>
      </SafeAreaView>
    </View>
  );
}

const styles = StyleSheet.create({
  root: {
    flex: 1,
    backgroundColor: colors.primaryContainer,
  },
  safeArea: {
    flex: 1,
    paddingHorizontal: spacing.containerMargin,
    justifyContent: 'space-between',
    paddingVertical: spacing.sectionGap,
  },
  header: {
    alignItems: 'center',
    gap: spacing.stackMd,
  },
  title: {
    ...typography.displayLg,
    color: colors.onPrimary,
  },
  tagline: {
    ...typography.bodyLg,
    color: colors.onPrimaryContainer,
    textAlign: 'center',
    maxWidth: 280,
  },
  illustration: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
  },
  sunGlow: {
    width: 220,
    height: 220,
    borderRadius: 110,
    backgroundColor: 'rgba(255,255,255,0.12)',
    alignItems: 'center',
    justifyContent: 'center',
  },
  actions: {
    gap: spacing.stackLg,
  },
  cta: {
    backgroundColor: colors.onPrimary,
  },
  terms: {
    ...typography.labelSm,
    color: 'rgba(255,255,255,0.7)',
    textAlign: 'center',
  },
});

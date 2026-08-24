import { useMemo } from 'react';
import { RefreshControl, ScrollView, StyleSheet, View, type StyleProp, type ViewStyle } from 'react-native';
import { SafeAreaView, useSafeAreaInsets } from 'react-native-safe-area-context';

import { spacing, type ThemeColors } from '@/theme/tokens';
import { useTheme } from '@/theme/ThemeContext';
import { TAB_BAR_CLEARANCE } from '@/theme/layout';

interface ScreenContainerProps {
  children: React.ReactNode;
  scroll?: boolean;
  /** Wire pull-to-refresh (scroll screens only). */
  refreshing?: boolean;
  onRefresh?: () => void;
  withTabBarClearance?: boolean;
  style?: StyleProp<ViewStyle>;
  contentContainerStyle?: StyleProp<ViewStyle>;
  edges?: ('top' | 'bottom' | 'left' | 'right')[];
}

export function ScreenContainer({
  children,
  scroll = false,
  refreshing = false,
  onRefresh,
  withTabBarClearance = false,
  style,
  contentContainerStyle,
  edges = ['top', 'left', 'right'],
}: ScreenContainerProps) {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  // The tab bar grows by the bottom safe-area inset (see (tabs)/_layout.tsx),
  // so clearance must grow with it or trailing content hides under the bar.
  const insets = useSafeAreaInsets();
  const bottomPad = withTabBarClearance ? TAB_BAR_CLEARANCE + insets.bottom : spacing.sectionGap;

  if (scroll) {
    return (
      <SafeAreaView style={[styles.root, style]} edges={edges}>
        <ScrollView
          contentContainerStyle={[
            styles.scrollContent,
            { paddingBottom: bottomPad },
            contentContainerStyle,
          ]}
          refreshControl={
            onRefresh ? <RefreshControl refreshing={refreshing} onRefresh={onRefresh} tintColor={colors.primary} /> : undefined
          }
        >
          {children}
        </ScrollView>
      </SafeAreaView>
    );
  }

  return (
    <SafeAreaView style={[styles.root, style]} edges={edges}>
      <View style={[styles.flexContent, { paddingBottom: bottomPad }, contentContainerStyle]}>{children}</View>
    </SafeAreaView>
  );
}

const createStyles = (colors: ThemeColors) => StyleSheet.create({
  root: {
    flex: 1,
    backgroundColor: colors.background,
  },
  scrollContent: {
    paddingHorizontal: spacing.containerMargin,
    paddingTop: spacing.stackLg,
  },
  flexContent: {
    flex: 1,
    paddingHorizontal: spacing.containerMargin,
  },
});

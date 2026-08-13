import { ScrollView, StyleSheet, View, type StyleProp, type ViewStyle } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { colors, spacing } from '@/theme/tokens';
import { TAB_BAR_CLEARANCE } from '@/theme/layout';

interface ScreenContainerProps {
  children: React.ReactNode;
  scroll?: boolean;
  withTabBarClearance?: boolean;
  style?: StyleProp<ViewStyle>;
  contentContainerStyle?: StyleProp<ViewStyle>;
  edges?: ('top' | 'bottom' | 'left' | 'right')[];
}

export function ScreenContainer({
  children,
  scroll = false,
  withTabBarClearance = false,
  style,
  contentContainerStyle,
  edges = ['top', 'left', 'right'],
}: ScreenContainerProps) {
  const bottomPad = withTabBarClearance ? TAB_BAR_CLEARANCE : spacing.sectionGap;

  if (scroll) {
    return (
      <SafeAreaView style={[styles.root, style]} edges={edges}>
        <ScrollView
          contentContainerStyle={[
            styles.scrollContent,
            { paddingBottom: bottomPad },
            contentContainerStyle,
          ]}
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

const styles = StyleSheet.create({
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

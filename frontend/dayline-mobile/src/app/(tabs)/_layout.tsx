import { MaterialIcons } from '@expo/vector-icons';
import { Tabs } from 'expo-router';
import { Platform, StyleSheet } from 'react-native';

import { usePushRegistration } from '@/hooks/usePushRegistration';
import { typography } from '@/theme/tokens';
import { useTheme } from '@/theme/ThemeContext';
import { TAB_BAR_HEIGHT } from '@/theme/layout';

export default function TabsLayout() {
  const { colors } = useTheme();
  usePushRegistration();

  return (
    <Tabs
      screenOptions={{
        headerShown: false,
        tabBarActiveTintColor: colors.primary,
        tabBarInactiveTintColor: colors.onSurfaceVariant,
        tabBarStyle: {
          height: TAB_BAR_HEIGHT + (Platform.OS === 'ios' ? 20 : 8),
          paddingTop: 8,
          paddingBottom: Platform.OS === 'ios' ? 20 : 8,
          backgroundColor: colors.surfaceContainerLowest,
          borderTopColor: colors.outlineVariant,
          borderTopWidth: StyleSheet.hairlineWidth,
        },
        // Fix-in-implementation: labels render at full width ("Subscriptions",
        // not "Subs") — the design references truncate this label on the
        // mobile bottom nav. minWidth plus no numberOfLines cap prevents clipping.
        tabBarLabelStyle: {
          fontFamily: typography.labelSm.fontFamily,
          fontSize: 11,
        },
        tabBarItemStyle: {
          minWidth: 84,
        },
      }}
    >
      <Tabs.Screen
        name="today"
        options={{
          title: 'Today',
          tabBarIcon: ({ color, size }) => <MaterialIcons name="calendar-today" size={size} color={color} />,
        }}
      />
      <Tabs.Screen
        name="discover"
        options={{
          title: 'Discover',
          tabBarIcon: ({ color, size }) => <MaterialIcons name="explore" size={size} color={color} />,
        }}
      />
      <Tabs.Screen
        name="subscriptions"
        options={{
          title: 'Subscriptions',
          tabBarIcon: ({ color, size }) => <MaterialIcons name="stars" size={size} color={color} />,
        }}
      />
      <Tabs.Screen
        name="profile"
        options={{
          title: 'Profile',
          tabBarIcon: ({ color, size }) => <MaterialIcons name="person" size={size} color={color} />,
        }}
      />
    </Tabs>
  );
}

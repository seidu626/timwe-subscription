import { forwardRef, useCallback, useState } from 'react';
import { Platform, Pressable, type PressableProps, type StyleProp, type View, type ViewStyle, type PressableStateCallbackType } from 'react-native';
import Animated, { useAnimatedStyle, useSharedValue, withSpring } from 'react-native-reanimated';
import * as Haptics from 'expo-haptics';

const ReanimatedPressable = Animated.createAnimatedComponent(Pressable);

interface AnimatedPressableProps extends Omit<PressableProps, 'style'> {
  children: React.ReactNode;
  hapticFeedback?: boolean;
  scaleTo?: number;
  style?: StyleProp<ViewStyle> | ((state: PressableStateCallbackType) => StyleProp<ViewStyle>);
}

export const AnimatedPressable = forwardRef<View, AnimatedPressableProps>(
  (
    { children, hapticFeedback = true, scaleTo = 0.97, style, onPressIn, onPressOut, ...props },
    ref
  ) => {
    const scale = useSharedValue(1);
    // createAnimatedComponent only reads object/array styles; a function style
    // is flattened away (dropping the caller's styles and the animated scale).
    // Resolve function styles here against a mirrored pressed state instead.
    const [pressed, setPressed] = useState(false);

    const animatedStyle = useAnimatedStyle(() => ({
      transform: [{ scale: scale.value }],
    }));

    const handlePressIn = useCallback(
      (e: any) => {
        setPressed(true);
        scale.value = withSpring(scaleTo, { stiffness: 400, damping: 25 });
        if (hapticFeedback) {
          Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Light);
        }
        onPressIn?.(e);
      },
      [hapticFeedback, onPressIn, scale, scaleTo]
    );

    const handlePressOut = useCallback(
      (e: any) => {
        setPressed(false);
        scale.value = withSpring(1, { stiffness: 400, damping: 25 });
        onPressOut?.(e);
        if (Platform.OS === 'web') {
          // Drop DOM focus from the pressed element so a Link-triggered
          // navigation doesn't leave focus trapped inside a screen that
          // React Navigation is about to mark aria-hidden (the source of
          // the "Blocked aria-hidden on an element because its descendant
          // retained focus" console warning).
          (e?.target as HTMLElement | undefined)?.blur?.();
        }
      },
      [onPressOut, scale]
    );

    const resolvedStyle = typeof style === 'function' ? style({ pressed } as PressableStateCallbackType) : style;

    return (
      <ReanimatedPressable
        {...props}
        ref={ref}
        onPressIn={handlePressIn}
        onPressOut={handlePressOut}
        style={[resolvedStyle, animatedStyle]}
      >
        {children}
      </ReanimatedPressable>
    );
  }
);

AnimatedPressable.displayName = 'AnimatedPressable';

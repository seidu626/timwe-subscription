import { forwardRef, useCallback, useState } from 'react';
import { Pressable, type PressableProps, type StyleProp, type ViewStyle, type PressableStateCallbackType } from 'react-native';
import Animated, { useAnimatedStyle, useSharedValue, withSpring } from 'react-native-reanimated';
import * as Haptics from 'expo-haptics';

const ReanimatedPressable = Animated.createAnimatedComponent(Pressable);

interface AnimatedPressableProps extends Omit<PressableProps, 'style'> {
  children: React.ReactNode;
  hapticFeedback?: boolean;
  scaleTo?: number;
  style?: StyleProp<ViewStyle> | ((state: PressableStateCallbackType) => StyleProp<ViewStyle>);
}

export const AnimatedPressable = forwardRef<ViewStyle, AnimatedPressableProps>(
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
      },
      [onPressOut, scale]
    );

    const resolvedStyle = typeof style === 'function' ? style({ pressed } as PressableStateCallbackType) : style;

    return (
      <ReanimatedPressable
        {...props}
        ref={ref as any}
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

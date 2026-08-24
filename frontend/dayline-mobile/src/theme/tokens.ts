/**
 * Design tokens ported from
 * apps/designs/stitch_dayline_subscription_content_app/dayline/DESIGN.md
 *
 * Fix-in-implementation deviations from the raw design file are called out
 * inline where they occur (see `focusRing` below).
 */

export const lightColors = {
  surface: '#f8faf6',
  surfaceDim: '#d8dbd3',
  surfaceBright: '#f8faf6',
  surfaceContainerLowest: '#ffffff',
  surfaceContainerLow: '#f2f5ec',
  surfaceContainer: '#ecefe7',
  surfaceContainerHigh: '#e6e9e1',
  surfaceContainerHighest: '#e0e4dc',
  onSurface: '#131915',
  onSurfaceVariant: '#3f4944',
  inverseSurface: '#2d312c',
  inverseOnSurface: '#eff2ea',
  outline: '#6f7a73',
  outlineVariant: '#d8ded9',
  surfaceTint: '#096b50',
  primary: '#00543d',
  onPrimary: '#ffffff',
  primaryContainer: '#0f6e52',
  onPrimaryContainer: '#9aedca',
  inversePrimary: '#84d7b4',
  secondary: '#b45309',
  onSecondary: '#ffffff',
  secondaryContainer: 'rgba(245, 158, 11, 0.12)',
  onSecondaryContainer: '#78350f',
  tertiary: '#484947',
  onTertiary: '#ffffff',
  tertiaryContainer: '#60615e',
  onTertiaryContainer: '#dddcd8',
  error: '#ba1a1a',
  onError: '#ffffff',
  errorContainer: '#ffdad6',
  onErrorContainer: '#93000a',
  primaryFixed: '#a0f3d0',
  primaryFixedDim: '#84d7b4',
  onPrimaryFixed: '#002116',
  onPrimaryFixedVariant: '#00513b',
  secondaryFixed: '#ffddb0',
  secondaryFixedDim: '#ffba47',
  onSecondaryFixed: '#291800',
  onSecondaryFixedVariant: '#614000',
  tertiaryFixed: '#e3e2df',
  tertiaryFixedDim: '#c7c7c3',
  onTertiaryFixed: '#1b1c1a',
  onTertiaryFixedVariant: '#464744',
  background: '#f8faf6',
  onBackground: '#131915',
  surfaceVariant: '#e0e4dc',
  cardBorder: 'rgba(0, 0, 0, 0.06)',
  primarySoft: 'rgba(0, 84, 61, 0.08)',
} as const;

export type ThemeColors = Readonly<Record<keyof typeof lightColors, string>>;

// Material 3 dark counterparts with elevated depth, crisp accents, and luminous borders.
export const darkColors: ThemeColors = {
  surface: '#0d110e',
  surfaceDim: '#0d110e',
  surfaceBright: '#2a322c',
  surfaceContainerLowest: '#141a15',
  surfaceContainerLow: '#182019',
  surfaceContainer: '#1d261f',
  surfaceContainerHigh: '#253027',
  surfaceContainerHighest: '#2e3a30',
  onSurface: '#e6ede8',
  onSurfaceVariant: '#a2b1a8',
  inverseSurface: '#e6ede8',
  inverseOnSurface: '#1a221b',
  outline: '#819187',
  outlineVariant: 'rgba(255, 255, 255, 0.1)',
  surfaceTint: '#34d399',
  primary: '#34d399',
  onPrimary: '#00382a',
  primaryContainer: '#044e38',
  onPrimaryContainer: '#a7f3d0',
  inversePrimary: '#00543d',
  secondary: '#fbbf24',
  onSecondary: '#451a03',
  secondaryContainer: 'rgba(245, 158, 11, 0.16)',
  onSecondaryContainer: '#fde68a',
  tertiary: '#cbd5e1',
  onTertiary: '#1e293b',
  tertiaryContainer: '#334155',
  onTertiaryContainer: '#e2e8f0',
  error: '#f87171',
  onError: '#450a0a',
  errorContainer: '#7f1d1d',
  onErrorContainer: '#fecaca',
  primaryFixed: '#a0f3d0',
  primaryFixedDim: '#84d7b4',
  onPrimaryFixed: '#002116',
  onPrimaryFixedVariant: '#00513b',
  secondaryFixed: '#ffddb0',
  secondaryFixedDim: '#ffba47',
  onSecondaryFixed: '#fef3c7',
  onSecondaryFixedVariant: '#fde68a',
  tertiaryFixed: '#e3e2df',
  tertiaryFixedDim: '#c7c7c3',
  onTertiaryFixed: '#1b1c1a',
  onTertiaryFixedVariant: '#464744',
  background: '#0d110e',
  onBackground: '#e6ede8',
  surfaceVariant: '#222b24',
  cardBorder: 'rgba(255, 255, 255, 0.08)',
  primarySoft: 'rgba(52, 211, 153, 0.14)',
};

// Fix-in-implementation: the design references (dayline_phone_entry,
// dayline_verification) render a neutral border-color focus state on
// inputs. Per the W1 brief this ships with an emerald focus ring (brand
// primary) instead, including on the OTP boxes.
// The ring color is the theme's primary; consumers take it from useTheme()
// so the ring tracks light/dark.
export const focusRing = {
  width: 2,
};

export const radii = {
  sm: 6,
  default: 10,
  md: 14,
  lg: 18,
  xl: 26,
  full: 9999,
} as const;

export const spacing = {
  base: 8,
  containerMargin: 20,
  gutter: 16,
  sectionGap: 36,
  stackSm: 4,
  stackMd: 12,
  stackLg: 20,
} as const;

export const shadow = {
  card: {
    shadowColor: '#000000',
    shadowOpacity: 0.12,
    shadowRadius: 16,
    shadowOffset: { width: 0, height: 4 },
    elevation: 3,
  },
  subtle: {
    shadowColor: '#000000',
    shadowOpacity: 0.06,
    shadowRadius: 8,
    shadowOffset: { width: 0, height: 2 },
    elevation: 1,
  },
  glow: {
    shadowColor: '#10b981',
    shadowOpacity: 0.25,
    shadowRadius: 20,
    shadowOffset: { width: 0, height: 4 },
    elevation: 5,
  },
} as const;

type FontWeightName = 'regular' | 'medium' | 'semiBold' | 'bold';

export const fontFamilies: Record<FontWeightName, string> = {
  regular: 'PlusJakartaSans_400Regular',
  medium: 'PlusJakartaSans_500Medium',
  semiBold: 'PlusJakartaSans_600SemiBold',
  bold: 'PlusJakartaSans_700Bold',
};

export const typography = {
  displayLg: {
    fontFamily: fontFamilies.bold,
    fontSize: 48,
    lineHeight: 56,
    letterSpacing: -0.96,
  },
  headlineLg: {
    fontFamily: fontFamilies.bold,
    fontSize: 32,
    lineHeight: 40,
    letterSpacing: -0.32,
  },
  headlineLgMobile: {
    fontFamily: fontFamilies.bold,
    fontSize: 28,
    lineHeight: 36,
    letterSpacing: 0,
  },
  headlineMd: {
    fontFamily: fontFamilies.semiBold,
    fontSize: 24,
    lineHeight: 32,
    letterSpacing: 0,
  },
  bodyLg: {
    fontFamily: fontFamilies.regular,
    fontSize: 18,
    lineHeight: 28,
    letterSpacing: 0,
  },
  bodyMd: {
    fontFamily: fontFamilies.regular,
    fontSize: 16,
    lineHeight: 24,
    letterSpacing: 0,
  },
  labelMd: {
    fontFamily: fontFamilies.semiBold,
    fontSize: 14,
    lineHeight: 20,
    letterSpacing: 0.14,
  },
  labelSm: {
    fontFamily: fontFamilies.medium,
    fontSize: 12,
    lineHeight: 16,
    letterSpacing: 0,
  },
} as const;

export type TypographyVariant = keyof typeof typography;

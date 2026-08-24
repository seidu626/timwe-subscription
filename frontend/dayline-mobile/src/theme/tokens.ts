/**
 * Design tokens ported from
 * apps/designs/stitch_dayline_subscription_content_app/dayline/DESIGN.md
 *
 * Fix-in-implementation deviations from the raw design file are called out
 * inline where they occur (see `focusRing` below).
 */

export const lightColors = {
  surface: '#f7faf2',
  surfaceDim: '#d8dbd3',
  surfaceBright: '#f7faf2',
  surfaceContainerLowest: '#ffffff',
  surfaceContainerLow: '#f2f5ec',
  surfaceContainer: '#ecefe7',
  surfaceContainerHigh: '#e6e9e1',
  surfaceContainerHighest: '#e0e4dc',
  onSurface: '#191d18',
  onSurfaceVariant: '#3f4944',
  inverseSurface: '#2d312c',
  inverseOnSurface: '#eff2ea',
  outline: '#6f7a73',
  outlineVariant: '#bec9c2',
  surfaceTint: '#096b50',
  primary: '#00543d',
  onPrimary: '#ffffff',
  primaryContainer: '#0f6e52',
  onPrimaryContainer: '#9aedca',
  inversePrimary: '#84d7b4',
  secondary: '#805600',
  onSecondary: '#ffffff',
  secondaryContainer: '#fdb741',
  onSecondaryContainer: '#6e4900',
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
  background: '#f7faf2',
  onBackground: '#191d18',
  surfaceVariant: '#e0e4dc',
  cardBorder: '#e5e5e1',
  primarySoft: 'rgba(15,110,82,0.12)',
} as const;

export type ThemeColors = Readonly<Record<keyof typeof lightColors, string>>;

// Material 3 dark counterparts for the same green seed. The *Fixed roles are
// theme-invariant by M3 definition and repeat the light values verbatim.
export const darkColors: ThemeColors = {
  surface: '#101410',
  surfaceDim: '#101410',
  surfaceBright: '#363a35',
  surfaceContainerLowest: '#0b0f0b',
  surfaceContainerLow: '#191d18',
  surfaceContainer: '#1d211c',
  surfaceContainerHigh: '#272b26',
  surfaceContainerHighest: '#323631',
  onSurface: '#e0e4dc',
  onSurfaceVariant: '#bec9c2',
  inverseSurface: '#e0e4dc',
  inverseOnSurface: '#2d312c',
  outline: '#89938c',
  outlineVariant: '#3f4944',
  surfaceTint: '#84d7b4',
  primary: '#84d7b4',
  onPrimary: '#00382a',
  primaryContainer: '#00513b',
  onPrimaryContainer: '#a0f3d0',
  inversePrimary: '#096b50',
  secondary: '#ffba47',
  onSecondary: '#442b00',
  secondaryContainer: '#614000',
  onSecondaryContainer: '#ffddb0',
  tertiary: '#c7c7c3',
  onTertiary: '#30312f',
  tertiaryContainer: '#464744',
  onTertiaryContainer: '#e3e2df',
  error: '#ffb4ab',
  onError: '#690005',
  errorContainer: '#93000a',
  onErrorContainer: '#ffdad6',
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
  background: '#101410',
  onBackground: '#e0e4dc',
  surfaceVariant: '#3f4944',
  cardBorder: '#2a2e29',
  primarySoft: 'rgba(132,215,180,0.16)',
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
  sm: 4,
  default: 8,
  md: 12,
  lg: 16,
  xl: 24,
  full: 9999,
} as const;

export const spacing = {
  base: 8,
  containerMargin: 24,
  gutter: 16,
  sectionGap: 48,
  stackSm: 4,
  stackMd: 12,
  stackLg: 20,
} as const;

export const shadow = {
  card: {
    shadowColor: '#20241f',
    shadowOpacity: 0.06,
    shadowRadius: 20,
    shadowOffset: { width: 0, height: 4 },
    elevation: 3,
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

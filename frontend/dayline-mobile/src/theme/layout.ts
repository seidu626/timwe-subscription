// Fix-in-implementation: the design references let long-form disclosure
// copy (STOP-keyword footnote, compliance text) sit flush with the bottom
// of the screen. On device that copy renders underneath the translucent
// tab bar. TAB_BAR_CLEARANCE is added as extra bottom padding on every
// scrollable tab screen so trailing content always clears the bar.
export const TAB_BAR_HEIGHT = 64;
export const TAB_BAR_CLEARANCE = TAB_BAR_HEIGHT + 24;

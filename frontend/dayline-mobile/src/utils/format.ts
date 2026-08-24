// amount is optional defensively: the backend excludes un-priced campaigns
// from the app catalog, but a data gap must degrade to blank, not crash.
export function formatCurrency(amount: number | null | undefined, currency: string): string {
  if (amount == null || !Number.isFinite(amount)) return '';
  return `${currency} ${amount.toFixed(2)}`;
}

export function formatBillingCycle(cycle: string): string {
  const normalized = cycle.toLowerCase();
  if (normalized === 'day' || normalized === 'daily') return '/day';
  if (normalized === 'week' || normalized === 'weekly') return '/week';
  if (normalized === 'month' || normalized === 'monthly') return '/month';
  return `/${normalized}`;
}

export function estimateReadTime(body: string): string {
  const words = body.trim().split(/\s+/).filter(Boolean).length;
  const minutes = Math.max(1, Math.round(words / 200));
  return `${minutes} min read`;
}

export function formatRelativeDay(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  const now = new Date();
  const startOfDay = (value: Date) => new Date(value.getFullYear(), value.getMonth(), value.getDate()).getTime();
  const diffDays = Math.round((startOfDay(now) - startOfDay(date)) / (1000 * 60 * 60 * 24));
  const time = date.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });
  if (diffDays === 0) return `Today, ${time}`;
  if (diffDays === 1) return `Yesterday, ${time}`;
  return date.toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' });
}

// Carrier catalogs deliver many product names in ALL CAPS. When a name has
// alphabetic characters but no lowercase ones it reads as shouting, so
// title-case each word; mixed-case names pass through untouched.
export function formatProductName(name: string): string {
  const trimmed = name.trim();
  if (!trimmed || /[a-z]/.test(trimmed) || !/[A-Z]/.test(trimmed)) return trimmed;
  return trimmed.toLowerCase().replace(/(^|[\s\-/("'])([a-z])/g, (_match, prefix: string, letter: string) => prefix + letter.toUpperCase());
}

export function truncate(text: string, maxLength: number): string {
  if (text.length <= maxLength) return text;
  return `${text.slice(0, maxLength).trimEnd()}…`;
}

export function pluralize(count: number, singular: string, plural?: string): string {
  const word = count === 1 ? singular : (plural ?? `${singular}s`);
  return `${count} ${word}`;
}

// LINK content items carry an external destination; only http/https URLs are
// safe to hand to Linking.openURL/window.open, so this returns null for
// anything else (custom schemes, malformed strings) and callers must render
// no CTA in that case.
export function parseHttpUrl(url: string): URL | null {
  try {
    const parsed = new URL(url);
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return null;
    return parsed;
  } catch {
    return null;
  }
}

// Ghana-only MSISDN helpers. The contract requires E.164 without a leading
// '+' (e.g. "233241234567"); the phone-entry screen collects a local
// 9-digit number after a locked +233 prefix.

export function toE164Ghana(localInput: string): string {
  const digits = localInput.replace(/\D/g, '');
  const national = digits.startsWith('0') ? digits.slice(1) : digits;
  return `233${national}`;
}

export function isCompleteGhanaLocalNumber(localInput: string): boolean {
  const digits = localInput.replace(/\D/g, '');
  const national = digits.startsWith('0') ? digits.slice(1) : digits;
  return national.length === 9;
}

export function formatLocalInput(raw: string): string {
  const digits = raw.replace(/\D/g, '').slice(0, 9);
  const parts = digits.match(/^(\d{0,2})(\d{0,3})(\d{0,4})$/);
  if (!parts) return digits;
  return [parts[1], parts[2], parts[3]].filter(Boolean).join(' ');
}

export function formatMsisdnForDisplay(msisdn: string): string {
  const national = msisdn.startsWith('233') ? msisdn.slice(3) : msisdn;
  const match = national.match(/^(\d{2})(\d{3})(\d{4})$/);
  if (!match) return `+${msisdn}`;
  return `+233 ${match[1]} ${match[2]} ${match[3]}`;
}

export function maskMsisdnForDisplay(msisdn: string): string {
  const national = msisdn.startsWith('233') ? msisdn.slice(3) : msisdn;
  const match = national.match(/^(\d{2})(\d{3})(\d{4})$/);
  if (!match) return `+${msisdn}`;
  return `+233 ${match[1]} *** ****`;
}

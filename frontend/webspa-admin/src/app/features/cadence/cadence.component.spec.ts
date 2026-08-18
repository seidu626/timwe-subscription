import { CadenceComponent } from './cadence.component';

describe('CadenceComponent SMS segment accounting', () => {
  // smsSegmentInfo is pure; no Angular services are touched.
  const component = new CadenceComponent(null as any, null as any);

  it('counts a short GSM-7 message as one 160-char segment', () => {
    const info = component.smsSegmentInfo('Daily tip: drink water.');
    expect(info.encoding).toBe('GSM-7');
    expect(info.segments).toBe(1);
    expect(info.perSegment).toBe(160);
  });

  it('splits long GSM-7 messages into 153-char segments', () => {
    const info = component.smsSegmentInfo('a'.repeat(161));
    expect(info.encoding).toBe('GSM-7');
    expect(info.segments).toBe(2);
    expect(info.perSegment).toBe(153);
  });

  it('counts GSM-7 extension characters as two chars', () => {
    const info = component.smsSegmentInfo('[]');
    expect(info.encoding).toBe('GSM-7');
    expect(info.chars).toBe(4);
  });

  it('falls back to Unicode with a 70-char first segment', () => {
    const info = component.smsSegmentInfo('Akwaaba 😀');
    expect(info.encoding).toBe('Unicode');
    expect(info.perSegment).toBe(70);
    expect(info.segments).toBe(1);
  });

  it('splits long Unicode messages into 67-char segments', () => {
    const info = component.smsSegmentInfo('😀' + 'a'.repeat(70));
    expect(info.encoding).toBe('Unicode');
    expect(info.segments).toBe(2);
  });
});

import { spawnSync } from 'child_process';
import { isValidCidr } from '@/validators.js';

export interface CidrRange {
  start: number;
  end: number;
  mask: number;
}

export function parseCidr(cidr: string): CidrRange | null {
  if (!isValidCidr(cidr)) return null;
  const [addr, maskStr] = cidr.split('/');
  const mask = Number(maskStr);
  const parts = addr.split('.').map(Number);
  const ip = (parts[0] << 24) | (parts[1] << 16) | (parts[2] << 8) | parts[3];
  const maskBits = mask === 0 ? 0 : (~0 << (32 - mask)) >>> 0;
  const start = (ip & maskBits) >>> 0;
  const end = (start | (~maskBits >>> 0)) >>> 0;
  return { start, end, mask };
}

function toIp(n: number): string {
  return [(n >>> 24) & 0xff, (n >>> 16) & 0xff, (n >>> 8) & 0xff, n & 0xff].join('.');
}

function toCidr(start: number, mask: number): string {
  return `${toIp(start)}/${mask}`;
}

export function rangesOverlap(a: CidrRange, b: CidrRange): boolean {
  return a.start <= b.end && b.start <= a.end;
}

export function listUsedSubnets(): CidrRange[] {
  const ids = spawnSync('docker', ['network', 'ls', '--quiet'], { encoding: 'utf8' });
  if (ids.status !== 0) return [];
  const idList = String(ids.stdout ?? '')
    .split('\n')
    .map((s) => s.trim())
    .filter(Boolean);
  if (idList.length === 0) return [];
  const r = spawnSync(
    'docker',
    [
      'network',
      'inspect',
      '--format',
      '{{range .IPAM.Config}}{{.Subnet}}\n{{end}}',
      ...idList,
    ],
    { encoding: 'utf8' },
  );
  if (r.status !== 0) return [];
  const out: CidrRange[] = [];
  for (const line of String(r.stdout ?? '').split('\n')) {
    const s = line.trim();
    if (!s) continue;
    const c = parseCidr(s);
    if (c) out.push(c);
  }
  return out;
}

function* candidates(): IterableIterator<string> {
  for (let c = 0; c <= 255; c++) {
    for (let d = 0; d <= 240; d += 16) yield `172.25.${c}.${d}/28`;
  }
  for (let b = 26; b <= 254; b++) yield `172.${b}.0.0/28`;
  for (let b = 0; b <= 254; b++) yield `10.${b}.0.0/28`;
}

export function findFreeSubnet(preferred: string, used?: CidrRange[]): string {
  const usedRanges = used ?? listUsedSubnets();
  const pref = parseCidr(preferred);
  if (pref && !usedRanges.some((u) => rangesOverlap(pref, u))) return preferred;
  for (const cand of candidates()) {
    const c = parseCidr(cand);
    if (!c) continue;
    if (cand === preferred) continue;
    if (!usedRanges.some((u) => rangesOverlap(c, u))) return cand;
  }
  return preferred;
}

export function subnetConflict(cidr: string, used?: CidrRange[]): CidrRange | null {
  const c = parseCidr(cidr);
  if (!c) return null;
  const list = used ?? listUsedSubnets();
  for (const u of list) {
    if (rangesOverlap(c, u)) return u;
  }
  return null;
}

export function formatCidr(r: CidrRange): string {
  return toCidr(r.start, r.mask);
}

export function nthHost(cidr: string, n: number): string | null {
  const r = parseCidr(cidr);
  if (!r) return null;
  const ip = (r.start + n) >>> 0;
  if (ip <= r.start || ip >= r.end) return null;
  return toIp(ip);
}

export function lastHost(cidr: string): string | null {
  const r = parseCidr(cidr);
  if (!r) return null;
  const ip = (r.end - 1) >>> 0;
  if (ip <= r.start || ip >= r.end) return null;
  return toIp(ip);
}

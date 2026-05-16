const IMAGE_NAME_RE =
  /^[a-z0-9]+(?:(?:[._]|__|[-]+)[a-z0-9]+)*(?:\/[a-z0-9]+(?:(?:[._]|__|[-]+)[a-z0-9]+)*)*(?::[\w][\w.-]{0,127})?$/;

const CIDR_RE = /^(\d{1,3}\.){3}\d{1,3}\/\d{1,2}$/;

export function isValidImageName(name: string): boolean {
  return IMAGE_NAME_RE.test(name);
}

export function isValidCidr(cidr: string): boolean {
  if (!CIDR_RE.test(cidr)) return false;
  const [addr, maskStr] = cidr.split('/');
  const mask = Number(maskStr);
  if (mask < 0 || mask > 32) return false;
  return addr.split('.').every((p) => {
    const n = Number(p);
    return n >= 0 && n <= 255;
  });
}

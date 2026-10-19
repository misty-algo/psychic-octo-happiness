const csrfCookieName = 'csrf_token';

export function getCSRFToken(): string {
  if (typeof document === 'undefined') return '';
  const cookies = document.cookie.split(';');
  for (const cookie of cookies) {
    const [rawName, ...rawValue] = cookie.trim().split('=');
    if (rawName !== csrfCookieName) continue;
    try {
      return decodeURIComponent(rawValue.join('='));
    } catch {
      return rawValue.join('=');
    }
  }
  return '';
}

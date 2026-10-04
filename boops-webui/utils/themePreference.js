export function normalizeThemePreference(preference) {
  return ['system', 'light', 'dark'].includes(preference) ? preference : 'system';
}

export function resolveTheme(preference, prefersDark) {
  const normalized = normalizeThemePreference(preference);
  return normalized === 'system' ? (prefersDark ? 'dark' : 'light') : normalized;
}

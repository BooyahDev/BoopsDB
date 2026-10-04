export function useApiBaseUrl() {
  return useRuntimeConfig().public.apiBaseUrl.replace(/\/$/, '');
}

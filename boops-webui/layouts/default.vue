<template>
  <ClientOnly>
    <v-app>
      <v-app-bar border elevation="0" color="surface" height="64">
        <v-app-bar-nav-icon v-if="mobile" ref="menuButton" aria-label="ナビゲーションを開閉" :aria-expanded="drawer" @click="drawer = !drawer" />
        <v-toolbar-title><AppBar /></v-toolbar-title>
        <v-select v-model="preference" :items="themeChoices" aria-label="表示テーマ" label="表示テーマ" hide-details class="theme-picker mr-4" />
      </v-app-bar>
      <v-navigation-drawer v-model="drawer" :temporary="mobile" :permanent="!mobile" width="240" @keydown.esc="closeDrawer">
        <SideMenu ref="sideMenu" @navigate="closeDrawer" />
        <template #append><div class="pa-4 text-caption text-medium-emphasis">BoopsDB · マシン管理</div></template>
      </v-navigation-drawer>
      <v-main><slot /></v-main>
    </v-app>
    <template #fallback>
      <div class="shell-fallback" :data-theme="fallbackTheme"><div class="fallback-header">BoopsDB Console</div><div class="fallback-body" role="status">画面を読み込んでいます…</div></div>
    </template>
  </ClientOnly>
</template>

<script setup>
import { ref, watch, onMounted, onBeforeUnmount, nextTick } from 'vue';
import { useTheme, useDisplay } from 'vuetify';
import { normalizeThemePreference, resolveTheme } from '@/utils/themePreference.js';

const preference = useCookie('boops-theme', { default: () => 'system', maxAge: 31536000, sameSite: 'lax', path: '/' });
const fallbackTheme = normalizeThemePreference(preference.value);
const theme = useTheme();
const { mobile } = useDisplay();
const drawer = ref(false);
const menuButton = ref(null);
const sideMenu = ref(null);
const themeChoices = [{ title: 'システム', value: 'system' }, { title: 'ライト', value: 'light' }, { title: 'ダーク', value: 'dark' }];
let media;
const applyTheme = () => { theme.global.name.value = resolveTheme(preference.value, media?.matches || false); };
watch(preference, () => { preference.value = normalizeThemePreference(preference.value); applyTheme(); });
watch(mobile, value => { drawer.value = !value; });
watch(drawer, async (value, previous) => {
  if (mobile.value && value) {
    await nextTick();
    sideMenu.value?.focusFirstLink();
  }
  if (mobile.value && previous && !value) {
    await nextTick();
    menuButton.value?.$el?.focus();
  }
});
const closeDrawer = () => { if (mobile.value) drawer.value = false; };
const handleEscape = event => { if (event.key === 'Escape' && mobile.value && drawer.value) { event.preventDefault(); closeDrawer(); } };
onMounted(() => {
  media = window.matchMedia('(prefers-color-scheme: dark)');
  applyTheme();
  media.addEventListener('change', applyTheme);
  document.addEventListener('keydown', handleEscape);
  drawer.value = !mobile.value;
});
onBeforeUnmount(() => { media?.removeEventListener('change', applyTheme); document.removeEventListener('keydown', handleEscape); });
</script>

<style>
.theme-picker { flex: 0 0 145px; }
.page-container { max-width: 1480px; padding: 28px 32px; }
.page-heading { display: flex; align-items: center; justify-content: space-between; gap: 16px; flex-wrap: wrap; margin-bottom: 24px; }
.page-heading h1 { font-size: 1.65rem; font-weight: 500; line-height: 1.4; }
.page-subtitle { margin-top: 4px; color: rgb(var(--v-theme-on-surface)); opacity: .7; font-size: .9rem; }
.console-card { border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); }
.mono { font-family: ui-monospace, SFMono-Regular, Consolas, monospace; overflow-wrap: anywhere; }
.shell-fallback { min-height: 100vh; background: #F7F9FC; color: #202124; font: 16px system-ui; }
.fallback-header { padding: 20px 32px; border-bottom: 1px solid #B8C0CC; font-weight: 600; }
.fallback-body { padding: 32px; }
.shell-fallback[data-theme="dark"] { background: #15191F; color: #E3E8F1; }
@media (prefers-color-scheme: dark) { .shell-fallback[data-theme="system"] { background: #15191F; color: #E3E8F1; } }
@media (max-width: 600px) { .page-container { padding: 20px 16px; } .theme-picker { flex-basis: 120px; margin-right: 8px !important; } .page-heading h1 { font-size: 1.35rem; } }
</style>

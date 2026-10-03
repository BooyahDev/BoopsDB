import '@mdi/font/css/materialdesignicons.css'
import 'vuetify/styles'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import { normalizeThemePreference, resolveTheme } from '@/utils/themePreference.js'

export default defineNuxtPlugin((app) => {
  const preference = useCookie('boops-theme', { default: () => 'system', maxAge: 31536000, sameSite: 'lax', path: '/' })
  preference.value = normalizeThemePreference(preference.value)
  const prefersDark = import.meta.client && window.matchMedia('(prefers-color-scheme: dark)').matches
  const vuetify = createVuetify({
    components, directives,
    defaults: {
      VBtn: { style: 'text-transform: none; letter-spacing: normal', rounded: 'sm' },
      VCard: { rounded: 'lg', elevation: 0 },
      VTextField: { variant: 'outlined', density: 'compact' },
      VSelect: { variant: 'outlined', density: 'compact' },
    },
    theme: {
      defaultTheme: resolveTheme(preference.value, prefersDark),
      themes: {
        light: {
          dark: false,
          colors: { primary: '#1967D2', secondary: '#5F6368', background: '#F7F9FC', surface: '#FFFFFF', 'surface-variant': '#E8EEF7', 'on-surface-variant': '#3C4043', error: '#B3261E', success: '#137333', warning: '#966000', info: '#1967D2' },
        },
        dark: {
          dark: true,
          colors: { primary: '#8AB4F8', secondary: '#BDC1C6', background: '#15191F', surface: '#20252D', 'surface-variant': '#303A49', 'on-surface-variant': '#E3E8F1', error: '#F2B8B5', success: '#81C995', warning: '#FDD663', info: '#8AB4F8' },
        },
      },
    },
  })
  app.vueApp.use(vuetify)
})

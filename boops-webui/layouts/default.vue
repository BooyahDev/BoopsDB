<template>
  <v-app>
    <v-app-bar color="indigo">
      <!-- <v-app-bar-nav-icon @click="drawer = !drawer"></v-app-bar-nav-icon> -->
      <v-toolbar-title style="cursor: pointer" @click="$router.push('/')">
          <AppBar></AppBar>
      </v-toolbar-title>
      <v-switch
        v-model="darkTheme"
        @update:model-value="changeTheme"
        :prepend-icon="darkTheme ? 'mdi-weather-night' : 'mdi-weather-sunny'"
        hide-details
        color="primary"
        inset
        class="mr-6"
      />
    </v-app-bar>
    <v-navigation-drawer v-model="drawer" temporary="">
      <SideMenu></SideMenu>
    </v-navigation-drawer>
    <slot />
  </v-app>
</template>

<script setup>

import {ref, onMounted } from 'vue'
import { useTheme } from 'vuetify'

const darkTheme = ref(false)
const theme = useTheme()
const drawer = ref(false);

const changeTheme = () => {
  theme.global.name.value = darkTheme.value ? 'dark' : 'light'
}

onMounted(() => {
  const preferDarkScheme = window.matchMedia('(prefers-color-scheme: dark)')
  darkTheme.value = preferDarkScheme.matches
})

</script>
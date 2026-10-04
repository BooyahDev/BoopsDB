<template>
  <NuxtLink to="/" class="brand-link"><v-icon color="primary" class="mr-2">mdi-server-network</v-icon>BoopsDB <span class="brand-caption">Console</span></NuxtLink>
</template>
<style scoped>
.brand-link { color: rgb(var(--v-theme-on-surface)); text-decoration: none; font-size: 1.2rem; font-weight: 600; display: inline-flex; align-items: center; }
.brand-caption { margin-left: 12px; font-size: .85rem; font-weight: 400; opacity: .7; }
@media (max-width: 450px) { .brand-caption { display: none; } }
</style>

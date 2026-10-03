<template>
  <v-list nav density="comfortable" aria-label="メインナビゲーション" class="px-3">
    <v-list-subheader>マシン管理</v-list-subheader>
    <v-list-item to="/" title="検索" prepend-icon="mdi-magnify" :active="route.path === '/'" @click="$emit('navigate')" />
    <v-list-item to="/machines" title="マシン一覧" prepend-icon="mdi-server" :active="route.path.startsWith('/machines') && route.path !== '/machines/register'" @click="$emit('navigate')" />
    <v-list-item to="/machines/register" title="マシン登録" prepend-icon="mdi-plus-box-outline" :active="route.path === '/machines/register'" @click="$emit('navigate')" />
  </v-list>
</template>
<script setup>
const route = useRoute();
defineEmits(['navigate']);
</script>

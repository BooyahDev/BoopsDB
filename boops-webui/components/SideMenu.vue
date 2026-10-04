<template>
  <nav ref="navigation" aria-label="メインナビゲーション">
  <v-list nav density="comfortable" class="px-3">
    <v-list-subheader>マシン管理</v-list-subheader>
    <v-list-item to="/" title="検索" prepend-icon="mdi-magnify" :active="route.path === '/'" @click="$emit('navigate')" />
    <v-list-item to="/machines" title="マシン一覧" prepend-icon="mdi-server" :active="route.path.startsWith('/machines') && route.path !== '/machines/register'" @click="$emit('navigate')" />
    <v-list-item to="/machines/register" title="マシン登録" prepend-icon="mdi-plus-box-outline" :active="route.path === '/machines/register'" @click="$emit('navigate')" />
  </v-list>
  </nav>
</template>
<script setup>
const route = useRoute();
const navigation = ref(null);
const focusFirstLink = () => navigation.value?.querySelector('a')?.focus();
defineExpose({ focusFirstLink });
defineEmits(['navigate']);
</script>

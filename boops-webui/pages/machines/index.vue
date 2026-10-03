<template>
  <v-container class="page-container">
    <div class="page-heading"><div><h1>マシン一覧</h1><p class="page-subtitle">{{ pagination?.total || 0 }} 台のマシン</p></div><v-btn to="/machines/register" color="primary" prepend-icon="mdi-plus">マシン登録</v-btn></div>
    <v-card class="console-card mb-5"><v-card-text>
      <form @submit.prevent="search">
        <v-text-field v-model="query" label="検索クエリ" placeholder="ホスト名、IP、メモなど" clearable append-inner-icon="mdi-magnify" @click:append-inner="search" />
        <v-row dense>
          <v-col cols="12" sm="4"><v-select v-model="sort" :items="sortOptions" label="並べ替え" hide-details /></v-col>
          <v-col cols="6" sm="3"><v-select v-model="order" :items="[{ title: '昇順', value: 'asc' }, { title: '降順', value: 'desc' }]" label="順序" :disabled="!sort" hide-details /></v-col>
          <v-col cols="6" sm="3"><v-select v-model="limit" :items="[25, 50, 100, 200]" label="表示件数" hide-details /></v-col>
          <v-col cols="12" sm="2"><v-btn type="submit" color="primary" block height="40">検索</v-btn></v-col>
        </v-row>
      </form>
    </v-card-text></v-card>
    <v-alert v-if="error" type="error" variant="tonal" class="mb-5">{{ error }}</v-alert>
    <v-card class="console-card">
      <v-progress-linear v-if="loading" indeterminate color="primary" aria-label="マシン一覧を読み込み中" />
      <div class="d-flex align-center flex-wrap ga-2 px-5 py-3"><span class="text-body-2">{{ pagination?.total || 0 }} 件中 {{ pagination?.total ? offset + 1 : 0 }}〜{{ offset + machines.length }} 件</span><v-spacer /><v-btn variant="text" prepend-icon="mdi-chevron-left" :disabled="offset === 0 || loading" @click="page(-1)">前へ</v-btn><v-btn variant="text" append-icon="mdi-chevron-right" :disabled="!pagination?.hasMore || loading" @click="page(1)">次へ</v-btn></div>
      <v-divider />
      <v-table density="comfortable">
        <thead><tr><th>ホスト名</th><th>IP アドレス</th><th>用途</th><th>種類</th><th>最終接続</th></tr></thead>
        <tbody>
          <tr v-for="machine in machines" :key="machine.id">
            <td><NuxtLink :to="{ path: `/machines/${machine.id}`, query: { returnTo: route.fullPath } }" class="hostname-link">{{ machine.hostname }}</NuxtLink><div class="text-caption text-medium-emphasis mono">{{ machine.id }}</div></td>
            <td><div v-for="iface in machine.interfaces" :key="iface.id"><span v-for="ip in iface.ips" :key="ip.id" class="d-block mono text-body-2">{{ ip.ip_address }}<span class="text-caption text-medium-emphasis ml-2">{{ iface.name }}</span></span></div></td>
            <td>{{ machine.purpose || '—' }}</td><td><v-chip size="small" variant="tonal">{{ machine.is_virtual ? '仮想' : '物理' }}</v-chip></td><td>{{ formatDate(machine.last_alive) }}</td>
          </tr>
          <tr v-if="!loading && !machines.length"><td colspan="5" class="text-center pa-8 text-medium-emphasis">条件に一致するマシンはありません。</td></tr>
        </tbody>
      </v-table>
    </v-card>
  </v-container>
</template>
<script setup>
import { ref, watch, onMounted } from 'vue';
import { useApiBaseUrl } from '@/apiConfig';
import { useDateFormatter } from '@/composables/useDateFormatter';
import { createMachineSearchParams } from '@/utils/machineSearchParams.js';
const apiBaseUrl = useApiBaseUrl();
const route = useRoute(), router = useRouter();
const { formatDate } = useDateFormatter();
const query = ref(''), sort = ref(''), order = ref('asc'), limit = ref(50), offset = ref(0);
const machines = ref([]), pagination = ref(null), loading = ref(false), error = ref('');
const sortOptions = [{ title: '登録順（既定）', value: '' }, { title: 'ホスト名', value: 'hostname' }, { title: '登録日時', value: 'created_at' }, { title: '更新日時', value: 'updated_at' }, { title: '最終接続', value: 'last_alive' }, { title: 'OS', value: 'os_name' }, { title: '用途', value: 'purpose' }];
let requestSequence = 0;
const load = async () => {
  const sequence = ++requestSequence;
  query.value = typeof route.query.q === 'string' ? route.query.q : '';
  sort.value = sortOptions.some(option => option.value === route.query.sort) ? route.query.sort : '';
  order.value = route.query.order === 'desc' ? 'desc' : 'asc';
  limit.value = [25, 50, 100, 200].includes(Number(route.query.limit)) ? Number(route.query.limit) : 50;
  offset.value = Math.max(0, Number.parseInt(route.query.offset) || 0);
  const params = createMachineSearchParams({ query: query.value, sort: sort.value, order: order.value, limit: limit.value, offset: offset.value });
  loading.value = true; error.value = '';
  try {
    const response = await fetch(`${apiBaseUrl}/machines/search?${params}`);
    if (!response.ok) throw new Error('マシン一覧を読み込めませんでした。');
    const data = await response.json();
    if (sequence !== requestSequence) return;
    machines.value = data.results;
    pagination.value = data.pagination;
  } catch (err) { if (sequence === requestSequence) error.value = err.message; }
  finally { if (sequence === requestSequence) loading.value = false; }
};
const navigate = nextOffset => {
  const next = { limit: String(limit.value), offset: String(nextOffset) };
  if (query.value?.trim()) next.q = query.value.trim();
  if (sort.value) { next.sort = sort.value; next.order = order.value; }
  if (router.resolve({ path: '/machines', query: next }).fullPath === route.fullPath) load();
  else router.push({ path: '/machines', query: next });
};
const search = () => navigate(0);
const page = direction => navigate(Math.max(0, offset.value + direction * limit.value));
watch(() => route.fullPath, load);
onMounted(load);
</script>
<style scoped>
.hostname-link { color: rgb(var(--v-theme-primary)); font-weight: 500; text-decoration: none; }
.hostname-link:hover { text-decoration: underline; }
</style>

<template>
  <v-container style="margin-top: 50px;">
    <h1>マシン検索結果</h1>

    <!-- Search Query Display -->
    <v-alert v-if="searchQuery" type="info" class="mb-4">
      検索クエリ: <strong>{{ searchQuery }}</strong>
      <v-btn size="small" variant="text" @click="clearSearch">クリア</v-btn>
    </v-alert>

    <!-- Pagination Info -->
    <v-card v-if="pagination" class="mb-4" elevation="1">
      <v-card-text>
        <v-row align="center">
          <v-col>
            {{ pagination.total }}件中 {{ Math.min(pagination.offset + 1, pagination.total) }}〜{{ Math.min(pagination.offset + currentResults.length, pagination.total) }}件を表示
          </v-col>
          <v-col cols="auto">
            <v-btn-group>
              <v-btn :disabled="pagination.offset === 0" @click="previousPage">
                <v-icon>mdi-chevron-left</v-icon>
                前へ
              </v-btn>
              <v-btn :disabled="!pagination.hasMore" @click="nextPage">
                次へ
                <v-icon>mdi-chevron-right</v-icon>
              </v-btn>
            </v-btn-group>
          </v-col>
        </v-row>
      </v-card-text>
    </v-card>

    <!-- Loading Indicator -->
    <div v-if="isLoading" class="text-center">
      <v-progress-circular indeterminate color="primary" size="64"></v-progress-circular>
      <p>検索中...</p>
    </div>
    
    <div v-else>
      <div v-if="currentResults.length > 0">
        <!-- Search Results Count -->
        <v-chip class="mb-4" color="primary" label>
          <v-icon start>mdi-database-search</v-icon>
          {{ pagination ? pagination.total : currentResults.length }}件見つかりました
        </v-chip>

        <v-data-table
          class="elevation-1"
          density="compact"
          :headers="headers"
          :items="currentResults"
          item-value="hostname"
          :items-per-page="-1"
          :sort-by="[{ key: sortBy, order: sortOrder }]"
        >
          <!-- Hostname をリンクに -->
          <template #item.hostname="{ item }">
            <a :href="`/machines/${item.id}`" class="hostname-link" target="_blank">
              {{ item.hostname }}
            </a>
          </template>

          <!-- IPAddr カスタム表示 -->
          <template #item.ipaddr="{ item }">
            <div class="ip-container">
              <v-chip v-for="(ip, index) in item.ipaddr.slice(0, 3)" 
                     :key="index" size="small" class="mr-1 mb-1">
                {{ ip }}
              </v-chip>
              <v-chip v-if="item.ipaddr.length > 3" size="small" variant="outlined">
                +{{ item.ipaddr.length - 3 }}
              </v-chip>
            </div>
          </template>

          <!-- VM Type display -->
          <template #item.vm_type="{ item }">
            <v-chip :color="item.vm_type === 'Virtual' ? 'green' : 'blue'" size="small">
              <v-icon start>{{ item.vm_type === 'Virtual' ? 'mdi-cloud' : 'mdi-server' }}</v-icon>
              {{ item.vm_type }}
            </v-chip>
          </template>

          <!-- Status カスタム表示 -->
          <template #item.status="{ item }">
            <v-chip :color="item.status === 'online' ? 'green' : 'red'" size="small">
              <v-icon start>{{ item.status === 'online' ? 'mdi-check-circle' : 'mdi-alert-circle' }}</v-icon>
              {{ item.status === 'online' ? 'Online' : 'Offline' }}
            </v-chip>
          </template>

          <!-- Last Alive display -->
          <template #item.last_alive="{ item }">
            <span :class="item.status === 'online' ? 'text-green' : 'text-red'">
              {{ formatLastAlive(item.last_alive) }}
            </span>
          </template>
        </v-data-table>
      </div>
      
      <v-empty-state v-else
        headline="検索結果が見つかりません"
        title="検索条件を変更してお試しください"
        text="異なるキーワードを使用するか、検索範囲を広げてみてください。"
        icon="mdi-database-search-outline"
      >
        <template #actions>
          <v-btn @click="clearSearch" color="primary">
            すべてのマシンを表示
          </v-btn>
        </template>
      </v-empty-state>
    </div>
  </v-container>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { apiBaseUrl } from '@/apiConfig';

const route = useRoute();
const router = useRouter();
const searchQuery = ref('');
const searchResults = ref([]);
const currentResults = ref([]);
const isLoading = ref(false);
const pagination = ref(null);

// URL parameters
const sortBy = ref(route.query.sort || 'hostname');
const sortOrder = ref(route.query.order || 'asc');
const limit = ref(parseInt(route.query.limit) || 50);
const offset = ref(parseInt(route.query.offset) || 0);

// テーブルのヘッダー定義
const headers = ref([
  { title: 'ホスト名', key: 'hostname', sortable: true },
  { title: 'IPアドレス', key: 'ipaddr', sortable: false },
  { title: '用途', key: 'purpose', sortable: true },
  { title: 'タイプ', key: 'vm_type', sortable: false },
  { title: 'ステータス', key: 'status', sortable: false },
  { title: '最終接続', key: 'last_alive', sortable: true }
]);

onMounted(() => {
  if (route.query.q) {
    searchQuery.value = route.query.q;
    performSearch(route.query.q);
  } else {
    fetchAllMachines();
  }
});

function formatLastAlive(lastAlive) {
  if (!lastAlive) return 'なし';
  
  const now = new Date();
  const lastAliveDate = new Date(lastAlive);
  const diffMinutes = Math.floor((now - lastAliveDate) / 1000 / 60);
  
  if (diffMinutes < 1) return 'たった今';
  if (diffMinutes < 60) return `${diffMinutes}分前`;
  if (diffMinutes < 1440) return `${Math.floor(diffMinutes / 60)}時間前`;
  return `${Math.floor(diffMinutes / 1440)}日前`;
}

async function fetchAllMachines() {
  isLoading.value = true;
  try {
    const response = await fetch(`${apiBaseUrl}/machines`);
    if (response.ok) {
      searchResults.value = await response.json();
      setTable(searchResults.value);
      pagination.value = null; // 全件取得の場合はページネーション無し
    } else {
      alert('Failed to fetch machines');
    }
  } catch (error) {
    console.error(error);
    alert('Unexpected error while fetching all machines.');
  } finally {
    isLoading.value = false;
  }
}

async function performSearch(query) {
  isLoading.value = true;
  try {
    const params = new URLSearchParams();
    params.append('q', query);
    params.append('sort', sortBy.value);
    params.append('order', sortOrder.value);
    params.append('limit', limit.value.toString());
    params.append('offset', offset.value.toString());

    const response = await fetch(`${apiBaseUrl}/machines/search?${params.toString()}`);
    if (response.ok) {
      const data = await response.json();
      
      // Check if response has pagination structure
      if (data.results && data.pagination) {
        searchResults.value = data.results;
        pagination.value = data.pagination;
      } else {
        // Fallback for old API response format
        searchResults.value = data;
        pagination.value = null;
      }
      
      setTable(searchResults.value);
    } else {
      alert('Failed to search machines');
    }
  } catch (error) {
    console.error('Search error:', error);
    alert('An unexpected error occurred during the search.');
  } finally {
    isLoading.value = false;
  }
}

function setTable(machines) {
  currentResults.value = []; // 前回結果をクリア
  const now = new Date();

  machines.forEach(machine => {
    const ipList = [];

    if (Array.isArray(machine.interfaces)) {
      machine.interfaces.forEach(iface => {
        if (Array.isArray(iface.ips)) {
          iface.ips.forEach(ipObj => {
            ipList.push(`${ipObj.ip_address}/${ipObj.subnet_mask}`);
          });
        }
      });
    }

    // last_alive の差分を計算
    const lastAlive = new Date(machine.last_alive);
    const diffMinutes = (now - lastAlive) / 1000 / 60;
    const isOnline = diffMinutes <= 5;

    const row = {
      id: machine.id,
      hostname: machine.hostname,
      ipaddr: ipList,
      purpose: machine.purpose || '-',
      vm_type: machine.is_virtual ? 'Virtual' : 'Physical',
      status: isOnline ? 'online' : 'offline',
      last_alive: machine.last_alive
    };

    currentResults.value.push(row);
  });
}

function clearSearch() {
  searchQuery.value = '';
  offset.value = 0;
  // URLのクエリパラメータをクリアしてからマシン一覧を取得
  const newQuery = { ...route.query };
  delete newQuery.q;
  delete newQuery.offset;
  
  router.replace({ path: '/machines', query: newQuery }).then(() => {
    fetchAllMachines();
  });
}

function nextPage() {
  if (pagination.value && pagination.value.hasMore) {
    offset.value += limit.value;
    updateURL();
    performSearch(searchQuery.value);
  }
}

function previousPage() {
  if (offset.value > 0) {
    offset.value = Math.max(0, offset.value - limit.value);
    updateURL();
    performSearch(searchQuery.value);
  }
}

function updateURL() {
  const query = { ...route.query };
  query.offset = offset.value.toString();
  query.limit = limit.value.toString();
  query.sort = sortBy.value;
  query.order = sortOrder.value;
  
  router.replace({ path: route.path, query });
}
</script>

<style scoped>
.hostname-link {
  text-decoration: none;
  color: #1976d2;
  font-weight: 500;
}

.hostname-link:hover {
  text-decoration: underline;
}

.ip-container {
  max-width: 200px;
}

.text-green {
  color: #4caf50;
}

.text-red {
  color: #f44336;
}

.text-grey {
  color: #9e9e9e;
}
</style>

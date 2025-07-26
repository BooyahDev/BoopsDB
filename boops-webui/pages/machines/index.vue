<template>
  <v-container style="margin-top: 50px;">
    <h1>マシン検索結果</h1>

    <!-- Loading Indicator -->
    <div v-if="isLoading" class="loading-indicator">
      Loading...
    </div>
    <div v-else>
      <div v-if="result.length > 0">
        <v-data-table
        class="elevation-1"
        density="compact"
        :headers="headers"
        :items="result"
        item-value="hostname"
        :items-per-page="-1"
        >
        <!-- Hostname をリンクに -->
        <template #item.hostname="{ item }">
            <a :href="`/machines/${item.id}`" class="hostname-link">
            {{ item.hostname }}
            </a>
        </template>

        <!-- IPAddr カスタム表示 -->
        <template #item.ipaddr="{ item }">
            <ul class="ip-list">
            <li v-for="(ip, index) in item.ipaddr" :key="index">
                {{ ip }}
            </li>
            </ul>
        </template>

        <!-- Status カスタム表示 -->
        <template #item.status="{ item }">
            <span :class="item.status === 'online' ? 'status-dot green' : 'status-dot red'"></span>
            {{ item.status === 'online' ? 'Online' : 'Offline' }}
        </template>
        </v-data-table>

      </div>
      <div v-else>
        Not Found...
      </div>
    </div>
  </v-container>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import { apiBaseUrl } from '@/apiConfig';

const route = useRoute();
const searchQuery = ref('');
const searchResults = ref([]);
const result = ref([]);
const isLoading = ref(false);

// テーブルのヘッダー定義
const headers = ref([
  { title: 'Hostname', key: 'hostname' },
  { title: 'IPAddr', key: 'ipaddr' },
  { title: 'Purpose', key: 'purpose' },
  { title: 'Status', key: 'status' }
]);

onMounted(() => {
  if (route.query.q) {
    searchQuery.value = route.query.q;
    performSearch(route.query.q);
  } else {
    fetchAllMachines();
  }
});

async function fetchAllMachines() {
  isLoading.value = true;
  try {
    const response = await fetch(`${apiBaseUrl}/machines`);
    if (response.ok) {
      searchResults.value = await response.json();
      setTable(searchResults.value);
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
    const response = await fetch(`${apiBaseUrl}/machines/search?q=${encodeURIComponent(query)}`);
    if (response.ok) {
      searchResults.value = await response.json();
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
  result.value = []; // 前回結果をクリア
  const now = new Date();

  machines.forEach(machine => {
    const ipList = [];

    if (Array.isArray(machine.interfaces)) {
      machine.interfaces.forEach(iface => {
        if (Array.isArray(iface.ips)) {
          iface.ips.forEach(ipObj => {
            ipList.push(`${ipObj.ip_address} / ${ipObj.subnet_mask}`);
          });
        }
      });
    }

    // updated_at の差分を計算
    const updatedAt = new Date(machine.updated_at);
    const diffMinutes = (now - updatedAt) / 1000 / 60;
    const isOnline = diffMinutes <= 5;

    const row = {
      id: machine.id,
      hostname: machine.hostname,
      ipaddr: ipList,
      purpose: machine.purpose || '-',
      status: isOnline ? 'online' : 'offline',
    };

    result.value.push(row);
  });
}


</script>

<style scoped>
.loading-indicator {
  font-weight: bold;
  font-size: 1.2em;
  color: #555;
  padding: 1em;
}

.ip-list {
  list-style-type: none;
  padding: 0;
  margin: 0;
}

.ip-list li {
  margin: 2px 0;
}

.status-dot {
  display: inline-block;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  margin-right: 6px;
  vertical-align: middle;
}

.status-dot.green {
  background-color: #4caf50;
}

.status-dot.red {
  background-color: #f44336;
}

</style>

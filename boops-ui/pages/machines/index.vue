<template>
  <div class="container">
    <h1>Machine Search Results</h1>

    <!-- Loading Indicator -->
    <div v-if="isLoading" class="loading-indicator">
      Loading...
    </div>

    <!-- Search Results Table -->
    <section v-if="searchResults.length > 0">
      <h2>Search Results</h2>
      <table>
        <thead>
          <tr>
            <th>Hostname</th>
            <th>IP Address</th>
            <th>OS Name</th>
            <th>Is Virtual Machine</th>
            <th>Parent ID</th>
            <th>Status</th>
            <th>Details</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="machine in searchResults" :key="machine.id">
            <td>{{ machine.hostname }}</td>
            <td>
              <ul class="ip-list">
            <li v-for="(interfaceData, name) in machine.interfaces" :key="name">
              {{ interfaceData.ip }} ({{ interfaceData.mac_address || 'N/A' }})
                </li>
              </ul>
            </td>
            <td>{{ machine.os_name || 'Unknown' }}</td>
            <td>{{ machine.is_virtual ? 'Yes' : 'No' }}</td>
            <td v-if="machine.is_virtual">{{ machine.parent_machine_id || 'None' }}</td>
            <td v-else>N/A</td>
            <td>{{ machine.status || 'Unknown' }}</td>
            <td>
              <nuxt-link :to="`/machines/${machine.id}`">View Details</nuxt-link>
            </td>
          </tr>
        </tbody>
      </table>
    </section>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';

const route = useRoute();
const router = useRouter();
const searchQuery = ref('');
const searchResults = ref([]);
const isLoading = ref(false);

onMounted(() => {
  if (route.query.q) {
    searchQuery.value = route.query.q;
    performSearch(route.query.q);
  } else {
    // If no query, fetch all machines
    fetchAllMachines();
  }
});

async function fetchAllMachines() {
  const response = await fetch('http://localhost:3001/api/machines');
  if (response.ok) {
    searchResults.value = await response.json();
  } else {
    alert('Failed to fetch machines');
  }
}

async function performSearch(query) {
  isLoading.value = true;

  try {
    const response = await fetch(`http://localhost:3001/api/machines/search?q=${encodeURIComponent(query)}`);
    if (response.ok) {
      searchResults.value = await response.json();
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
</script>

<style scoped>
.container {
  max-width: 1200px;
  margin: auto;
  padding: 2rem;
}

h1, h2 {
  color: #333;
}

form div {
  margin-bottom: 1rem;
}

label {
  font-weight: bold;
  display: block;
  margin-bottom: 0.5rem;
}

input[type="text"] {
  width: calc(100% - 24px);
  padding: 8px 12px;
  margin-top: 4px;
  border-radius: 4px;
  border: 1px solid #ccc;
}

button {
  display: inline-block;
  background-color: #007bff;
  color: white;
  padding: 0.5rem 1rem;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  margin-right: 0.5rem;
}

button:hover {
  background-color: #0056b3;
}

table {
  width: 100%;
  border-collapse: collapse;
  margin-top: 2rem;
}

th, td {
  border: 1px solid #ddd;
  padding: 8px;
  text-align: left;
}

th {
  background-color: #f4f4f4;
  font-weight: bold;
}

.ip-list {
  list-style-type: none;
  padding: 0;
}

.ip-list li {
  margin: 2px 0;
}
</style>

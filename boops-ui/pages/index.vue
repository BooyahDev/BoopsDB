<template>
  <div class="container">
    <h1>Search Machines</h1>

    <!-- Machine Search Form -->
    <section>
      <form @submit.prevent="searchMachines">
        <div>
          <label for="search">Search by any element:</label>
          <input v-model="searchQuery" type="text" id="search" required />
        </div>
        <button type="submit">Search</button>
      </form>

      <!-- Search Results -->
      <section v-if="searchResults.length > 0">
        <h2>Search Results</h2>
        <ul>
          <li v-for="machine in searchResults" :key="machine.id">
            <h3>{{ machine.hostname }}</h3>
            <div v-for="(interfaceData, name) in machine.interfaces" :key="name">
              <h4>Interface: {{ name }}</h4>
              <p><strong>IP Address:</strong> {{ interfaceData.ip }}</p>
            </div>
          </li>
        </ul>
      </section>
    </section>
  </div>
</template>

<script setup>
import { ref } from 'vue';

const searchQuery = ref('');
const searchResults = ref([]);

async function searchMachines() {
  if (!searchQuery.value.trim()) return;

  const response = await fetch(`http://localhost:3001/api/machines/search?q=${encodeURIComponent(searchQuery.value)}`);
  if (response.ok) {
    searchResults.value = await response.json();
  } else {
    alert('Failed to search machines');
  }
}
</script>

<style scoped>
.container {
  max-width: 800px;
  margin: auto;
  padding: 2rem;
}
h1, h2, h3, h4 {
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
</style>

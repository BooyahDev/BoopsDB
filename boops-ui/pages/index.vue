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
          </section>
  </div>
</template>

<script setup>
import { ref } from 'vue';
import { useRouter } from 'vue-router';

const router = useRouter();
const searchQuery = ref('');

async function searchMachines() {
  if (!searchQuery.value.trim()) return;

  // Redirect to the machines page with query
  router.push({ path: '/machines', query: { q: searchQuery.value } });
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

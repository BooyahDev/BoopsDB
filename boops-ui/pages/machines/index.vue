<template>
  <div class="container">
    <h1>Machine List</h1>

    <!-- Register New Machine Link -->
    <NuxtLink to="/machines/register" class="register-btn">Register New Machine</NuxtLink>

    <!-- Machine List -->
    <section v-if="machines.length > 0">
      <table class="machine-table">
        <thead>
          <tr>
            <th>Hostname</th>
            <th>Last Alive</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="machine in machines" :key="machine.id">
            <td><NuxtLink :to="`/machines/${machine.id}`">{{ machine.hostname }}</NuxtLink></td>
            <td>{{ machine.last_alive ? new Date(machine.last_alive).toLocaleString() : 'N/A' }}</td>
          </tr>
        </tbody>
      </table>
    </section>

    <section v-else>
      <p>No machines available.</p>
    </section>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';

const machines = ref([]);

async function fetchMachines() {
  const response = await fetch('http://localhost:3001/api/machines');
  machines.value = await response.json();
}

onMounted(fetchMachines);
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
.machine-table {
  width: 100%;
  border-collapse: collapse;
  margin-top: 1rem;
}
.machine-table th,
.machine-table td {
  padding: 0.5rem;
  text-align: left;
  border-bottom: 1px solid #ddd;
}
.machine-table th {
  background-color: #f2f2f2;
}
.machine-table tr:hover {
  background-color: #f9f9f9;
}
a {
  text-decoration: none;
  color: #007bff;
}
a:hover {
  text-decoration: underline;
}
.register-btn {
  display: inline-block;
  background-color: #28a745;
  color: white;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  margin-bottom: 1rem;
  text-decoration: none;
}
.register-btn:hover {
  background-color: #218838;
}
</style>

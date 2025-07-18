<template>
  <div class="container">
    <h1>{{ machine ? machine.hostname : 'Loading...' }}</h1>

    <section v-if="machine" class="machine-details">
      <p><strong>Model:</strong> {{ machine.model_info || 'N/A' }}</p>
      <p><strong>Usage:</strong> {{ machine.usage_desc || 'N/A' }}</p>
      <p><strong>Memo:</strong> {{ machine.memo || 'N/A' }}</p>
      <p><strong>Last Alive:</strong> {{ machine.last_alive ? new Date(machine.last_alive).toLocaleString() : 'N/A' }}</p>

      <!-- Interfaces -->
      <div v-for="(interfaceData, name) in machine.interfaces" :key="name" class="interface">
        <h2>Interface: {{ name }}</h2>
        <p><strong>IP Address:</strong> {{ interfaceData.ip_address || 'N/A' }}</p>
        <p><strong>Subnet Mask:</strong> {{ interfaceData.subnet_mask || 'N/A' }}</p>
        <p><strong>Gateway:</strong> {{ interfaceData.gateway || 'N/A' }}</p>
        <p><strong>DNS Servers:</strong> {{ interfaceData.dns_servers.length > 0 ? interfaceData.dns_servers.join(', ') : 'N/A' }}</p>
      </div>

      <!-- Back to list -->
      <NuxtLink to="/machines">Back to Machine List</NuxtLink>
    </section>

    <section v-else-if="error">
      <p>Error loading machine details: {{ error.message }}</p>
      <NuxtLink to="/machines">Back to Machine List</NuxtLink>
    </section>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import { useRoute } from 'vue-router';

const route = useRoute();
const machine = ref(null);
const error = ref(null);

async function fetchMachineDetails() {
  try {
    const response = await fetch(`http://localhost:3001/api/machines/${route.params.id}`);
    if (!response.ok) {
      throw new Error('Failed to fetch machine details');
    }
    machine.value = await response.json();
  } catch (err) {
    error.value = err;
  }
}

onMounted(fetchMachineDetails);
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
.machine-details p {
  margin-bottom: 0.5rem;
}
.interface {
  border-top: 1px solid #ccc;
  padding-top: 1rem;
  margin-top: 1rem;
}
a {
  text-decoration: none;
  color: #007bff;
}
a:hover {
  text-decoration: underline;
}
</style>

<template>
  <div class="container" v-if="machine">
    <h1>{{ machine.hostname }} Details</h1>

    <!-- Machine Information -->
    <section>
      <h2>Main Information</h2>
      <table>
        <tr>
          <th>ID:</th>
          <td>{{ machine.id }}</td>
        </tr>
        <tr>
          <th>Hostname:</th>
          <td>{{ machine.hostname }}</td>
        </tr>
        <tr>
          <th>Status:</th>
          <td>{{ machine.status || 'Unknown' }}</td>
        </tr>
      </table>
    </section>

    <!-- Interfaces -->
    <section v-for="(interfaceData, name) in machine.interfaces" :key="name">
      <h2>Interface: {{ name }}</h2>
      <table>
        <tr>
          <th>IP Address:</th>
          <td>{{ interfaceData.ip }}</td>
        </tr>
        <tr>
          <th>Subnet Mask:</th>
          <td>{{ interfaceData.subnet_mask || 'N/A' }}</td>
        </tr>
        <tr>
          <th>Gateway:</th>
          <td>{{ interfaceData.gateway || 'N/A' }}</td>
        </tr>
        <tr>
          <th>DNS Servers:</th>
          <td>{{ interfaceData.dns_servers ? interfaceData.dns_servers.join(', ') : 'N/A' }}</td>
        </tr>
      </table>
    </section>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import { useRoute } from 'vue-router';

const route = useRoute();
const machine = ref(null);

onMounted(async () => {
  const response = await fetch(`http://localhost:3001/api/machines/${route.params.id}`);
  if (response.ok) {
    machine.value = await response.json();
  } else {
    alert('Failed to load machine details');
  }
});
</script>

<style scoped>
.container {
  max-width: 800px;
  margin: auto;
  padding: 2rem;
}

h1, h2 {
  color: #333;
}

table {
  width: 100%;
  border-collapse: collapse;
  margin-bottom: 2rem;
}

th, td {
  border: 1px solid #ddd;
  padding: 0.5rem;
  text-align: left;
}

th {
  background-color: #f4f4f4;
  font-weight: bold;
}
</style>

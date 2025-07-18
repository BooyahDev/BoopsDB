<template>
  <div class="container" v-if="machine">
    <h1>{{ machine.hostname }} Details</h1>

    <!-- Machine Information -->
    <section>
      <h2>Main Information</h2>
      <dl>
        <dt>ID:</dt>
        <dd>{{ machine.id }}</dd>
        <dt>Hostname:</dt>
        <dd>{{ machine.hostname }}</dd>
        <dt>Status:</dt>
        <dd>{{ machine.status || 'Unknown' }}</dd>
      </dl>
    </section>

    <!-- Interfaces -->
    <section v-for="(interfaceData, name) in machine.interfaces" :key="name">
      <h2>Interface: {{ name }}</h2>
      <dl>
        <dt>IP Address:</dt>
        <dd>{{ interfaceData.ip }}</dd>
        <dt>Subnet Mask:</dt>
        <dd>{{ interfaceData.subnet_mask || 'N/A' }}</dd>
        <dt>Gateway:</dt>
        <dd>{{ interfaceData.gateway || 'N/A' }}</dd>
        <dt>DNS Servers:</dt>
        <dd>{{ interfaceData.dns_servers ? interfaceData.dns_servers.join(', ') : 'N/A' }}</dd>
      </dl>
    </section>

    <!-- Back to Results -->
    <nuxt-link to="/machines">Back to Search Results</nuxt-link>
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

dl {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  margin-bottom: 2rem;
}

dt {
  font-weight: bold;
  flex: 0 0 150px;
  text-align: right;
  margin-right: 1rem;
}

dd {
  flex: 1 1 calc(100% - 160px);
}
</style>

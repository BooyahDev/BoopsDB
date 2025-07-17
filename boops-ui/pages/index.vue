<template>
  <div class="container">
    <h1>Machine Management</h1>

    <!-- Machine List -->
    <section v-if="machines.length > 0">
      <h2>Machines</h2>
      <ul>
        <li v-for="machine in machines" :key="machine.id">
          <h3>{{ machine.hostname }}</h3>
          <div v-for="(interfaceData, name) in machine.interfaces" :key="name">
            <h4>Interface: {{ name }}</h4>
            <p><strong>IP Address:</strong> {{ interfaceData.ip }}</p>
            <p><strong>Subnet Mask:</strong> {{ interfaceData.subnet }}</p>
            <p><strong>Gateway:</strong> {{ interfaceData.gateway || 'N/A' }}</p>
            <p><strong>DNS Servers:</strong> {{ interfaceData.dns_servers.length > 0 ? interfaceData.dns_servers.join(', ') : 'N/A' }}</p>
          </div>
        </li>
      </ul>
    </section>

    <!-- New Machine Form -->
    <section>
      <h2>Add New Machine</h2>
      <form @submit.prevent="addMachine">
        <div>
          <label for="hostname">Hostname:</label>
          <input v-model="newMachine.hostname" type="text" id="hostname" required />
        </div>

        <!-- Interface Form -->
        <div v-for="(interfaceData, name) in newMachine.interfaces" :key="name">
          <h3>Interface: {{ name }}</h3>
          <div>
            <label for="ip">IP Address:</label>
            <input v-model="interfaceData.ip_address" type="text" required />
          </div>
          <div>
            <label for="subnet">Subnet Mask:</label>
            <input v-model="interfaceData.subnet_mask" type="text" />
          </div>
          <div>
            <label for="gateway">Gateway:</label>
            <input v-model="interfaceData.gateway" type="text" />
          </div>
          <div>
            <label for="dns_servers">DNS Servers (comma-separated):</label>
            <input v-model="interfaceData.dns_servers" type="text" />
          </div>
        </div>

        <!-- Add another interface -->
        <button type="button" @click.prevent="addInterface">Add Interface</button>

        <button type="submit">Add Machine</button>
      </form>
    </section>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';

const machines = ref([]);
const newMachine = ref({
  hostname: '',
  interfaces: {
    eth0: {
      ip_address: '',
      subnet_mask: '',
      gateway: '',
      dns_servers: ''
    }
  }
});

async function fetchMachines() {
  const response = await fetch('http://localhost:3001/api/machines');
  machines.value = await response.json();
}

function addInterface() {
  const interfaceCount = Object.keys(newMachine.value.interfaces).length;
  newMachine.value.interfaces[`eth${interfaceCount}`] = {
    ip_address: '',
    subnet_mask: '',
    gateway: '',
    dns_servers: ''
  };
}

async function addMachine() {
  // Convert interfaces object to the format expected by API
  const formattedInterfaces = {};
  for (const [name, data] of Object.entries(newMachine.value.interfaces)) {
    formattedInterfaces[name] = {
      ip_address: data.ip_address,
      subnet_mask: data.subnet_mask || '',
      gateway: data.gateway || '',
      dns_servers: data.dns_servers ? data.dns_servers.split(',').map(s => s.trim()) : []
    };
  }

  const response = await fetch('http://localhost:3001/api/machines', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      hostname: newMachine.value.hostname,
      interfaces: formattedInterfaces
    })
  });

  if (response.ok) {
    fetchMachines();
    // Reset form
    newMachine.value = {
      hostname: '',
      interfaces: { eth0: { ip_address: '', subnet_mask: '', gateway: '', dns_servers: '' } }
    };
  } else {
    alert('Failed to add machine');
  }
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

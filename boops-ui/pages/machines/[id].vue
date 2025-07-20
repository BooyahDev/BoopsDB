<template>
  <div class="container" v-if="machine">
    <h1>{{ machine.hostname }} Details</h1>

      <!-- Machine Information -->
    <section>
      <h2>Main Information</h2>
      <table>
        <tr>
          <th>ID:</th>
          <td>
            {{ machine.id }}
            <button @click="copyToClipboard(machine.id, $event)" class="copy-btn">Copy</button>
          </td>
        </tr>
        <tr>
          <th>Hostname:</th>
          <td>
            {{ machine.hostname }}
            <button @click="copyToClipboard(machine.hostname, $event)" class="copy-btn">Copy</button>
          </td>
        </tr>
        <tr>
          <th>CPU Info:</th>
          <td>{{ machine.cpu_info || 'N/A' }}</td>
        </tr>
        <tr>
          <th>CPU Architecture:</th>
          <td>{{ machine.cpu_arch || 'N/A' }}</td>
        </tr>
        <tr>
          <th>Memory Size:</th>
          <td>{{ machine.memory_size || 'N/A' }}</td>
        </tr>
        <tr>
          <th>Disk Info:</th>
          <td>{{ machine.disk_info || 'N/A' }}</td>
        </tr>
        <tr>
          <th>Is Virtual Machine:</th>
          <template v-if="isEditingVmStatus">
            <td>
              <label>
                <input type="checkbox" v-model="vmStatusEdit.is_virtual" @change="handleVmStatusChange" />
                {{ vmStatusEdit.is_virtual ? 'Yes' : 'No' }}
              </label>
              <div v-if="vmStatusEdit.is_virtual" class="parent-id-edit">
                <label>Parent Machine ID:</label>
                <input v-model="vmStatusEdit.parent_machine_id" placeholder="Enter parent machine ID" />
              </div>
              <button @click="saveVmStatus" :disabled="isUpdatingVmStatus">Save</button>
              <span v-if="isUpdatingVmStatus">Saving...</span>
              <button @click="cancelEditVmStatus">Cancel</button>
            </td>
          </template>
          <template v-else>
            <td>
              {{ machine.is_virtual ? 'Yes' : 'No' }}
              <button @click="enableEditVmStatus">Edit</button>
            </td>
          </template>
        </tr>
        <tr>
          <th>Purpose:</th>
          <td>{{ machine.purpose || 'N/A' }}</td>
        </tr>
        <tr v-if="machine.is_virtual">
          <th>Parent Machine ID:</th>
          <template v-if="isEditingParentId">
            <td>
              <input v-model="machine.parent_machine_id" />
              <button @click="updateParentId" :disabled="isUpdatingParentId">Save</button>
              <span v-if="isUpdatingParentId">Saving...</span>
              <button @click="cancelEditParentId">Cancel</button>
            </td>
          </template>
          <template v-else>
            <td>
              {{ machine.parent_machine_id || 'N/A' }}
              <button @click="copyToClipboard(machine.parent_machine_id, $event)" class="copy-btn">Copy</button>
              <!-- <button @click="enableEditParentId">Edit</button> -->
            </td>
          </template>
        </tr>
        <tr v-if="machine.is_virtual && machine.parent_machine_id">
          <th>Parent Machine:</th>
          <td>
            <a :href="`/machines/${machine.parent_machine_id}`">{{ machine.parentHostname || machine.hostname || 'Unknown' }}</a>
          </td>
        </tr>
        <tr>
          <th>Memo:</th>
          <template v-if="isEditingMemo">
            <td>
              <textarea v-model="machine.memo" rows="3"></textarea>
              <button @click="updateMemo" :disabled="isUpdatingMemo">Save</button>
              <span v-if="isUpdatingMemo">Saving...</span>
              <button @click="cancelEditMemo">Cancel</button>
            </td>
          </template>
          <template v-else>
            <td>{{ machine.memo || 'N/A' }}
              <button @click="enableEditMemo">Edit</button>
            </td>
          </template>
        </tr>
      </table>
    </section>

      <!-- Interfaces -->
    <section v-for="(interfaceData, name) in machine.interfaces" :key="name">
      <h2 v-if="isEditingName[name]">
        Interface Name:
        <input v-model="interfaceData.name" />
        <button @click="saveInterfaceName(name)" :disabled="isLoadingName[name]">Save</button>
        <span v-if="isLoadingName[name]">Saving...</span>
        <button @click="cancelEditInterfaceName(name)">Cancel</button>
      </h2>
      <h2 v-else>
        Interface: {{ name }}
        <button @click="editInterfaceName(name)">Edit</button>
        <button @click="confirmDeleteInterface(name)" class="delete-btn">Delete</button>
      </h2>
      <template v-if="isEditingIp[name]">
        <table>
          <tr>
            <th>IP Address:</th>
            <td>
              <input v-model="interfaceData.ip" />
              <button @click="saveNetworkSetting(name, 'ip')" :disabled="isLoadingIp[name]">Save</button>
              <span v-if="isLoadingIp[name]">Saving...</span>
              <button @click="cancelEditIp(name)">Cancel</button>
            </td>
          </tr>
          <tr>
            <th>MAC Address:</th>
            <td>{{ interfaceData.mac_address || 'N/A' }}</td>
          </tr>
          <template v-if="interfaceData">
            <tr>
              <th>Subnet Mask:</th>
              <template v-if="isEditingSubnet[name]">
                <td>
                  <input v-model="interfaceData.subnet" />
                  <button @click="saveNetworkSetting(name, 'subnet')" :disabled="isLoadingSubnet[name]">Save</button>
                  <span v-if="isLoadingSubnet[name]">Saving...</span>
                  <button @click="cancelEditNetwork(name, 'subnet')">Cancel</button>
                </td>
              </template>
              <template v-else>
                <td>{{ interfaceData.subnet || 'N/A' }}
                  <button @click="editSubnet(name)">Edit</button>
                </td>
              </template>
            </tr>
            <tr>
              <th>Gateway:</th>
              <template v-if="isEditingGateway[name]">
                <td>
                  <input v-model="interfaceData.gateway" />
                  <button @click="saveNetworkSetting(name, 'gateway')" :disabled="isLoadingGateway[name]">Save</button>
                  <span v-if="isLoadingGateway[name]">Saving...</span>
                  <button @click="cancelEditNetwork(name, 'gateway')">Cancel</button>
                </td>
              </template>
              <template v-else>
                <td>{{ interfaceData.gateway || 'N/A' }}
                  <button @click="editGateway(name)">Edit</button>
                </td>
              </template>
            </tr>
            <tr>
              <th>DNS Servers:</th>
              <template v-if="isEditingDns[name]">
                <td>
                  <input v-model="interfaceData.dns_servers" />
                  <button @click="saveNetworkSetting(name, 'dns')" :disabled="isLoadingDns[name]">Save</button>
                  <span v-if="isLoadingDns[name]">Saving...</span>
                  <button @click="cancelEditNetwork(name, 'dns')">Cancel</button>
                </td>
              </template>
              <template v-else>
                <td>{{ interfaceData.dns_servers.join(', ') }}
                  <button @click="editDns(name)">Edit</button>
                </td>
              </template>
            </tr>
          </template>
        </table>
      </template>
      <template v-else>
        <table>
          <tr>
            <th>IP Address:</th>
            <td>
              {{ interfaceData.ip || 'N/A' }}
              <button @click="copyToClipboard(interfaceData.ip, $event)" class="copy-btn">Copy</button>
              <button @click="editIp(name)">Edit</button>
            </td>
          </tr>
          <tr>
            <th>MAC Address:</th>
            <td>{{ interfaceData.mac_address || 'N/A' }}</td>
          </tr>
          <template v-if="interfaceData">
            <tr>
              <th>Subnet Mask:</th>
              <template v-if="isEditingSubnet[name]">
                <td>
                  <input v-model="interfaceData.subnet" />
                  <button @click="saveNetworkSetting(name, 'subnet')" :disabled="isLoadingSubnet[name]">Save</button>
                  <span v-if="isLoadingSubnet[name]">Saving...</span>
                  <button @click="cancelEditNetwork(name, 'subnet')">Cancel</button>
                </td>
              </template>
              <template v-else>
                <td>{{ interfaceData.subnet || 'N/A' }}
                  <button @click="editSubnet(name)">Edit</button>
                </td>
              </template>
            </tr>
            <tr>
              <th>Gateway:</th>
              <template v-if="isEditingGateway[name]">
                <td>
                  <input v-model="interfaceData.gateway" />
                  <button @click="saveNetworkSetting(name, 'gateway')" :disabled="isLoadingGateway[name]">Save</button>
                  <span v-if="isLoadingGateway[name]">Saving...</span>
                  <button @click="cancelEditNetwork(name, 'gateway')">Cancel</button>
                </td>
              </template>
              <template v-else>
                <td>{{ interfaceData.gateway || 'N/A' }}
                  <button @click="editGateway(name)">Edit</button>
                </td>
              </template>
            </tr>
            <tr>
              <th>DNS Servers:</th>
              <template v-if="isEditingDns[name]">
                <td>
                  <input v-model="interfaceData.dns_servers" />
                  <button @click="saveNetworkSetting(name, 'dns')" :disabled="isLoadingDns[name]">Save</button>
                  <span v-if="isLoadingDns[name]">Saving...</span>
                  <button @click="cancelEditNetwork(name, 'dns')">Cancel</button>
                </td>
              </template>
              <template v-else>
                <td>{{ interfaceData.dns_servers.join(', ') }}
                  <button @click="editDns(name)">Edit</button>
                </td>
              </template>
            </tr>
          </template>
        </table>
      </template>
    </section>

    <!-- 削除確認モーダル -->
    <div v-if="showDeleteModal" class="modal-overlay">
      <div class="modal">
        <h3>Confirm Delete</h3>
        <p>Are you sure you want to delete interface "{{ interfaceToDelete }}"?</p>
        <div class="modal-buttons">
          <button @click="deleteInterface" :disabled="isDeletingInterface" class="confirm-delete-btn">
            {{ isDeletingInterface ? 'Deleting...' : 'Delete' }}
          </button>
          <button @click="cancelDelete" class="cancel-btn">Cancel</button>
        </div>
        <span v-if="deleteError" class="error">{{ deleteError }}</span>
      </div>
    </div>

    <!-- 新しいインターフェイス追加セクション -->
    <section>
      <h2>Add New Interface</h2>
      <table>
        <tr>
          <th>Name:</th>
          <td>
            <input v-model="newInterface.name" placeholder="eth0" />
          </td>
        </tr>
        <tr>
          <th>IP Address:</th>
          <td>
            <input v-model="newInterface.ip_address" placeholder="192.168.1.100" />
          </td>
        </tr>
        <tr>
          <th>Subnet Mask:</th>
          <td>
            <input v-model="newInterface.subnet_mask" placeholder="255.255.255.0" />
          </td>
        </tr>
        <tr>
          <th>Gateway:</th>
          <td>
            <input v-model="newInterface.gateway" placeholder="192.168.1.1" />
          </td>
        </tr>
        <tr>
          <th>DNS Servers (comma separated):</th>
          <td>
            <input v-model="newInterface.dns_servers" placeholder="8.8.8.8, 8.8.4.4" />
          </td>
        </tr>
        <tr>
          <th>MAC Address:</th>
          <td>
            <input v-model="newInterface.mac_address" placeholder="00:1A:2B:3C:4D:5E" />
          </td>
        </tr>
        <tr>
          <td colspan="2">
            <button @click="addNewInterface" :disabled="isAddingInterface"
              class="action-btn">
              {{ isAddingInterface ? 'Adding...' : 'Add Interface' }}
            </button>
            <span v-if="addInterfaceError" class="error">{{ addInterfaceError }}</span>
          </td>
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
const isEditingMemo = ref(false);
const isUpdatingMemo = ref(false); // New loading state for memo
const isEditingIp = ref({});
const isLoadingIp = ref({}); // Loading state for IP
const isEditingSubnet = ref({});
const isLoadingSubnet = ref({}); // Loading state for subnet
const isEditingGateway = ref({});
const isLoadingGateway = ref({}); // Loading state for gateway
const isEditingDns = ref({});
const isLoadingDns = ref({}); // Loading state for DNS
const isEditingName = ref({}); // New state for editing interface name
const isLoadingName = ref({}); // New loading state for interface name
const isEditingParentId = ref(false);
const isUpdatingParentId = ref(false);

const isAddingInterface = ref(false);
const addInterfaceError = ref('');
const newInterface = ref({
  name: '',
  ip_address: '',
  subnet_mask: '',
  gateway: '',
  dns_servers: '',
  mac_address: ''
});

// 状態変数の追加
const isEditingVmStatus = ref(false);
const isUpdatingVmStatus = ref(false);
const vmStatusEdit = ref({
  is_virtual: false,
  parent_machine_id: null
});

// 削除関連の新しいref
const showDeleteModal = ref(false);
const interfaceToDelete = ref('');
const isDeletingInterface = ref(false);
const deleteError = ref('');

// インターフェイス削除を確認
function confirmDeleteInterface(interfaceName) {
  interfaceToDelete.value = interfaceName;
  showDeleteModal.value = true;
  deleteError.value = '';
}

// 削除をキャンセル
function cancelDelete() {
  showDeleteModal.value = false;
  interfaceToDelete.value = '';
  deleteError.value = '';
}

function enableEditVmStatus() {
  vmStatusEdit.value = {
    is_virtual: machine.value.is_virtual,
    parent_machine_id: machine.value.parent_machine_id
  };
  isEditingVmStatus.value = true;
}

function cancelEditVmStatus() {
  isEditingVmStatus.value = false;
}

function handleVmStatusChange() {
  if (!vmStatusEdit.value.is_virtual) {
    vmStatusEdit.value.parent_machine_id = null;
  }
}

async function saveVmStatus() {
  isUpdatingVmStatus.value = true;

  try {
    // If parent ID is empty string, set is_virtual to false
    if (vmStatusEdit.value.parent_machine_id === '') {
      vmStatusEdit.value.is_virtual = false;
      vmStatusEdit.value.parent_machine_id = null;
    }

    const response = await fetch(`http://localhost:3001/api/machines/${machine.value.id}/update-vm-status`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        is_virtual: vmStatusEdit.value.is_virtual,
        parent_machine_id: vmStatusEdit.value.parent_machine_id
      })
    });

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to update VM status');
    }

    // 更新後のデータを取得
    const updatedResponse = await fetch(`http://localhost:3001/api/machines/${machine.value.id}`);
    if (updatedResponse.ok) {
      machine.value = await updatedResponse.json();
      isEditingVmStatus.value = false;
      
      // 親マシンのホスト名を更新
      if (machine.value.is_virtual && machine.value.parent_machine_id) {
        const parentResponse = await fetch(`http://localhost:3001/api/machines/${machine.value.parent_machine_id}`);
        if (parentResponse.ok) {
          const parentMachine = await parentResponse.json();
          machine.value.parentHostname = parentMachine.hostname;
        }
      } else {
        machine.value.parentHostname = null;
      }
    }
  } catch (err) {
    console.error(err);
    alert(err.message);
  } finally {
    isUpdatingVmStatus.value = false;
  }
}

// インターフェイスを削除
async function deleteInterface() {
  isDeletingInterface.value = true;
  deleteError.value = '';

  try {
    const response = await fetch(
      `http://localhost:3001/api/machines/${machine.value.id}/interfaces/${interfaceToDelete.value}`,
      {
        method: 'DELETE'
      }
    );

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to delete interface');
    }

    // 成功したらモーダルを閉じ、マシンデータを再読み込み
    showDeleteModal.value = false;
    interfaceToDelete.value = '';

    // マシンデータを再読み込み
    const updatedResponse = await fetch(`http://localhost:3001/api/machines/${route.params.id}`);
    if (updatedResponse.ok) {
      machine.value = await updatedResponse.json();
    }
  } catch (err) {
    deleteError.value = err.message;
  } finally {
    isDeletingInterface.value = false;
  }
}

// 新しいインターフェイスを追加する関数
async function addNewInterface() {
  if (!newInterface.value.name || !newInterface.value.ip_address) {
    addInterfaceError.value = 'Name and IP address are required';
    return;
  }

  isAddingInterface.value = true;
  addInterfaceError.value = '';

  try {
    const response = await fetch(`http://localhost:3001/api/machines/${machine.value.id}/interfaces`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: newInterface.value.name,
        ip_address: newInterface.value.ip_address,
        subnet_mask: newInterface.value.subnet_mask,
        gateway: newInterface.value.gateway,
        dns_servers: newInterface.value.dns_servers.split(',').map(s => s.trim()),
        mac_address: newInterface.value.mac_address
      })
    });

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to add interface');
    }

    // 成功したらフォームをリセットし、マシンデータを再読み込み
    newInterface.value = {
      name: '',
      ip_address: '',
      subnet_mask: '',
      gateway: '',
      dns_servers: '',
      mac_address: ''
    };

    // マシンデータを再読み込み
    const updatedResponse = await fetch(`http://localhost:3001/api/machines/${route.params.id}`);
    if (updatedResponse.ok) {
      machine.value = await updatedResponse.json();
    }
  } catch (err) {
    addInterfaceError.value = err.message;
  } finally {
    isAddingInterface.value = false;
  }
}

function editIp(interfaceName) {
  isEditingIp.value[interfaceName] = true;
}

function cancelEditIp(interfaceName) {
  isEditingIp.value[interfaceName] = false;

  // Reload current values from server
  fetch(`http://localhost:3001/api/machines/${route.params.id}`)
    .then(response => response.json())
    .then(data => {
      machine.value.interfaces[interfaceName] = data.interfaces[interfaceName];
    })
    .catch(err => console.error(err));
}

function enableEditParentId() {
  isEditingParentId.value = true;
}

function cancelEditParentId() {
  isEditingParentId.value = false;
}

async function updateParentId() {
  isUpdatingParentId.value = true;

  try {
    const response = await fetch(`http://localhost:3001/api/machines/${machine.value.id}/update-parent-id`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        parent_machine_id: machine.value.parent_machine_id || null
      })
    });

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to update parent machine ID');
    }

    isEditingParentId.value = false;
    
    // 親マシンのホスト名を再取得
    if (machine.value.parent_machine_id) {
      const parentResponse = await fetch(`http://localhost:3001/api/machines/${machine.value.parent_machine_id}`);
      if (parentResponse.ok) {
        const parentMachine = await parentResponse.json();
        machine.value.parentHostname = parentMachine.hostname;
      }
    } else {
      machine.value.parentHostname = null;
    }
  } catch (err) {
    console.error(err);
    alert(err.message);
  } finally {
    isUpdatingParentId.value = false;
  }
}

function editSubnet(interfaceName) {
  isEditingSubnet.value[interfaceName] = true;
}

function editGateway(interfaceName) {
  isEditingGateway.value[interfaceName] = true;
}

function editDns(interfaceName) {
  isEditingDns.value[interfaceName] = true;
}

function editInterfaceName(interfaceName) {
  // Ensure the interface name is properly set before entering edit mode
  if (!machine.value.interfaces[interfaceName].name) {
    machine.value.interfaces[interfaceName].name = interfaceName; // Use the key as fallback
  }
  isEditingName.value[interfaceName] = true;
}

async function saveNetworkSetting(interfaceName, settingType) {
  const interfaceData = machine.value.interfaces[interfaceName];
  let url = '';
  let body = {};

  // Set loading state to true for the specific setting
  if (settingType === 'ip') {
    isLoadingIp.value[interfaceName] = true;
  } else if (settingType === 'subnet') {
    isLoadingSubnet.value[interfaceName] = true;
  } else if (settingType === 'gateway') {
    isLoadingGateway.value[interfaceName] = true;
  } else if (settingType === 'dns') {
    isLoadingDns.value[interfaceName] = true;
  }

  try {
    // Set up request based on setting type
    if (settingType === 'ip') {
      // Update IP address
      if (!interfaceData.ip) {
        alert('IP address cannot be empty');
        return;
      }

      url = `http://localhost:3001/api/interfaces/${machine.value.id}/${interfaceName}`;
      body = { ip_address: interfaceData.ip };

    } else if (settingType === 'subnet') {
      // Update subnet mask
      if (!interfaceData.subnet) {
        alert('Subnet mask cannot be empty');
        return;
      }

      url = `http://localhost:3001/api/interfaces/${machine.value.id}/${interfaceName}/update-subnet-mask`;
      body = { subnet_mask: interfaceData.subnet };

    } else if (settingType === 'gateway') {
      // Update gateway - Allow empty values
      url = `http://localhost:3001/api/interfaces/${machine.value.id}/${interfaceName}/update-gateway`;
      body = { gateway: interfaceData.gateway };

    } else if (settingType === 'dns') {
      // Update DNS servers - Allow empty values
      let dnsServersArray;
      if (typeof interfaceData.dns_servers === 'string' && interfaceData.dns_servers.trim()) {
        // Convert comma-separated string to array of trimmed values
        dnsServersArray = interfaceData.dns_servers.split(',').map(s => s.trim());
      } else if (Array.isArray(interfaceData.dns_servers)) {
        dnsServersArray = interfaceData.dns_servers;
      } else {
        // Allow empty array for blank DNS servers
        dnsServersArray = [];
      }

      url = `http://localhost:3001/api/interfaces/${machine.value.id}/${interfaceName}/update-dns`;
      body = { dns_servers: dnsServersArray };
    }

    const response = await fetch(url, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });

    if (!response.ok) {
      const errorData = await response.json();
      alert(`Failed to update ${settingType}: ${errorData.error || 'Unknown error'}`);
      return;
    }

    // Reset editing and loading states based on setting type
    if (settingType === 'ip') {
      isEditingIp.value[interfaceName] = false;
      isLoadingIp.value[interfaceName] = false;
    } else if (settingType === 'subnet') {
      isEditingSubnet.value[interfaceName] = false;
      isLoadingSubnet.value[interfaceName] = false;
    } else if (settingType === 'gateway') {
      isEditingGateway.value[interfaceName] = false;
      isLoadingGateway.value[interfaceName] = false;
    } else if (settingType === 'dns') {
      isEditingDns.value[interfaceName] = false;
      isLoadingDns.value[interfaceName] = false;
    }

  } catch (err) {
    console.error(err);
    alert(`An error occurred while updating the ${settingType}`);

    // Reset loading state on error
    if (settingType === 'ip') {
      isLoadingIp.value[interfaceName] = false;
    } else if (settingType === 'subnet') {
      isLoadingSubnet.value[interfaceName] = false;
    } else if (settingType === 'gateway') {
      isLoadingGateway.value[interfaceName] = false;
    } else if (settingType === 'dns') {
      isLoadingDns.value[interfaceName] = false;
    }
  }
}

async function saveInterfaceName(interfaceName) { // New function to save interface name
  const newName = machine.value.interfaces[interfaceName].name;

  // Set loading state to true
  isLoadingName.value[interfaceName] = true;

  try {
    const response = await fetch(`http://localhost:3001/api/interfaces/${machine.value.id}/${interfaceName}/update-name`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: newName })
    });

    if (!response.ok) {
      const errorData = await response.json();
      alert(`Failed to update interface name: ${errorData.error || 'Unknown error'}`);
      return;
    }

    // Reset editing and loading states
    isEditingName.value[interfaceName] = false;
    isLoadingName.value[interfaceName] = false;

    // Reload machine data after successful update
    const updatedResponse = await fetch(`http://localhost:3001/api/machines/${route.params.id}`);
    if (updatedResponse.ok) {
      machine.value = await updatedResponse.json();
    } else {
      console.error('Failed to reload machine details');
    }

  } catch (err) {
    console.error(err);
    alert('An error occurred while updating the interface name');

    // Reset loading state on error
    isLoadingName.value[interfaceName] = false;
  }
}

function cancelEditNetwork(interfaceName, settingType) {
  if (settingType === 'subnet') {
    isEditingSubnet.value[interfaceName] = false;
    isLoadingSubnet.value[interfaceName] = false; // Reset loading state
  } else if (settingType === 'gateway') {
    isEditingGateway.value[interfaceName] = false;
    isLoadingGateway.value[interfaceName] = false; // Reset loading state
  } else if (settingType === 'dns') {
    isEditingDns.value[interfaceName] = false;
    isLoadingDns.value[interfaceName] = false; // Reset loading state
  }

  // Reload current values from server
  fetch(`http://localhost:3001/api/machines/${route.params.id}`)
    .then(response => response.json())
    .then(data => {
      machine.value.interfaces[interfaceName] = data.interfaces[interfaceName];
    })
    .catch(err => console.error(err));
}

function cancelEditInterfaceName(interfaceName) { // New function to cancel editing interface name
  isEditingName.value[interfaceName] = false;

  // Reload current values from server
  fetch(`http://localhost:3001/api/machines/${route.params.id}`)
    .then(response => response.json())
    .then(data => {
      machine.value.interfaces[interfaceName] = data.interfaces[interfaceName];
    })
    .catch(err => console.error(err));
}

onMounted(async () => {
  const response = await fetch(`http://localhost:3001/api/machines/${route.params.id}`);
  if (response.ok) {
    machine.value = await response.json();

    // If this is a virtual machine with a parent, fetch the parent machine details
    if (machine.value.is_virtual && machine.value.parent_machine_id) {
      const parentResponse = await fetch(`http://localhost:3001/api/machines/${machine.value.parent_machine_id}`);
      if (parentResponse.ok) {
        const parentMachine = await parentResponse.json();
        // Store the parent hostname to display in the link
        machine.value.parentHostname = parentMachine.hostname;
      } else {
        console.error('Failed to load parent machine details');
      }
    }
  } else {
    alert('Failed to load machine details');
  }
});

async function updateMemo() {
  isUpdatingMemo.value = true; // Set loading state

  const response = await fetch(`http://localhost:3001/api/machines/${machine.value.id}/update-memo`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ memo: machine.value.memo })
  });

  isUpdatingMemo.value = false; // Reset loading state

  if (response.ok) {
    isEditingMemo.value = false;
    alert('Memo updated successfully');
  } else {
    const errorData = await response.json();
    alert(`Failed to update memo: ${errorData.error || 'Unknown error'}`);
  }
}

function enableEditMemo() {
  isEditingMemo.value = true;
}

function cancelEditMemo() {
  isEditingMemo.value = false;
}

function copyToClipboard(id, event) {
  const button = event.target;
  navigator.clipboard.writeText(id).then(() => {
    button.innerText = 'Copied!';
    setTimeout(() => {
      button.innerText = 'Copy';
    }, 2000);
  }).catch(err => {
    console.error('Could not copy text: ', err);
  });
}
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

.copy-btn {
  background-color: #007bff;
  color: white;
  border: none;
  padding: 0.25rem 0.5rem;
  cursor: pointer;
  margin-left: 0.5rem;
  border-radius: 4px;
}

.copy-btn:hover {
  background-color: #0069d9;
}

/* モーダルスタイル */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 1000;
}

.modal {
  background-color: white;
  padding: 2rem;
  border-radius: 8px;
  max-width: 500px;
  width: 90%;
}

.modal h3 {
  margin-top: 0;
  color: #dc3545;
}

.modal p {
  margin-bottom: 1.5rem;
}

.modal-buttons {
  display: flex;
  gap: 1rem;
  justify-content: flex-end;
}

.confirm-delete-btn {
  background-color: #dc3545;
  color: white;
  border: none;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  cursor: pointer;
}

.confirm-delete-btn:hover {
  background-color: #c82333;
}

.confirm-delete-btn:disabled {
  background-color: #6c757d;
  cursor: not-allowed;
}

.cancel-btn {
  background-color: #6c757d;
  color: white;
  border: none;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  cursor: pointer;
}

.cancel-btn:hover {
  background-color: #5a6268;
}

.error {
  color: #dc3545;
  margin-top: 1rem;
  display: block;
}
</style>

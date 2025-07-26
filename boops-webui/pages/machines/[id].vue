<template>
  <v-container class="mt-6">
    <v-card v-if="machine" class="pa-6">
      <!-- メインヘッダーと操作ボタン -->
      <v-card-title class="d-flex justify-space-between align-center mb-6">
        <h1 class="text-h4">{{ machine.hostname }} Details</h1>
        <div>
          <v-btn color="primary" @click="duplicateMachine" class="mr-2">
            <v-icon start>mdi-content-copy</v-icon>
            Duplicate
          </v-btn>
          <v-btn color="error" @click="confirmMachineDelete">
            <v-icon start>mdi-delete</v-icon>
            Delete
          </v-btn>
        </div>
      </v-card-title>

      <!-- メイン情報セクション -->
      <v-card-text>
        <v-sheet class="mb-8">
          <h2 class="text-h5 mb-4">Main Information</h2>
          <v-table class="elevation-1">
            <tbody>
              <tr>
                <th width="20%">ID:</th>
                <td width="80%">
                  {{ machine.id }}
                  <v-btn 
                    icon 
                    variant="text" 
                    size="small" 
                    @click="copyToClipboard(machine.id, 'machine-id')"
                    class="ml-2"
                  >
                    <v-icon>
                      {{ copiedItems['machine-id'] ? 'mdi-check' : 'mdi-content-copy' }}
                    </v-icon>
                  </v-btn>
                </td>
              </tr>
              <tr>
                <th>Hostname:</th>
                <td>
                  <template v-if="isEditingHostname">
                    <div class="d-flex align-center">
                      <v-text-field v-model="machine.hostname" density="compact" hide-details class="mr-2" />
                      <v-btn color="success" icon size="small" @click="saveHostname" :loading="isUpdatingHostname" class="mr-1">
                        <v-icon>mdi-check</v-icon>
                      </v-btn>
                      <v-btn color="error" icon size="small" @click="cancelEditHostname">
                        <v-icon>mdi-close</v-icon>
                      </v-btn>
                    </div>
                  </template>
                  <template v-else>
                    {{ machine.hostname }}
                    <v-btn 
                      icon 
                      variant="text" 
                      size="small" 
                      @click="copyToClipboard(machine.hostname, 'hostname')"
                      class="ml-1 mr-1"
                    >
                      <v-icon>
                        {{ copiedItems['hostname'] ? 'mdi-check' : 'mdi-content-copy' }}
                      </v-icon>
                    </v-btn>
                    <v-btn icon variant="text" size="small" @click="enableEditHostname">
                      <v-icon>mdi-pencil</v-icon>
                    </v-btn>
                  </template>
                </td>
              </tr>
              <tr>
                <th>OS:</th>
                <td>{{ machine.os_name || 'N/A' }}</td>
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
                <td>
                  <template v-if="isEditingVmStatus">
                    <div>
                      <v-checkbox v-model="vmStatusEdit.is_virtual" label="Is Virtual Machine" hide-details density="compact" @change="handleVmStatusChange" />
                      <div v-if="vmStatusEdit.is_virtual" class="mt-2">
                        <v-text-field v-model="vmStatusEdit.parent_machine_id" label="Parent Machine ID" density="compact" hide-details />
                      </div>
                      <div class="mt-2">
                        <v-btn color="success" size="small" @click="saveVmStatus" :loading="isUpdatingVmStatus" class="mr-2">
                          Save
                        </v-btn>
                        <v-btn color="error" size="small" @click="cancelEditVmStatus">
                          Cancel
                        </v-btn>
                      </div>
                    </div>
                  </template>
                  <template v-else>
                    {{ machine.is_virtual ? 'Yes' : 'No' }}
                    <v-btn icon variant="text" size="small" @click="enableEditVmStatus" class="ml-1">
                      <v-icon>mdi-pencil</v-icon>
                    </v-btn>
                  </template>
                </td>
              </tr>
              <tr>
                <th>Purpose:</th>
                <td>
                  <template v-if="isEditingPurpose">
                    <div class="d-flex align-center">
                      <v-text-field v-model="machine.purpose" density="compact" hide-details class="mr-2" />
                      <v-btn color="success" icon size="small" @click="updatePurpose" :loading="isUpdatingPurpose" class="mr-1">
                        <v-icon>mdi-check</v-icon>
                      </v-btn>
                      <v-btn color="error" icon size="small" @click="cancelEditPurpose">
                        <v-icon>mdi-close</v-icon>
                      </v-btn>
                    </div>
                  </template>
                  <template v-else>
                    {{ machine.purpose || 'N/A' }}
                    <v-btn icon variant="text" size="small" @click="enableEditPurpose" class="ml-1">
                      <v-icon>mdi-pencil</v-icon>
                    </v-btn>
                  </template>
                </td>
              </tr>
              <tr v-if="machine.is_virtual">
                <th>Parent Machine ID:</th>
                <td>
                  <template v-if="isEditingParentId">
                    <div class="d-flex align-center">
                      <v-text-field v-model="machine.parent_machine_id" density="compact" hide-details class="mr-2" />
                      <v-btn color="success" icon size="small" @click="updateParentId" :loading="isUpdatingParentId" class="mr-1">
                        <v-icon>mdi-check</v-icon>
                      </v-btn>
                      <v-btn color="error" icon size="small" @click="cancelEditParentId">
                        <v-icon>mdi-close</v-icon>
                      </v-btn>
                    </div>
                  </template>
                  <template v-else>
                    {{ machine.parent_machine_id || 'N/A' }}
                    <!-- <v-btn icon variant="text" size="small" @click="copyToClipboard(machine.parent_machine_id)" class="ml-1 mr-1">
                      <v-icon>mdi-content-copy</v-icon>
                    </v-btn> -->
                    <v-btn 
                      icon 
                      variant="text" 
                      size="small" 
                      @click="copyToClipboard(machine.parent_machine_id, 'parent_machine-id')"
                      class="ml-2"
                    >
                      <v-icon>
                        {{ copiedItems['parent_machine-id'] ? 'mdi-check' : 'mdi-content-copy' }}
                      </v-icon>
                    </v-btn>

                  </template>
                </td>
              </tr>
              <tr v-if="machine.is_virtual && machine.parent_machine_id">
                <th>Parent Machine:</th>
                <td>
                  <v-btn variant="text" :to="`/machines/${machine.parent_machine_id}`" color="primary">
                    {{ machine.parentHostname || 'Unknown' }}
                    <v-icon end>mdi-open-in-new</v-icon>
                  </v-btn>
                </td>
              </tr>
              <tr>
                <th>Memo:</th>
                <td>
                  <template v-if="isEditingMemo">
                    <div>
                      <v-textarea v-model="machine.memo" rows="3" density="compact" hide-details />
                      <div class="mt-2">
                        <v-btn color="success" size="small" @click="updateMemo" :loading="isUpdatingMemo" class="mr-2">
                          Save
                        </v-btn>
                        <v-btn color="error" size="small" @click="cancelEditMemo">
                          Cancel
                        </v-btn>
                      </div>
                    </div>
                  </template>
                  <template v-else>
                    {{ machine.memo || 'N/A' }}
                    <v-btn icon variant="text" size="small" @click="enableEditMemo" class="ml-1">
                      <v-icon>mdi-pencil</v-icon>
                    </v-btn>
                  </template>
                </td>
              </tr>
            </tbody>
          </v-table>
        </v-sheet>

        <!-- インターフェースセクション -->
        <v-sheet v-for="(interfaceData, index) in machine.interfaces" :key="interfaceData.id" class="mb-8">
          <div class="d-flex justify-space-between align-center mb-4">
            <h2 class="text-h5">
              <template v-if="isEditingName[interfaceData.id]">
                <v-text-field v-model="interfaceData.name" density="compact" hide-details class="d-inline-block" style="width: 200px;" />
              </template>
              <template v-else>
                Interface: {{ interfaceData.name }}
              </template>
            </h2>
            <div>
              <template v-if="isEditingName[interfaceData.id]">
                <v-btn color="success" icon size="small" @click="saveInterfaceName(interfaceData.id)" :loading="isLoadingName[interfaceData.id]" class="mr-1">
                  <v-icon>mdi-check</v-icon>
                </v-btn>
                <v-btn color="error" icon size="small" @click="cancelEditInterfaceName(interfaceData.id)">
                  <v-icon>mdi-close</v-icon>
                </v-btn>
              </template>
              <template v-else>
                <v-btn icon variant="text" size="small" @click="editInterfaceName(interfaceData.id)" class="mr-1">
                  <v-icon>mdi-pencil</v-icon>
                </v-btn>
                <v-btn icon variant="text" size="small" color="error" @click="confirmDeleteInterface(interfaceData.id, interfaceData.name)">
                  <v-icon>mdi-delete</v-icon>
                </v-btn>
              </template>
            </div>
          </div>

          <!-- IPアドレステーブル -->
          <v-sheet class="mb-4">
            <div class="d-flex justify-space-between align-center mb-2">
              <h3 class="text-h6">IP Addresses</h3>
              <v-btn color="primary" size="small" @click="prepareAddIp(interfaceData)">
                <v-icon start>mdi-plus</v-icon>
                Add IP
              </v-btn>
            </div>
            <v-table class="elevation-1">
              <thead>
                <tr>
                  <th width="15%">IP Address</th>
                  <th width="15%">Subnet Mask</th>
                  <th width="10%">Actions</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(ipData, ipIndex) in interfaceData.ips" :key="ipIndex">
                  <td>
                    <template v-if="isEditingIp[ipData.id]">
                      <v-text-field v-model="ipData.ip_address" density="compact" hide-details />
                    </template>
                    <template v-else>
                      {{ ipData.ip_address || 'N/A' }}
                    </template>
                  </td>
                  <td>
                    <template v-if="isEditingSubnet[ipData.id]">
                      <v-text-field v-model="ipData.subnet_mask" density="compact" hide-details />
                    </template>
                    <template v-else>
                      {{ ipData.subnet_mask || 'N/A' }}
                    </template>
                  </td>
                  <td>
                    <template v-if="isEditingIp[ipData.id] || isEditingSubnet[ipData.id]">
                      <v-btn color="success" icon size="small" @click="saveIpAddress(interfaceData.id, ipData.id, ipData.ip_address)" :loading="isLoadingIp[ipData.id]" class="mr-1">
                        <v-icon>mdi-check</v-icon>
                      </v-btn>
                      <v-btn color="error" icon size="small" @click="cancelEditIp(ipData.id)">
                        <v-icon>mdi-close</v-icon>
                      </v-btn>
                    </template>
                    <template v-else>
        <v-btn
          icon
          variant="text"
          size="small"
          @click="copyToClipboard(ipData.ip_address, 'ip-' + interfaceData.id + '-' + ipData.id)"
          class="mr-1"
        >
          <v-icon>
            {{ copiedItems['ip-' + interfaceData.id + '-' + ipData.id] ? 'mdi-check' : 'mdi-content-copy' }}
          </v-icon>
        </v-btn>
                      <v-btn icon variant="text" size="small" @click="editIp(ipData.id)" class="mr-1">
                        <v-icon>mdi-pencil</v-icon>
                      </v-btn>
                      <v-btn icon variant="text" size="small" color="error" @click="confirmDeleteIp(interfaceData.id, ipData.id, ipData.ip_address)">
                        <v-icon>mdi-delete</v-icon>
                      </v-btn>
                    </template>
                  </td>
                </tr>
              </tbody>
            </v-table>
          </v-sheet>

          <!-- インターフェース詳細 -->
          <v-table class="elevation-1">
            <tbody>
              <tr>
                <th width="20%">MAC Address:</th>
                <td width="80%">{{ interfaceData.mac_address || 'N/A' }}</td>
              </tr>
              <tr>
                <th>Gateway:</th>
                <td>
                  <template v-if="isEditingGateway[interfaceData.id]">
                    <div class="d-flex align-center">
                      <v-text-field v-model="interfaceData.gateway" density="compact" hide-details class="mr-2" />
                      <v-btn color="success" icon size="small" @click="saveGateway(interfaceData.id, interfaceData.gateway)" :loading="isLoadingGateway[interfaceData.id]" class="mr-1">
                        <v-icon>mdi-check</v-icon>
                      </v-btn>
                      <v-btn color="error" icon size="small" @click="cancelEditGateway(interfaceData.id)">
                        <v-icon>mdi-close</v-icon>
                      </v-btn>
                    </div>
                  </template>
                  <template v-else>
                    {{ interfaceData.gateway || 'N/A' }}
                    <v-btn icon variant="text" size="small" @click="editGateway(interfaceData.id)" class="ml-1">
                      <v-icon>mdi-pencil</v-icon>
                    </v-btn>
                  </template>
                </td>
              </tr>
              <tr>
                <th>DNS Servers:</th>
                <td>
                  <template v-if="isEditingDns[interfaceData.id]">
                    <div class="d-flex align-center">
                      <v-text-field v-model="interfaceData.dns_servers" density="compact" hide-details class="mr-2" />
                      <v-btn color="success" icon size="small" @click="saveDnsServers(interfaceData.id, interfaceData.dns_servers)" :loading="isLoadingDns[interfaceData.id]" class="mr-1">
                        <v-icon>mdi-check</v-icon>
                      </v-btn>
                      <v-btn color="error" icon size="small" @click="cancelEditDns(interfaceData.id)">
                        <v-icon>mdi-close</v-icon>
                      </v-btn>
                    </div>
                  </template>
                  <template v-else>
                    {{ interfaceData.dns_servers || 'N/A' }}
                    <v-btn icon variant="text" size="small" @click="editDns(interfaceData.id)" class="ml-1">
                      <v-icon>mdi-pencil</v-icon>
                    </v-btn>
                  </template>
                </td>
              </tr>
            </tbody>
          </v-table>
        </v-sheet>

        <!-- 新しいインターフェース追加 -->
        <v-sheet class="mt-8">
          <h2 class="text-h5 mb-4">Add New Interface</h2>
          <v-form @submit.prevent="addNewInterface">
            <v-table>
              <tbody>
                <tr>
                  <th width="20%">Name:</th>
                  <td width="80%">
                    <v-text-field v-model="newInterface.name" placeholder="eth0" density="compact" hide-details />
                  </td>
                </tr>
                <tr>
                  <th>MAC Address:</th>
                  <td>
                    <v-text-field v-model="newInterface.mac_address" placeholder="00:1A:2B:3C:4D:5E" density="compact" hide-details />
                  </td>
                </tr>
                <tr>
                  <th>Gateway:</th>
                  <td>
                    <v-text-field v-model="newInterface.gateway" placeholder="192.168.1.1" density="compact" hide-details />
                  </td>
                </tr>
                <tr>
                  <th>DNS Servers:</th>
                  <td>
                    <v-text-field v-model="newInterface.dns_servers" placeholder="8.8.8.8,8.8.4.4" density="compact" hide-details />
                  </td>
                </tr>
                <tr>
                  <td colspan="2" class="text-right pt-4">
                    <v-btn color="primary" type="submit" :loading="isAddingInterface">
                      Add Interface
                    </v-btn>
                    <v-alert v-if="addInterfaceError" type="error" density="compact" class="mt-2">
                      {{ addInterfaceError }}
                    </v-alert>
                  </td>
                </tr>
              </tbody>
            </v-table>
          </v-form>
        </v-sheet>
      </v-card-text>
    </v-card>

    <!-- IP追加モーダル -->
    <v-dialog v-model="showAddIpModal" max-width="600">
      <v-card>
        <v-card-title>Add New IP Address to {{ selectedInterfaceForNewIp.name }}</v-card-title>
        <v-card-text>
          <v-form @submit.prevent="addNewIp">
            <v-table>
              <tbody>
                <tr>
                  <th width="30%">IP Address:</th>
                  <td width="70%">
                    <v-text-field v-model="newIp.ip_address" placeholder="192.168.1.100" density="compact" hide-details />
                  </td>
                </tr>
                <tr>
                  <th>Subnet Mask:</th>
                  <td>
                    <v-text-field v-model="newIp.subnet_mask" placeholder="255.255.255.0" density="compact" hide-details />
                  </td>
                </tr>
              </tbody>
            </v-table>
            <v-alert v-if="addIpError" type="error" density="compact" class="mt-2">
              {{ addIpError }}
            </v-alert>
            <div class="d-flex justify-end mt-4">
              <v-btn color="primary" type="submit" :loading="isAddingIp" class="mr-2">
                Add IP
              </v-btn>
              <v-btn color="error" @click="cancelAddIp">
                Cancel
              </v-btn>
            </div>
          </v-form>
        </v-card-text>
      </v-card>
    </v-dialog>

    <!-- インターフェース削除確認ダイアログ -->
    <v-dialog v-model="showDeleteInterfaceModal" max-width="500">
      <v-card>
        <v-card-title>Confirm Delete</v-card-title>
        <v-card-text>
          Are you sure you want to delete interface "{{ interfaceToDeleteName }}"?
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn color="error" @click="deleteInterface" :loading="isDeletingInterface">
            Delete
          </v-btn>
          <v-btn color="secondary" @click="cancelDelete">
            Cancel
          </v-btn>
        </v-card-actions>
        <v-alert v-if="deleteError" type="error" density="compact" class="mx-4 mb-4">
          {{ deleteError }}
        </v-alert>
      </v-card>
    </v-dialog>

    <!-- IP削除確認ダイアログ -->
    <v-dialog v-model="showDeleteIpModal" max-width="500">
      <v-card>
        <v-card-title>Confirm Delete</v-card-title>
        <v-card-text>
          Are you sure you want to delete IP address "{{ ipToDelete }}"?
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn color="error" @click="deleteIp" :loading="isDeletingIp">
            Delete
          </v-btn>
          <v-btn color="secondary" @click="cancelDeleteIp">
            Cancel
          </v-btn>
        </v-card-actions>
        <v-alert v-if="deleteIpError" type="error" density="compact" class="mx-4 mb-4">
          {{ deleteIpError }}
        </v-alert>
      </v-card>
    </v-dialog>

    <!-- マシン削除確認ダイアログ -->
    <v-dialog v-model="showDeleteMachineDialog" max-width="500">
      <v-card>
        <v-card-title>Confirm Machine Deletion</v-card-title>
        <v-card-text>
          <p>本当にこのマシンを削除しますか？</p>
          <p class="text-error">この操作は元に戻せません！</p>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn color="error" @click="deleteMachine">
            削除
          </v-btn>
          <v-btn color="secondary" @click="cancelMachineDelete">
            キャンセル
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </v-container>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import { useRouter } from 'vue-router';
import { apiBaseUrl } from '@/apiConfig';

const router = useRouter();
const route = useRoute();
const machine = ref(null);
const isEditingMemo = ref(false);
const isUpdatingMemo = ref(false);
const isEditingIp = ref({});
const isLoadingIp = ref({});
const isEditingSubnet = ref({});
const isLoadingSubnet = ref({});
const isEditingGateway = ref({});
const isLoadingGateway = ref({});
const isEditingDns = ref({});
const isLoadingDns = ref({});
const isEditingName = ref({});
const isLoadingName = ref({});
const isEditingParentId = ref(false);
const isUpdatingParentId = ref(false);
const isEditingHostname = ref(false);
const isUpdatingHostname = ref(false);
const isEditingPurpose = ref(false);
const isUpdatingPurpose = ref(false);
const isAddingInterface = ref(false);
const addInterfaceError = ref('');
const newInterface = ref({
  name: '',
  gateway: '',
  dns_servers: '',
  mac_address: ''
});
const isEditingVmStatus = ref(false);
const isUpdatingVmStatus = ref(false);
const vmStatusEdit = ref({
  is_virtual: false,
  parent_machine_id: null
});
const showDeleteInterfaceModal = ref(false);
const interfaceToDelete = ref('');
const interfaceToDeleteName = ref('');
const isDeletingInterface = ref(false);
const deleteError = ref('');
const deleteDialog = ref(null);
const isDeleting = ref(false);
const showAddIpModal = ref(false);
const selectedInterfaceForNewIp = ref(null);
const newIp = ref({
  ip_address: '',
  subnet_mask: ''
});
const isAddingIp = ref(false);
const addIpError = ref('');
const showDeleteIpModal = ref(false);
const ipToDelete = ref('');
const ipToDeleteId = ref('');
const interfaceIdForIpDelete = ref('');
const isDeletingIp = ref(false);
const deleteIpError = ref('');
const copiedItems = ref({});

function confirmMachineDelete() {
  deleteDialog.value.showModal();
}

function cancelMachineDelete() {
  deleteDialog.value.close();
}

async function deleteMachine() {
  isDeleting.value = true;
  try {
    const response = await fetch(`${apiBaseUrl}/machines/${machine.value.id}`, {
      method: 'DELETE'
    });

    if (!response.ok) {
      throw new Error('Failed to delete machine');
    }

    router.push('/');
  } catch (err) {
    console.error(err);
    alert(err.message);
  } finally {
    isDeleting.value = false;
    deleteDialog.value.close();
  }
}

function confirmDeleteInterface(interfaceId, interfaceName) {
  interfaceToDelete.value = interfaceId;
  interfaceToDeleteName.value = interfaceName;
  showDeleteInterfaceModal.value = true;
  deleteError.value = '';
}

function cancelDelete() {
  showDeleteInterfaceModal.value = false;
  interfaceToDelete.value = '';
  interfaceToDeleteName.value = '';
  deleteError.value = '';
}

// IPアドレス削除関連の関数
function confirmDeleteIp(interfaceId, ipId, ipAddress) {
  interfaceIdForIpDelete.value = interfaceId;
  ipToDeleteId.value = ipId;
  ipToDelete.value = ipAddress;
  showDeleteIpModal.value = true;
  deleteIpError.value = '';
}

function cancelDeleteIp() {
  showDeleteIpModal.value = false;
  interfaceIdForIpDelete.value = '';
  ipToDeleteId.value = '';
  ipToDelete.value = '';
  deleteIpError.value = '';
}

async function deleteIp() {
  isDeletingIp.value = true;
  deleteIpError.value = '';

  try {
    const response = await fetch(
      `${apiBaseUrl}/interfaces/${interfaceIdForIpDelete.value}/ips/${ipToDeleteId.value}`,
      {
        method: 'DELETE'
      }
    );

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to delete IP address');
    }

    showDeleteIpModal.value = false;
    ipToDeleteId.value = '';
    ipToDelete.value = '';

    // マシンデータを再読み込み
    const updatedResponse = await fetch(`${apiBaseUrl}/machines/${route.params.id}`);
    if (updatedResponse.ok) {
      machine.value = await updatedResponse.json();
    }
  } catch (err) {
    deleteIpError.value = err.message;
  } finally {
    isDeletingIp.value = false;
  }
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
    if (vmStatusEdit.value.parent_machine_id === '') {
      vmStatusEdit.value.is_virtual = false;
      vmStatusEdit.value.parent_machine_id = null;
    }

    const response = await fetch(`${apiBaseUrl}/machines/${machine.value.id}/update-vm-status`, {
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

    const updatedResponse = await fetch(`${apiBaseUrl}/machines/${machine.value.id}`);
    if (updatedResponse.ok) {
      machine.value = await updatedResponse.json();
      isEditingVmStatus.value = false;
      
      if (machine.value.is_virtual && machine.value.parent_machine_id) {
        const parentResponse = await fetch(`${apiBaseUrl}/machines/${machine.value.parent_machine_id}`);
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

async function deleteInterface() {
  isDeletingInterface.value = true;
  deleteError.value = '';

  try {
    const response = await fetch(
      `${apiBaseUrl}/machines/${machine.value.id}/interfaces/${interfaceToDelete.value}`,
      {
        method: 'DELETE'
      }
    );

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to delete interface');
    }

    showDeleteInterfaceModal.value = false;
    interfaceToDelete.value = '';

    const updatedResponse = await fetch(`${apiBaseUrl}/machines/${route.params.id}`);
    if (updatedResponse.ok) {
      machine.value = await updatedResponse.json();
    }
  } catch (err) {
    deleteError.value = err.message;
  } finally {
    isDeletingInterface.value = false;
  }
}

function prepareAddIp(interfaceData) {
  selectedInterfaceForNewIp.value = interfaceData;
  newIp.value = {
    ip_address: '',
    subnet_mask: ''
  };
  showAddIpModal.value = true;
  addIpError.value = '';
}

function cancelAddIp() {
  showAddIpModal.value = false;
  selectedInterfaceForNewIp.value = null;
  newIp.value = {
    ip_address: '',
    subnet_mask: ''
  };
}

async function addNewIp() {
  if (!newIp.value.ip_address) {
    addIpError.value = 'IP address is required';
    return;
  }

  isAddingIp.value = true;
  addIpError.value = '';

  try {
    const response = await fetch(`${apiBaseUrl}/interfaces/${selectedInterfaceForNewIp.value.id}/ips`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        ip_address: newIp.value.ip_address,
        subnet_mask: newIp.value.subnet_mask
      })
    });

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to add IP address');
    }

    showAddIpModal.value = false;
    selectedInterfaceForNewIp.value = null;
    newIp.value = {
      ip_address: '',
      subnet_mask: ''
    };

    // マシンデータを再読み込み
    const updatedResponse = await fetch(`${apiBaseUrl}/machines/${route.params.id}`);
    if (updatedResponse.ok) {
      machine.value = await updatedResponse.json();
    }
  } catch (err) {
    addIpError.value = err.message;
  } finally {
    isAddingIp.value = false;
  }
}

async function addNewInterface() {
  if (!newInterface.value.name) {
    addInterfaceError.value = 'Name is required';
    return;
  }

  isAddingInterface.value = true;
  addInterfaceError.value = '';

  try {
    const response = await fetch(`${apiBaseUrl}/machines/${machine.value.id}/interfaces`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: newInterface.value.name,
        gateway: newInterface.value.gateway,
        dns_servers: newInterface.value.dns_servers,
        mac_address: newInterface.value.mac_address
      })
    });

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to add interface');
    }

    newInterface.value = {
      name: '',
      gateway: '',
      dns_servers: '',
      mac_address: ''
    };

    const updatedResponse = await fetch(`${apiBaseUrl}/machines/${route.params.id}`);
    if (updatedResponse.ok) {
      machine.value = await updatedResponse.json();
    }
  } catch (err) {
    addInterfaceError.value = err.message;
  } finally {
    isAddingInterface.value = false;
  }
}

// IPアドレス編集関連の関数
function editIp(ipId) {
  isEditingIp.value[ipId] = true;
}

function cancelEditIp(ipId) {
  isEditingIp.value[ipId] = false;
  // サーバーから最新データを再取得
  fetch(`${apiBaseUrl}/machines/${route.params.id}`)
    .then(response => response.json())
    .then(data => {
      machine.value = data;
    })
    .catch(err => console.error(err));
}


async function saveIpAddress(interfaceId, ipId, ipAddress) {
  if (!ipAddress) {
    alert('IP address cannot be empty');
    return;
  }

  isLoadingIp.value[ipId] = true;

  try {
    const response = await fetch(`${apiBaseUrl}/interfaces/${interfaceId}/ips/${ipId}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ip_address: ipAddress })
    });

    if (!response.ok) {
      const errorData = await response.json();
      alert(`Failed to update IP address: ${errorData.error || 'Unknown error'}`);
      return;
    }

    isEditingIp.value[ipId] = false;
    isLoadingIp.value[ipId] = false;

    // マシンデータを再読み込み
    const updatedResponse = await fetch(`${apiBaseUrl}/machines/${route.params.id}`);
    if (updatedResponse.ok) {
      machine.value = await updatedResponse.json();
    }
  } catch (err) {
    console.error(err);
    alert('An error occurred while updating the IP address');
    isLoadingIp.value[ipId] = false;
  }
}

// サブネットマスク編集関連の関数
function editSubnet(ipId) {
  isEditingSubnet.value[ipId] = true;
}

function cancelEditSubnet(ipId) {
  isEditingSubnet.value[ipId] = false;
  // サーバーから最新データを再取得
  fetch(`${apiBaseUrl}/machines/${route.params.id}`)
    .then(response => response.json())
    .then(data => {
      machine.value = data;
    })
    .catch(err => console.error(err));
}

async function saveSubnetMask(interfaceId, ipId, subnetMask) {
  isLoadingSubnet.value[ipId] = true;

  try {
    const response = await fetch(`${apiBaseUrl}/interfaces/${interfaceId}/ips/${ipId}/update-subnet-mask`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ subnet_mask: subnetMask })
    });

    if (!response.ok) {
      const errorData = await response.json();
      alert(`Failed to update subnet mask: ${errorData.error || 'Unknown error'}`);
      return;
    }

    isEditingSubnet.value[ipId] = false;
    isLoadingSubnet.value[ipId] = false;

    // マシンデータを再読み込み
    const updatedResponse = await fetch(`${apiBaseUrl}/machines/${route.params.id}`);
    if (updatedResponse.ok) {
      machine.value = await updatedResponse.json();
    }
  } catch (err) {
    console.error(err);
    alert('An error occurred while updating the subnet mask');
    isLoadingSubnet.value[ipId] = false;
  }
}

function editGateway(interfaceId) {
  isEditingGateway.value[interfaceId] = true;
}

function cancelEditGateway(interfaceId) {
  isEditingGateway.value[interfaceId] = false;
  fetch(`${apiBaseUrl}/machines/${route.params.id}`)
    .then(response => response.json())
    .then(data => {
      machine.value = data;
    })
    .catch(err => console.error(err));
}

async function saveGateway(interfaceId, gateway) {
  isLoadingGateway.value[interfaceId] = true;

  try {
    const response = await fetch(`${apiBaseUrl}/interfaces/${interfaceId}/update-gateway`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ gateway: gateway })
    });

    if (!response.ok) {
      const errorData = await response.json();
      alert(`Failed to update gateway: ${errorData.error || 'Unknown error'}`);
      return;
    }

    isEditingGateway.value[interfaceId] = false;
    isLoadingGateway.value[interfaceId] = false;

    const updatedResponse = await fetch(`${apiBaseUrl}/machines/${route.params.id}`);
    if (updatedResponse.ok) {
      machine.value = await updatedResponse.json();
    }
  } catch (err) {
    console.error(err);
    alert('An error occurred while updating the gateway');
    isLoadingGateway.value[interfaceId] = false;
  }
}

function editDns(interfaceId) {
  isEditingDns.value[interfaceId] = true;
}

function cancelEditDns(interfaceId) {
  isEditingDns.value[interfaceId] = false;
  fetch(`${apiBaseUrl}/machines/${route.params.id}`)
    .then(response => response.json())
    .then(data => {
      machine.value = data;
    })
    .catch(err => console.error(err));
}

async function saveDnsServers(interfaceId, dnsServers) {
  isLoadingDns.value[interfaceId] = true;

  try {
    const response = await fetch(`${apiBaseUrl}/interfaces/${interfaceId}/update-dns`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ dns_servers: dnsServers })
    });

    if (!response.ok) {
      const errorData = await response.json();
      alert(`Failed to update DNS servers: ${errorData.error || 'Unknown error'}`);
      return;
    }

    isEditingDns.value[interfaceId] = false;
    isLoadingDns.value[interfaceId] = false;

    const updatedResponse = await fetch(`${apiBaseUrl}/machines/${route.params.id}`);
    if (updatedResponse.ok) {
      machine.value = await updatedResponse.json();
    }
  } catch (err) {
    console.error(err);
    alert('An error occurred while updating the DNS servers');
    isLoadingDns.value[interfaceId] = false;
  }
}

function editInterfaceName(interfaceId) {
  isEditingName.value[interfaceId] = true;
}

async function saveInterfaceName(interfaceId) {
  const interfaceData = machine.value.interfaces.find(intf => intf.id === interfaceId);
  const newName = interfaceData.name;

  isLoadingName.value[interfaceId] = true;

  try {
    const response = await fetch(`${apiBaseUrl}/interfaces/${interfaceId}/update-name`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: newName })
    });

    if (!response.ok) {
      const errorData = await response.json();
      alert(`Failed to update interface name: ${errorData.error || 'Unknown error'}`);
      return;
    }

    isEditingName.value[interfaceId] = false;
    isLoadingName.value[interfaceId] = false;

    const updatedResponse = await fetch(`${apiBaseUrl}/machines/${route.params.id}`);
    if (updatedResponse.ok) {
      machine.value = await updatedResponse.json();
    }
  } catch (err) {
    console.error(err);
    alert('An error occurred while updating the interface name');
    isLoadingName.value[interfaceId] = false;
  }
}

function cancelEditInterfaceName(interfaceId) {
  isEditingName.value[interfaceId] = false;
  fetch(`${apiBaseUrl}/machines/${route.params.id}`)
    .then(response => response.json())
    .then(data => {
      machine.value = data;
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
    const response = await fetch(`${apiBaseUrl}/machines/${machine.value.id}/update-parent-id`, {
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
    
    if (machine.value.parent_machine_id) {
      const parentResponse = await fetch(`${apiBaseUrl}/machines/${machine.value.parent_machine_id}`);
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

onMounted(async () => {
  const response = await fetch(`${apiBaseUrl}/machines/${route.params.id}`);
  if (response.ok) {
    machine.value = await response.json();

    if (machine.value.is_virtual && machine.value.parent_machine_id) {
      const parentResponse = await fetch(`${apiBaseUrl}/machines/${machine.value.parent_machine_id}`);
      if (parentResponse.ok) {
        const parentMachine = await parentResponse.json();
        machine.value.parentHostname = parentMachine.hostname;
      }
    }
  } else {
    alert('Failed to load machine details');
  }
});

function duplicateMachine() {
  if (!machine.value) return;

  const duplicateData = {
    ...machine.value,
    hostname: `${machine.value.hostname}`,
    parent_machine_id: null
  };

  const storageKey = `duplicate_machine_${Date.now()}`;
  const storageItem = {
    data: duplicateData,
    expires: Date.now() + 600000
  };

  localStorage.setItem(storageKey, JSON.stringify(storageItem));
  window.open(`/machines/register?duplicate=${storageKey}`, '_blank');
}

async function updateMemo() {
  isUpdatingMemo.value = true;

  const response = await fetch(`${apiBaseUrl}/machines/${machine.value.id}/update-memo`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ memo: machine.value.memo })
  });

  isUpdatingMemo.value = false;

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

function enableEditHostname() {
  isEditingHostname.value = true;
}

function cancelEditHostname() {
  isEditingHostname.value = false;

  fetch(`${apiBaseUrl}/machines/${route.params.id}`)
    .then(response => response.json())
    .then(data => {
      machine.value.hostname = data.hostname;
    })
    .catch(err => console.error(err));
}

async function saveHostname() {
  isUpdatingHostname.value = true;

  const response = await fetch(`${apiBaseUrl}/machines/${machine.value.id}/update-hostname`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ hostname: machine.value.hostname })
  });

  isUpdatingHostname.value = false;

  if (response.ok) {
    isEditingHostname.value = false;
    alert('Hostname updated successfully');
  } else {
    const errorData = await response.json();
    alert(`Failed to update hostname: ${errorData.error || 'Unknown error'}`);
  }
}

function enableEditPurpose() {
  isEditingPurpose.value = true;
}

function cancelEditPurpose() {
  isEditingPurpose.value = false;

  fetch(`${apiBaseUrl}/machines/${route.params.id}`)
    .then(response => response.json())
    .then(data => {
      machine.value.purpose = data.purpose;
    })
    .catch(err => console.error(err));
}

async function updatePurpose() {
  isUpdatingPurpose.value = true;

  const response = await fetch(`${apiBaseUrl}/machines/${machine.value.id}/update-purpose`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ purpose: machine.value.purpose })
  });

  isUpdatingPurpose.value = false;

  if (response.ok) {
    isEditingPurpose.value = false;
    alert('Purpose updated successfully');
  } else {
    const errorData = await response.json();
    alert(`Failed to update purpose: ${errorData.error || 'Unknown error'}`);
  }
}

async function copyToClipboard(text, itemId) {
  if (!text || !itemId) return;

  // If already copying this specific item, skip the operation
  if (copiedItems.value[itemId]) return;

  try {
    let success = false;

    if (navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(text);
      console.log(`Copied to clipboard: ${text}`);
      success = true;
    } else {
      console.warn('navigator.clipboard.writeText is not supported in this browser');

      // Fallback for older browsers
      const textarea = document.createElement('textarea');
      textarea.value = text;
      document.body.appendChild(textarea);
      textarea.select();

      try {
        if (document.execCommand) {
          document.execCommand('copy');
          console.log(`Fallback copy successful: ${text}`);
          success = true;
        } else {
          throw new Error('Browser does not support clipboard operations');
        }
      } catch (fallbackErr) {
        console.error('Fallback copy also failed:', fallbackErr);
      } finally {
        document.body.removeChild(textarea);
      }
    }

    // Only update state if copy was successful
    if (success) {
      copiedItems.value = { ...copiedItems.value, [itemId]: true };

      // Reset only this specific item's copy state after 3 seconds
      setTimeout(() => {
        copiedItems.value = { ...copiedItems.value, [itemId]: false };
      }, 3000);
    }

  } catch (err) {
    console.error('Copy to clipboard failed:', err);
    if (copiedItems.value[itemId]) {
      copiedItems.value = { ...copiedItems.value, [itemId]: false };
    }
  }
}
</script>

<style scoped>
.container {
  max-width: 800px;
  margin: auto;
  padding: 2rem;
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
  /* background-color: #f4f4f4; */
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

.actions {
  margin-top: 2rem;
  text-align: right;
}

.delete-btn {
  background-color: #dc3545;
  color: white;
  padding: 0.5rem 1rem;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}

.delete-btn:hover {
  background-color: #c82333;
}

dialog {
  border: 1px solid #ddd;
  border-radius: 8px;
  padding: 1.5rem;
  width: 400px;
  max-width: 90%;
}

dialog::backdrop {
  background-color: rgba(0, 0, 0, 0.5);
}

.warning-text {
  color: #dc3545;
  font-weight: bold;
  margin: 1rem 0;
}

.dialog-buttons {
  display: flex;
  justify-content: flex-end;
  gap: 1rem;
}

.confirm-btn {
  background-color: #dc3545;
  color: white;
  padding: 0.5rem 1rem;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}

.confirm-btn:hover {
  background-color: #c82333;
}

.cancel-btn {
  background-color: #6c757d;
  color: white;
  padding: 0.5rem 1rem;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}

.cancel-btn:hover {
  background-color: #5a6268;
}

.duplicate-btn {
  background-color: #28a745;
  color: white;
  padding: 0.5rem 1rem;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  margin-right: 1rem;
}

.duplicate-btn:hover {
  background-color: #218838;
}

.error {
  color: #dc3545;
  margin-top: 1rem;
  display: block;
}
</style>

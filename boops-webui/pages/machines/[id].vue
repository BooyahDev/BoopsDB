<template>
  <v-container class="mt-6">
    <v-card v-if="machine" class="pa-6">
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
              Interface: {{ interfaceData.name }}
            </h2>
            <div>
              <v-btn 
                icon 
                variant="text" 
                size="small" 
                @click="prepareEditIps(interfaceData)"
                class="mr-1"
              >
                <v-icon>mdi-pencil</v-icon>
              </v-btn>
              <v-btn 
                icon 
                variant="text" 
                size="small" 
                color="error" 
                @click="confirmDeleteInterface(interfaceData.id, interfaceData.name)"
              >
                <v-icon>mdi-delete</v-icon>
              </v-btn>
            </div>
          </div>

          <!-- IPアドレステーブル -->
          <v-sheet class="mb-4">
            <h3 class="text-h6">IP Addresses</h3>
            <v-table class="elevation-1">
              <thead>
                <tr>
                  <th width="30%">IP Address</th>
                  <th width="30%">Subnet Mask</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(ipData, ipIndex) in interfaceData.ips" :key="ipIndex">
                  <td>
                    {{ ipData.ip_address || 'N/A' }}
                    <v-btn 
                      icon 
                      variant="text" 
                      size="small" 
                      @click="copyToClipboard(ipData.ip_address, 'ip-' + interfaceData.id + '-' + ipData.id)"
                      class="ml-1"
                    >
                      <v-icon>
                        {{ copiedItems['ip-' + interfaceData.id + '-' + ipData.id] ? 'mdi-check' : 'mdi-content-copy' }}
                      </v-icon>
                    </v-btn>
                  </td>
                  <td>{{ ipData.subnet_mask || '255.255.255.0' }}</td>
                </tr>
                <tr v-if="interfaceData.ips.length === 0">
                  <td colspan="2" class="text-center text-grey">No IP addresses configured</td>
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
                      <v-text-field 
                        v-model="gatewayEdit[interfaceData.id]" 
                        density="compact" 
                        hide-details 
                        class="mr-2" 
                      />
                      <v-btn 
                        color="success" 
                        icon 
                        size="small" 
                        @click="saveGateway(interfaceData.id)" 
                        :loading="isUpdatingGateway[interfaceData.id]" 
                        class="mr-1"
                      >
                        <v-icon>mdi-check</v-icon>
                      </v-btn>
                      <v-btn 
                        color="error" 
                        icon 
                        size="small" 
                        @click="cancelEditGateway(interfaceData.id)"
                      >
                        <v-icon>mdi-close</v-icon>
                      </v-btn>
                    </div>
                  </template>
                  <template v-else>
                    {{ interfaceData.gateway || 'N/A' }}
                    <v-btn 
                      icon 
                      variant="text" 
                      size="small" 
                      @click="enableEditGateway(interfaceData.id, interfaceData.gateway)" 
                      class="ml-1"
                    >
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
                      <v-text-field 
                        v-model="dnsEdit[interfaceData.id]" 
                        density="compact" 
                        hide-details 
                        class="mr-2" 
                        placeholder="8.8.8.8,8.8.4.4"
                      />
                      <v-btn 
                        color="success" 
                        icon 
                        size="small" 
                        @click="saveDnsServers(interfaceData.id)" 
                        :loading="isUpdatingDns[interfaceData.id]" 
                        class="mr-1"
                      >
                        <v-icon>mdi-check</v-icon>
                      </v-btn>
                      <v-btn 
                        color="error" 
                        icon 
                        size="small" 
                        @click="cancelEditDns(interfaceData.id)"
                      >
                        <v-icon>mdi-close</v-icon>
                      </v-btn>
                    </div>
                  </template>
                  <template v-else>
                    {{ formatDnsServers(interfaceData.dns_servers) || 'N/A' }}
                    <v-btn 
                      icon 
                      variant="text" 
                      size="small" 
                      @click="enableEditDns(interfaceData.id, interfaceData.dns_servers)" 
                      class="ml-1"
                    >
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
                    <v-text-field 
                      v-model="newInterface.name" 
                      placeholder="eth0" 
                      density="compact" 
                      hide-details
                      :rules="[required]"
                    />
                  </td>
                </tr>
                <tr>
                  <th>MAC Address:</th>
                  <td>
                    <v-text-field 
                      v-model="newInterface.mac_address" 
                      placeholder="00:1A:2B:3C:4D:5E" 
                      density="compact" 
                      hide-details
                    />
                  </td>
                </tr>
                <tr>
                  <th>Gateway:</th>
                  <td>
                    <v-text-field 
                      v-model="newInterface.gateway" 
                      placeholder="192.168.1.1" 
                      density="compact" 
                      hide-details
                    />
                  </td>
                </tr>
                <tr>
                  <th>DNS Servers:</th>
                  <td>
                    <v-text-field 
                      v-model="newInterface.dns_servers" 
                      placeholder="8.8.8.8,8.8.4.4" 
                      density="compact" 
                      hide-details
                    />
                  </td>
                </tr>
                <tr>
                  <th>IP Addresses:</th>
                  <td>
                    <div v-for="(ip, index) in newInterface.ips" :key="index" class="mb-2">
                      <div class="d-flex align-center">
                        <v-text-field
                          v-model="ip.ip_address"
                          placeholder="192.168.1.100"
                          density="compact"
                          hide-details
                          class="mr-2"
                          :rules="[required]"
                        />
                        <v-text-field
                          v-model="ip.subnet_mask"
                          placeholder="255.255.255.0"
                          density="compact"
                          hide-details
                          class="mr-2"
                        />
                        <v-btn
                          icon
                          color="error"
                          size="small"
                          @click="removeNewIp(index)"
                          v-if="newInterface.ips.length > 1"
                        >
                          <v-icon>mdi-delete</v-icon>
                        </v-btn>
                      </div>
                    </div>
                    <v-btn
                      color="primary"
                      @click="addNewIp"
                      prepend-icon="mdi-plus"
                      size="small"
                    >
                      Add IP Address
                    </v-btn>
                  </td>
                </tr>
                <tr>
                  <td colspan="2" class="text-right pt-4">
                    <v-btn 
                      color="primary" 
                      type="submit" 
                      :loading="isAddingInterface"
                    >
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

    <!-- IPアドレス編集モーダル -->
    <v-dialog v-model="showEditIpModal" max-width="800">
      <v-card>
        <v-card-title>
          <template v-if="isEditingInterfaceName">
            <v-text-field
              v-model="editingInterfaceName"
              label="Interface Name"
              density="compact"
              hide-details
              class="mb-2"
            />
          </template>
          <template v-else>
            Edit IP Addresses - {{ selectedInterfaceForEdit?.name || 'Unknown Interface' }}
            <v-btn
              icon
              variant="text"
              size="small"
              @click="enableEditInterfaceName"
              class="ml-2"
              :disabled="!selectedInterfaceForEdit"
            >
              <v-icon>mdi-pencil</v-icon>
            </v-btn>
          </template>
        </v-card-title>
        <v-card-text>
          <v-table>
            <thead>
              <tr>
                <th>IP Address</th>
                <th>Subnet Mask</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(ip, index) in editingIps[currentEditInterfaceId]" :key="index">
                <td>
                  <v-text-field
                    v-model="ip.ip_address"
                    density="compact"
                    hide-details
                    placeholder="192.168.1.100"
                  />
                </td>
                <td>
                  <v-text-field
                    v-model="ip.subnet_mask"
                    density="compact"
                    hide-details
                    placeholder="255.255.255.0"
                  />
                </td>
                <td>
                  <v-btn
                    icon
                    color="error"
                    size="small"
                    @click="removeIpFromEdit(index)"
                  >
                    <v-icon>mdi-delete</v-icon>
                  </v-btn>
                </td>
              </tr>
              <tr>
                <td colspan="3" class="text-center pt-4">
                  <v-btn
                    color="primary"
                    @click="addNewIpRow"
                    prepend-icon="mdi-plus"
                  >
                    Add IP Row
                  </v-btn>
                </td>
              </tr>
            </tbody>
          </v-table>
          <div v-if="isEditingInterfaceName" class="d-flex justify-end mt-4">
            <v-btn
              color="primary"
              @click="saveInterfaceName"
              :loading="isSavingInterfaceName"
              class="mr-2"
            >
              Save Name
            </v-btn>
            <v-btn
              color="secondary"
              @click="cancelEditInterfaceName"
            >
              Cancel
            </v-btn>
          </div>
          <v-alert v-if="ipEditError" type="error" density="compact" class="mt-4">
            {{ ipEditError }}
          </v-alert>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn
            color="primary"
            @click="saveInterfaceIps"
            :loading="isSavingInterfaceIps"
          >
            Save Changes
          </v-btn>
          <v-btn
            color="secondary"
            @click="cancelEditIpModal"
          >
            Cancel
          </v-btn>
        </v-card-actions>
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
          <v-btn color="secondary" @click="cancelDeleteInterface">
            Cancel
          </v-btn>
        </v-card-actions>
        <v-alert v-if="deleteError" type="error" density="compact" class="mx-4 mb-4">
          {{ deleteError }}
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
const required = (value) => !!value || 'Required';
import { ref, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { apiBaseUrl } from '@/apiConfig';

const router = useRouter();
const route = useRoute();

// メインのマシンデータ
const machine = ref(null);

// ホスト名編集関連
const isEditingHostname = ref(false);
const isUpdatingHostname = ref(false);

// メモ編集関連
const isEditingMemo = ref(false);
const isUpdatingMemo = ref(false);

// 目的編集関連
const isEditingPurpose = ref(false);
const isUpdatingPurpose = ref(false);

// VMステータス編集関連
const isEditingVmStatus = ref(false);
const isUpdatingVmStatus = ref(false);
const vmStatusEdit = ref({
  is_virtual: false,
  parent_machine_id: null
});

// 親マシンID編集関連
const isEditingParentId = ref(false);
const isUpdatingParentId = ref(false);

// インターフェース追加関連
const isAddingInterface = ref(false);
const addInterfaceError = ref('');
const newInterface = ref({
  name: '',
  mac_address: '',
  gateway: '',
  dns_servers: '',
  ips: [
    { ip_address: '', subnet_mask: '255.255.255.0' }
  ]
});

// IPアドレス編集関連
const showEditIpModal = ref(false);
const selectedInterfaceForEdit = ref(null);
const currentEditInterfaceId = ref('');
const editingIps = ref({});
const isSavingInterfaceIps = ref(false);
const ipEditError = ref('');

// ゲートウェイ編集関連
const isEditingGateway = ref({});
const gatewayEdit = ref({});
const isUpdatingGateway = ref({});

// DNS編集関連
const isEditingDns = ref({});
const dnsEdit = ref({});
const isUpdatingDns = ref({});

// インターフェース編集関連
const isEditingInterfaceName = ref(false);
const editingInterfaceName = ref('');
const isSavingInterfaceName = ref(false);

// インターフェース削除関連
const showDeleteInterfaceModal = ref(false);
const interfaceToDelete = ref('');
const interfaceToDeleteName = ref('');
const isDeletingInterface = ref(false);
const deleteError = ref('');

// マシン削除関連
const showDeleteMachineDialog = ref(false);
const isDeleting = ref(false);

// クリップボード関連
const copiedItems = ref({});

// 新しいIPアドレスを追加
const addNewIp = () => {
  newInterface.value.ips.push({
    ip_address: '',
    subnet_mask: '255.255.255.0'
  });
};

// IPアドレスを削除
const removeNewIp = (index) => {
  newInterface.value.ips.splice(index, 1);
};

// IPアドレス編集モーダルを開く
const prepareEditIps = (interfaceData) => {
  if (!interfaceData) {
    console.error('Interface data is null or undefined');
    return;
  }

  selectedInterfaceForEdit.value = interfaceData;
  currentEditInterfaceId.value = interfaceData.id;
  editingInterfaceName.value = interfaceData.name || '';
  isEditingInterfaceName.value = false;
  
  editingIps.value[interfaceData.id] = interfaceData.ips?.length > 0
    ? interfaceData.ips.map(ip => ({ ...ip }))
    : [{ ip_address: '', subnet_mask: '255.255.255.0' }];
  
  showEditIpModal.value = true;
  ipEditError.value = '';
};


// インターフェース名編集を有効化
const enableEditInterfaceName = () => {
  isEditingInterfaceName.value = true;
};

// インターフェース名編集をキャンセル
const cancelEditInterfaceName = () => {
  isEditingInterfaceName.value = false;
  editingInterfaceName.value = selectedInterfaceForEdit.value.name;
};

// インターフェース名を保存
const saveInterfaceName = async () => {
  if (!selectedInterfaceForEdit.value) {
    console.error('No interface selected for editing');
    return;
  }

  const newName = (editingInterfaceName.value || '').trim();
  if (!newName) {
    alert('Interface name cannot be empty');
    return;
  }

  isSavingInterfaceName.value = true;

  try {
    const response = await fetch(
      `${apiBaseUrl}/interfaces/${machine.value.id}/${selectedInterfaceForEdit.value.name}/update-name`,
      {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: newName })
      }
    );

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to update interface name');
    }

    isEditingInterfaceName.value = false;
    const updatedResponse = await fetch(`${apiBaseUrl}/machines/${route.params.id}`);
    if (updatedResponse.ok) {
      machine.value = await updatedResponse.json();
      // 安全なプロパティアクセス
      const updatedInterface = machine.value.interfaces?.find(
        intf => intf.id === currentEditInterfaceId.value
      );
      if (updatedInterface) {
        selectedInterfaceForEdit.value = updatedInterface;
        editingInterfaceName.value = updatedInterface.name || '';
      }
    }
  } catch (err) {
    console.error('Failed to update interface name:', err);
    alert(err.message);
  } finally {
    isSavingInterfaceName.value = false;
  }
};

// IP行を追加
const addNewIpRow = () => {
  if (!editingIps.value[currentEditInterfaceId.value]) {
    editingIps.value[currentEditInterfaceId.value] = [];
  }
  editingIps.value[currentEditInterfaceId.value].push({
    ip_address: '',
    subnet_mask: '255.255.255.0'
  });
};

// IP行を削除
const removeIpFromEdit = (index) => {
  editingIps.value[currentEditInterfaceId.value].splice(index, 1);
};

// IP編集をキャンセル
const cancelEditIpModal = () => {
  showEditIpModal.value = false;
  selectedInterfaceForEdit.value = null;
  currentEditInterfaceId.value = '';
  ipEditError.value = '';
};

// IPアドレスを保存
const saveInterfaceIps = async () => {
  const interfaceId = currentEditInterfaceId.value;
  if (!editingIps.value[interfaceId]) return;

  isSavingInterfaceIps.value = true;
  ipEditError.value = '';

  try {
    // バリデーション
    const invalidIps = editingIps.value[interfaceId].filter(
      ip => ip.ip_address && !isValidIp(ip.ip_address)
    );
    
    if (invalidIps.length > 0) {
      throw new Error('Invalid IP address format');
    }

    const interfaceData = machine.value.interfaces.find(intf => intf.id === interfaceId);
    if (!interfaceData) throw new Error('Interface not found');

    // 空のIPアドレスをフィルタリング
    const ipsToSave = editingIps.value[interfaceId].filter(ip => ip.ip_address.trim() !== '');

    const response = await fetch(
      `${apiBaseUrl}/interfaces/${machine.value.id}/${interfaceData.name}/ips`,
      {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ips: ipsToSave })
      }
    );

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to update IP addresses');
    }

    showEditIpModal.value = false;
    const updatedResponse = await fetch(`${apiBaseUrl}/machines/${route.params.id}`);
    if (updatedResponse.ok) {
      machine.value = await updatedResponse.json();
    }
  } catch (err) {
    console.error('IPアドレスの更新エラー:', err);
    ipEditError.value = err.message;
  } finally {
    isSavingInterfaceIps.value = false;
  }
};

// IPアドレスのバリデーション
const isValidIp = (ip) => {
  const ipRegex = /^(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$/;
  return ipRegex.test(ip);
};

// ホスト名編集
const enableEditHostname = () => {
  isEditingHostname.value = true;
};

const cancelEditHostname = () => {
  isEditingHostname.value = false;
  fetch(`${apiBaseUrl}/machines/${route.params.id}`)
    .then(response => response.json())
    .then(data => {
      machine.value.hostname = data.hostname;
    })
    .catch(err => console.error(err));
};

const saveHostname = async () => {
  isUpdatingHostname.value = true;

  const response = await fetch(`${apiBaseUrl}/machines/${machine.value.id}/update-hostname`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ hostname: machine.value.hostname })
  });

  isUpdatingHostname.value = false;

  if (response.ok) {
    isEditingHostname.value = false;
  } else {
    const errorData = await response.json();
    alert(`Failed to update hostname: ${errorData.error || 'Unknown error'}`);
  }
};

// メモ編集
const enableEditMemo = () => {
  isEditingMemo.value = true;
};

const cancelEditMemo = () => {
  isEditingMemo.value = false;
};

const updateMemo = async () => {
  isUpdatingMemo.value = true;

  const response = await fetch(`${apiBaseUrl}/machines/${machine.value.id}/update-memo`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ memo: machine.value.memo })
  });

  isUpdatingMemo.value = false;

  if (response.ok) {
    isEditingMemo.value = false;
  } else {
    const errorData = await response.json();
    alert(`Failed to update memo: ${errorData.error || 'Unknown error'}`);
  }
};

// 目的編集
const enableEditPurpose = () => {
  isEditingPurpose.value = true;
};

const cancelEditPurpose = () => {
  isEditingPurpose.value = false;
  fetch(`${apiBaseUrl}/machines/${route.params.id}`)
    .then(response => response.json())
    .then(data => {
      machine.value.purpose = data.purpose;
    })
    .catch(err => console.error(err));
};

const updatePurpose = async () => {
  isUpdatingPurpose.value = true;

  const response = await fetch(`${apiBaseUrl}/machines/${machine.value.id}/update-purpose`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ purpose: machine.value.purpose })
  });

  isUpdatingPurpose.value = false;

  if (response.ok) {
    isEditingPurpose.value = false;
  } else {
    const errorData = await response.json();
    alert(`Failed to update purpose: ${errorData.error || 'Unknown error'}`);
  }
};

// VMステータス編集
const enableEditVmStatus = () => {
  vmStatusEdit.value = {
    is_virtual: machine.value.is_virtual,
    parent_machine_id: machine.value.parent_machine_id
  };
  isEditingVmStatus.value = true;
};

const cancelEditVmStatus = () => {
  isEditingVmStatus.value = false;
};

const handleVmStatusChange = () => {
  if (!vmStatusEdit.value.is_virtual) {
    vmStatusEdit.value.parent_machine_id = null;
  }
};

const saveVmStatus = async () => {
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
};

// 親マシンID編集
const enableEditParentId = () => {
  isEditingParentId.value = true;
};

const cancelEditParentId = () => {
  isEditingParentId.value = false;
};

const updateParentId = async () => {
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
};

// インターフェース削除
const confirmDeleteInterface = (interfaceId, interfaceName) => {
  interfaceToDelete.value = interfaceId;
  interfaceToDeleteName.value = interfaceName;
  showDeleteInterfaceModal.value = true;
  deleteError.value = '';
};

const cancelDeleteInterface = () => {
  showDeleteInterfaceModal.value = false;
  interfaceToDelete.value = '';
  interfaceToDeleteName.value = '';
  deleteError.value = '';
};

const deleteInterface = async () => {
  isDeletingInterface.value = true;
  deleteError.value = '';

  try {
    const interfaceData = machine.value.interfaces.find(intf => intf.id === interfaceToDelete.value);
    if (!interfaceData) {
      throw new Error('Interface not found');
    }

    const response = await fetch(
      `${apiBaseUrl}/machines/${machine.value.id}/interfaces/${interfaceData.name}`,
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

    // マシンデータを再読み込み
    const updatedResponse = await fetch(`${apiBaseUrl}/machines/${route.params.id}`);
    if (updatedResponse.ok) {
      machine.value = await updatedResponse.json();
    }
  } catch (err) {
    console.error('インターフェース削除エラー:', err);
    deleteError.value = err.message;
  } finally {
    isDeletingInterface.value = false;
  }
};

// マシン削除
const confirmMachineDelete = () => {
  showDeleteMachineDialog.value = true;
};

const cancelMachineDelete = () => {
  showDeleteMachineDialog.value = false;
};

const deleteMachine = async () => {
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
    showDeleteMachineDialog.value = false;
  }
};

// ゲートウェイ編集を有効化
const enableEditGateway = (interfaceId, currentGateway) => {
  isEditingGateway.value[interfaceId] = true;
  gatewayEdit.value[interfaceId] = currentGateway || '';
};

// ゲートウェイ編集をキャンセル
const cancelEditGateway = (interfaceId) => {
  isEditingGateway.value[interfaceId] = false;
  delete gatewayEdit.value[interfaceId];
};

// ゲートウェイを保存
const saveGateway = async (interfaceId) => {
  const interfaceData = machine.value.interfaces.find(intf => intf.id === interfaceId);
  if (!interfaceData) return;

  isUpdatingGateway.value[interfaceId] = true;

  try {
    const response = await fetch(
      `${apiBaseUrl}/interfaces/${machine.value.id}/${interfaceData.name}/update-gateway`,
      {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ gateway: gatewayEdit.value[interfaceId] || null })
      }
    );

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to update gateway');
    }

    // 成功したら編集モードを終了し、データを再読み込み
    isEditingGateway.value[interfaceId] = false;
    const updatedResponse = await fetch(`${apiBaseUrl}/machines/${route.params.id}`);
    if (updatedResponse.ok) {
      machine.value = await updatedResponse.json();
    }
  } catch (err) {
    console.error('ゲートウェイの更新エラー:', err);
    alert(err.message);
  } finally {
    isUpdatingGateway.value[interfaceId] = false;
  }
};

// DNSサーバー編集を有効化
const enableEditDns = (interfaceId, currentDns) => {
  isEditingDns.value[interfaceId] = true;
  dnsEdit.value[interfaceId] = Array.isArray(currentDns) 
    ? currentDns.join(', ') 
    : (currentDns || '');
};

// DNSサーバー編集をキャンセル
const cancelEditDns = (interfaceId) => {
  isEditingDns.value[interfaceId] = false;
  delete dnsEdit.value[interfaceId];
};

// DNSサーバーを保存
const saveDnsServers = async (interfaceId) => {
  const interfaceData = machine.value.interfaces.find(intf => intf.id === interfaceId);
  if (!interfaceData) return;

  isUpdatingDns.value[interfaceId] = true;

  try {
    // 入力値を配列に変換（カンマ区切りを想定）
    const dnsArray = dnsEdit.value[interfaceId]
      ? dnsEdit.value[interfaceId].split(',').map(item => item.trim()).filter(item => item)
      : [];

    const response = await fetch(
      `${apiBaseUrl}/interfaces/${machine.value.id}/${interfaceData.name}/update-dns`,
      {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ dns_servers: dnsArray })
      }
    );

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to update DNS servers');
    }

    // 成功したら編集モードを終了し、データを再読み込み
    isEditingDns.value[interfaceId] = false;
    const updatedResponse = await fetch(`${apiBaseUrl}/machines/${route.params.id}`);
    if (updatedResponse.ok) {
      machine.value = await updatedResponse.json();
    }
  } catch (err) {
    console.error('DNSサーバーの更新エラー:', err);
    alert(err.message);
  } finally {
    isUpdatingDns.value[interfaceId] = false;
  }
};

// DNSサーバーの表示フォーマット
const formatDnsServers = (dns) => {
  if (!dns) return '';
  return Array.isArray(dns) ? dns.join(', ') : dns;
};

// インターフェース追加
const addNewInterface = async () => {
  // バリデーション
  if (!newInterface.value.name) {
    addInterfaceError.value = 'Interface name is required';
    return;
  }

  if (!newInterface.value.ips.some(ip => ip.ip_address)) {
    addInterfaceError.value = 'At least one IP address is required';
    return;
  }

  isAddingInterface.value = true;
  addInterfaceError.value = '';

  try {
    // リクエストデータを整形
    const requestData = {
      name: newInterface.value.name,
      mac_address: newInterface.value.mac_address || null,
      gateway: newInterface.value.gateway || null,
      dns_servers: newInterface.value.dns_servers 
        ? newInterface.value.dns_servers.split(',').map(s => s.trim()).filter(s => s)
        : null,
      ips: newInterface.value.ips
        .filter(ip => ip.ip_address)
        .map(ip => ({
          ip_address: ip.ip_address,
          subnet_mask: ip.subnet_mask || '255.255.255.0'
        }))
    };

    const response = await fetch(`${apiBaseUrl}/machines/${machine.value.id}/interfaces`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(requestData)
    });

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to add interface');
    }

    // 成功したらフォームをリセット
    newInterface.value = {
      name: '',
      mac_address: '',
      gateway: '',
      dns_servers: '',
      ips: [
        { ip_address: '', subnet_mask: '255.255.255.0' }
      ]
    };

    // マシンデータを再読み込み
    const updatedResponse = await fetch(`${apiBaseUrl}/machines/${route.params.id}`);
    if (updatedResponse.ok) {
      machine.value = await updatedResponse.json();
    }
  } catch (err) {
    console.error('Failed to add interface:', err);
    addInterfaceError.value = err.message;
  } finally {
    isAddingInterface.value = false;
  }
};

// マシン複製
const duplicateMachine = () => {
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
};

// クリップボードにコピー
const copyToClipboard = async (text, itemId) => {
  if (!text || !itemId) return;

  if (copiedItems.value[itemId]) return;

  try {
    let success = false;

    if (navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(text);
      success = true;
    } else {
      const textarea = document.createElement('textarea');
      textarea.value = text;
      document.body.appendChild(textarea);
      textarea.select();

      try {
        if (document.execCommand) {
          document.execCommand('copy');
          success = true;
        } else {
          throw new Error('Browser does not support clipboard operations');
        }
      } catch (fallbackErr) {
        console.error('Fallback copy failed:', fallbackErr);
      } finally {
        document.body.removeChild(textarea);
      }
    }

    if (success) {
      copiedItems.value = { ...copiedItems.value, [itemId]: true };

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
};

// 初期データ読み込み
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

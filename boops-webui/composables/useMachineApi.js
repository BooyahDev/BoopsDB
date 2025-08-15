import { apiBaseUrl } from '@/apiConfig';

export function useMachineApi() {
  const updateMachineField = async (machineId, fieldName, value) => {
    let endpoint = '';
    let body = {};

    switch (fieldName) {
      case 'hostname':
        endpoint = `${apiBaseUrl}/machines/${machineId}/update-hostname`;
        body = { hostname: value };
        break;
      case 'purpose':
        endpoint = `${apiBaseUrl}/machines/${machineId}/update-purpose`;
        body = { purpose: value };
        break;
      case 'memo':
        endpoint = `${apiBaseUrl}/machines/${machineId}/update-memo`;
        body = { memo: value };
        break;
      case 'parent_machine_id':
        endpoint = `${apiBaseUrl}/machines/${machineId}/update-parent-id`;
        body = { parent_machine_id: value || null };
        break;
      case 'vm_status':
        endpoint = `${apiBaseUrl}/machines/${machineId}/update-vm-status`;
        body = {
          is_virtual: value.is_virtual,
          parent_machine_id: value.parent_machine_id
        };
        break;
      default:
        throw new Error(`Unknown field: ${fieldName}`);
    }

    const response = await fetch(endpoint, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to update machine');
    }

    // Return updated machine data
    const updatedResponse = await fetch(`${apiBaseUrl}/machines/${machineId}`);
    if (!updatedResponse.ok) {
      throw new Error('Failed to fetch updated machine data');
    }

    const updatedMachine = await updatedResponse.json();

    // Handle parent machine hostname if needed
    if (updatedMachine.is_virtual && updatedMachine.parent_machine_id) {
      try {
        const parentResponse = await fetch(`${apiBaseUrl}/machines/${updatedMachine.parent_machine_id}`);
        if (parentResponse.ok) {
          const parentMachine = await parentResponse.json();
          updatedMachine.parentHostname = parentMachine.hostname;
        }
      } catch (err) {
        console.warn('Failed to fetch parent machine hostname:', err);
      }
    } else {
      updatedMachine.parentHostname = null;
    }

    return updatedMachine;
  };

  const deleteMachine = async (machineId) => {
    const response = await fetch(`${apiBaseUrl}/machines/${machineId}`, {
      method: 'DELETE'
    });

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to delete machine');
    }
  };

  const getMachine = async (machineId) => {
    const response = await fetch(`${apiBaseUrl}/machines/${machineId}`);
    
    if (!response.ok) {
      throw new Error('Failed to load machine details');
    }

    const machine = await response.json();

    // Process dns_register as Boolean
    if (machine && Array.isArray(machine.interfaces)) {
      for (const iface of machine.interfaces) {
        if (Array.isArray(iface.ips)) {
          for (const ip of iface.ips) {
            ip.dns_register = !!ip.dns_register;
          }
        }
      }
    }

    // Get parent machine hostname if applicable
    if (machine.is_virtual && machine.parent_machine_id) {
      try {
        const parentResponse = await fetch(`${apiBaseUrl}/machines/${machine.parent_machine_id}`);
        if (parentResponse.ok) {
          const parentMachine = await parentResponse.json();
          machine.parentHostname = parentMachine.hostname;
        }
      } catch (err) {
        console.warn('Failed to fetch parent machine hostname:', err);
      }
    }

    return machine;
  };

  const duplicateMachine = (machine) => {
    if (!machine) return;

    const duplicateData = {
      ...machine,
      hostname: `${machine.hostname}`,
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

  return {
    updateMachineField,
    deleteMachine,
    getMachine,
    duplicateMachine
  };
}

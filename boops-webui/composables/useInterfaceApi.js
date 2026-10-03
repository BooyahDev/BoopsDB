import { useApiBaseUrl } from '@/apiConfig';

export function useInterfaceApi() {
  const apiBaseUrl = useApiBaseUrl();
  const updateInterfaceGateway = async (machineId, interfaceName, gateway) => {
    const response = await fetch(
      `${apiBaseUrl}/interfaces/${machineId}/${encodeURIComponent(interfaceName)}/update-gateway`,
      {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ gateway: gateway?.trim() || '' })
      }
    );

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to update gateway');
    }
  };

  const updateInterfaceDns = async (machineId, interfaceName, dnsServers) => {
    const response = await fetch(
      `${apiBaseUrl}/interfaces/${machineId}/${encodeURIComponent(interfaceName)}/update-dns`,
      {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ dns_servers: dnsServers })
      }
    );

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to update DNS servers');
    }
  };

  const updateInterfaceIps = async (machineId, interfaceName, ips) => {
    const response = await fetch(
      `${apiBaseUrl}/interfaces/${machineId}/${encodeURIComponent(interfaceName)}/ips`,
      {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ips })
      }
    );

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to update IP addresses');
    }
  };

  const updateInterfaceName = async (machineId, oldName, newName) => {
    const response = await fetch(
      `${apiBaseUrl}/interfaces/${machineId}/${encodeURIComponent(oldName)}/update-name`,
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
  };

  const createInterface = async (machineId, interfaceData) => {
    const response = await fetch(`${apiBaseUrl}/machines/${machineId}/interfaces`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(interfaceData)
    });

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to add interface');
    }
  };

  const deleteInterface = async (machineId, interfaceName) => {
    const response = await fetch(
      `${apiBaseUrl}/machines/${machineId}/interfaces/${encodeURIComponent(interfaceName)}`,
      {
        method: 'DELETE'
      }
    );

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to delete interface');
    }
  };

  return {
    updateInterfaceGateway,
    updateInterfaceDns,
    updateInterfaceIps,
    updateInterfaceName,
    createInterface,
    deleteInterface
  };
}

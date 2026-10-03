export function createMachineLoader({ getMachine, getMachineId, onMachine, onLoading, onError }) {
  let latestRequest = 0;
  return async () => {
    const request = ++latestRequest;
    const machineId = getMachineId();
    const isCurrent = () => request === latestRequest && machineId === getMachineId();
    onLoading(true);
    onError('');
    try {
      const machine = await getMachine(machineId);
      if (isCurrent()) onMachine(machine);
    } catch (error) {
      if (isCurrent()) onError(error.message);
    } finally {
      if (isCurrent()) onLoading(false);
    }
  };
}

export function refreshUpdatedMachine(updatedMachine, currentMachineId, reload) {
  if (typeof updatedMachine.id === 'string' && typeof currentMachineId === 'string'
    && updatedMachine.id.toLowerCase() === currentMachineId.toLowerCase()) return reload();
}

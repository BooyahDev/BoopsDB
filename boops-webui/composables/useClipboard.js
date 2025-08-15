import { ref } from 'vue';

export function useClipboard() {
  const copiedItems = ref({});

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

  return {
    copiedItems,
    copyToClipboard
  };
}

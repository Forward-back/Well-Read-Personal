import { ref } from 'vue'
import { defineStore } from 'pinia'

export const useChatReferencesStore = defineStore('chatReferences', () => {
  const visible = ref(false)

  function setVisible(value: boolean) {
    visible.value = value
  }

  return { visible, setVisible }
})

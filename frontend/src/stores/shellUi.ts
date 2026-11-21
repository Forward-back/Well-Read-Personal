import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useShellUiStore = defineStore('shellUi', () => {
  const fullscreen = ref(false)

  function setFullscreen(value: boolean) {
    fullscreen.value = value
  }

  return { fullscreen, setFullscreen }
})

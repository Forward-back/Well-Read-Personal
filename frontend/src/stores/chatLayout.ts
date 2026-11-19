import { ref } from 'vue'
import { defineStore } from 'pinia'

const STORAGE_KEY = 'chat_rail_visible'

export const useChatLayoutStore = defineStore('chatLayout', () => {
  const railVisible = ref(localStorage.getItem(STORAGE_KEY) !== 'false')
  const searchOpen = ref(false)

  function setRailVisible(value: boolean) {
    railVisible.value = value
    localStorage.setItem(STORAGE_KEY, String(value))
  }

  function toggleRail() {
    setRailVisible(!railVisible.value)
  }

  function openSearch() {
    searchOpen.value = true
  }

  function closeSearch() {
    searchOpen.value = false
  }

  return { railVisible, searchOpen, setRailVisible, toggleRail, openSearch, closeSearch }
})

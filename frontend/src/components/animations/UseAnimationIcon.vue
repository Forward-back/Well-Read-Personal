<template>
  <span
    ref="host"
    class="use-animation-icon"
    :style="{ width: `${size}px`, height: `${size}px`, color }"
    :role="label ? 'img' : undefined"
    :aria-label="label || undefined"
    :aria-hidden="label ? undefined : 'true'"
  />
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { USE_ANIMATIONS, type UseAnimationName } from './useAnimations'
import { cloneAndTintLottie } from './lottieTint'

const props = withDefaults(
  defineProps<{
    name: UseAnimationName
    size?: number
    color?: string
    loop?: boolean
    autoplay?: boolean
    speed?: number
    label?: string
  }>(),
  { size: 20, color: 'currentColor', loop: true, autoplay: true, speed: 1, label: '' },
)

const host = ref<HTMLElement | null>(null)
const themeKey = ref(0)
let animation: { destroy: () => void; play: () => void; goToAndStop: (value: number, isFrame?: boolean) => void; setSpeed: (value: number) => void } | null = null
let mountSeq = 0
let themeObserver: MutationObserver | null = null
let motionQuery: MediaQueryList | null = null

const source = computed(() => USE_ANIMATIONS[props.name])
const reducedMotion = () => motionQuery?.matches === true

async function mountAnimation() {
  const seq = ++mountSeq
  const target = host.value
  const data = source.value
  if (!target || !data) return
  animation?.destroy()
  animation = null
  target.innerHTML = ''
  const tone = getComputedStyle(target).color
  const tinted = cloneAndTintLottie(data, tone)
  const module = await import('lottie-web')
  if (seq !== mountSeq || !host.value) return
  const lottie = module.default
  const instance = lottie.loadAnimation({
    container: target,
    renderer: 'svg',
    loop: props.loop && !reducedMotion(),
    autoplay: false,
    animationData: tinted,
  })
  animation = instance as unknown as typeof animation
  instance.setSpeed(props.speed)
  if (reducedMotion()) {
    instance.goToAndStop(0, true)
  } else if (props.autoplay) {
    instance.play()
  }
}

function handleMotionChange() {
  void mountAnimation()
}

watch(
  () => [props.name, props.size, props.color, props.loop, props.autoplay, props.speed, themeKey.value],
  () => { void nextTick(mountAnimation) },
)

onMounted(() => {
  motionQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
  motionQuery.addEventListener('change', handleMotionChange)
  themeObserver = new MutationObserver(() => { themeKey.value += 1 })
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['theme-mode'] })
  void mountAnimation()
})

onUnmounted(() => {
  mountSeq += 1
  animation?.destroy()
  animation = null
  themeObserver?.disconnect()
  themeObserver = null
  motionQuery?.removeEventListener('change', handleMotionChange)
  motionQuery = null
})
</script>

<style scoped>
.use-animation-icon {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  vertical-align: middle;
}

.use-animation-icon :deep(svg) {
  width: 100%;
  height: 100%;
  display: block;
}
</style>

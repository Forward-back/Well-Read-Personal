import loading from '@/assets/animations/useanimations/loading.json'

export type UseAnimationName = 'loading'

type LottieData = Record<string, unknown>

export const USE_ANIMATIONS: Record<UseAnimationName, LottieData> = {
  loading: loading as LottieData,
}

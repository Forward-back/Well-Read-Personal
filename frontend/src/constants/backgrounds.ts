export interface BuiltinBackground {
  id: string
  index: number
  url: string
}

export const BUILTIN_BACKGROUNDS: BuiltinBackground[] = [
  { id: 'builtin:1', index: 1, url: '/backgrounds/wallpaper-1.jpg' },
  { id: 'builtin:2', index: 2, url: '/backgrounds/wallpaper-2.jpg' },
  { id: 'builtin:3', index: 3, url: '/backgrounds/wallpaper-3.jpg' },
  { id: 'builtin:4', index: 4, url: '/backgrounds/wallpaper-4.jpg' },
  { id: 'builtin:5', index: 5, url: '/backgrounds/wallpaper-5.jpg' },
]

export const DEFAULT_BACKGROUND_CHOICE = 'default'

export function builtinBackgroundUrl(choice: string): string | null {
  return BUILTIN_BACKGROUNDS.find((item) => item.id === choice)?.url ?? null
}

export function productArtwork(slug: string) {
  if (slug === 'grand-theft-auto-vi-ps5') return '/art/grand-theft-auto-vi.jpg'
  if (slug === 'sekiro-shadows-die-twice-pc') return '/art/sekiro-shadows-die-twice.jpg'
  if (slug === 'hollow-knight-silksong-pc') return '/art/silksong.jpg'
  if (slug === 'hollow-knight-pc') return '/art/hollow-knight.jpg'
  if (slug === 'minecraft-pc') return '/art/minecraft.jpg'
  if (slug === 'terraria-pc') return '/art/terraria.jpg'
  return '/art/game-placeholder.svg'
}

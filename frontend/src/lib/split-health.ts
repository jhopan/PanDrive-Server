export type SplitHealth = { status: string }

export function requireHealthySplit(health: SplitHealth) {
  if (health.status !== 'ok') throw new Error('This split video cannot be streamed because one or more split parts are missing or changed. Restore every part, then try again.')
}

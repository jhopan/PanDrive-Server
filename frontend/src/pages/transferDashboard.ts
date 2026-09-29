export type OAuthQuota = { status: string; requestCount: number }

export function quotaState(config: OAuthQuota, threshold: number, max: number) {
  if (config.status !== 'active') return 'unavailable'
  if (config.requestCount >= threshold || config.requestCount >= max) return 'rotating'
  return 'available'
}

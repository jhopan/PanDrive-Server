export type TransferSchedulerSnapshot = { activeFiles: number; activeChunks: number }

type Job<T> = () => Promise<T>

function limit(max: number, onChange: () => void) {
  let active = 0
  const waiting: Array<() => void> = []
  return {
    get active() { return active },
    async run<T>(job: Job<T>) {
      if (active >= max) await new Promise<void>((resolve) => waiting.push(resolve))
      active += 1
      onChange()
      try {
        return await job()
      } finally {
        active -= 1
        waiting.shift()?.()
        onChange()
      }
    }
  }
}

export function createTransferScheduler(onChange: (snapshot: TransferSchedulerSnapshot) => void = () => undefined) {
  let files!: ReturnType<typeof limit>
  let chunks!: ReturnType<typeof limit>
  const changed = () => onChange({ activeFiles: files.active, activeChunks: chunks.active })
  files = limit(3, changed)
  chunks = limit(4, changed)
  const splitParts = limit(2, () => undefined)
  return {
    runFile: files.run,
    runChunk: chunks.run,
    runSplitPart: splitParts.run,
  }
}

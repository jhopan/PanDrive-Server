import { useCallback, useEffect, useState } from 'react'
import { ArrowLeftRight, RefreshCw, Timer } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { PageHeader } from '@/components/drive/PageHeader'
import { useUpload } from '@/context/UploadContext'
import { apiFetch, formatBytes, formatDate } from '@/lib/api'

type Reservation = { id: string; fileName: string; sizeBytes: string; status: string; expiresAt: string; accountEmail: string }
type QueueItem = { id: string; fileName: string; sizeBytes: string; status: string; accountEmail: string; createdAt: string }
type RateLimits = { requestsLast100s: number; limit: number; threshold: number; windowSeconds: number }

export function TransfersPage() {
  const [reservations, setReservations] = useState<Reservation[]>([])
  const [queue, setQueue] = useState<QueueItem[]>([])
  const [rate, setRate] = useState<RateLimits | null>(null)
  const [loading, setLoading] = useState(true)
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [message, setMessage] = useState('')
  const { transferScheduler } = useUpload()
  const load = useCallback(async () => {
    try {
      const [reservationData, queueData, rateData] = await Promise.all([apiFetch<{ items: Reservation[] }>('/uploads/reservations'), apiFetch<{ items: QueueItem[] }>('/uploads/queue?status=all'), apiFetch<RateLimits>('/system/rate-limits')])
      setReservations(reservationData.items); setQueue(queueData.items.filter((item) => ['uploading', 'pending', 'in_progress'].includes(item.status))); setRate(rateData); setMessage('')
    } catch (error) { setMessage(error instanceof Error ? error.message : 'Failed to load transfers') } finally { setLoading(false) }
  }, [])
  useEffect(() => { load().catch(() => undefined) }, [load])
  useEffect(() => { if (!autoRefresh) return; const id = window.setInterval(() => load().catch(() => undefined), 5000); return () => window.clearInterval(id) }, [autoRefresh, load])
  const requests = rate?.requestsLast100s ?? 0; const limit = rate?.limit ?? 10000; const percent = Math.min(100, requests / limit * 100)
  return <>
    <PageHeader title="Transfers" description="Live upload work, held capacity, and Google API pressure." actions={<div className="flex gap-2"><Button variant="outline" size="sm" onClick={() => setAutoRefresh((value) => !value)}><Timer className="h-4 w-4" />Auto {autoRefresh ? 'On' : 'Off'}</Button><Button variant="outline" size="sm" onClick={() => load()} disabled={loading}><RefreshCw className={loading ? 'h-4 w-4 animate-spin' : 'h-4 w-4'} />Refresh</Button></div>} />
    {message ? <p className="mt-4 rounded-xl bg-red-50 p-3 text-sm text-red-700">{message}</p> : null}
    <div className="mt-5 grid gap-3 sm:grid-cols-4"><Card className="p-4"><p className="text-xs font-bold uppercase text-slate-400">Active files</p><p className="mt-1 text-2xl font-extrabold text-violet-600">{transferScheduler.activeFiles}/3</p></Card><Card className="p-4"><p className="text-xs font-bold uppercase text-slate-400">Active chunks</p><p className="mt-1 text-2xl font-extrabold text-violet-600">{transferScheduler.activeChunks}/4</p></Card><Card className="p-4"><p className="text-xs font-bold uppercase text-slate-400">Server queue</p><p className="mt-1 text-2xl font-extrabold text-blue-600">{queue.length}</p></Card><Card className="p-4"><p className="text-xs font-bold uppercase text-slate-400">Capacity held</p><p className="mt-1 text-2xl font-extrabold text-amber-600">{reservations.length}</p></Card></div>
    <Card className="mt-4 p-4"><div className="flex items-center justify-between"><h2 className="flex items-center gap-2 text-[16px] font-bold"><ArrowLeftRight className="h-4 w-4 text-blue-600" />Google API rate</h2><span className="text-xs text-slate-500">{requests.toLocaleString()} / {limit.toLocaleString()} requests per {rate?.windowSeconds ?? 100}s</span></div><div className="mt-3 h-2 overflow-hidden rounded-full bg-slate-100"><div className={requests >= (rate?.threshold ?? 8000) ? 'h-full bg-amber-500' : 'h-full bg-emerald-500'} style={{ width: `${percent}%` }} /></div></Card>
    <Card className="mt-4 p-4"><h2 className="text-[16px] font-bold">Running uploads</h2><div className="mt-3 grid gap-2">{queue.length === 0 ? <p className="text-sm text-slate-500">No active upload sessions.</p> : queue.map((item) => <div key={item.id} className="rounded-xl border border-slate-100 p-3"><p className="font-semibold text-sm">{item.fileName}</p><p className="mt-0.5 text-xs text-slate-500">{formatBytes(item.sizeBytes)} · {item.accountEmail || 'no account'} · {item.status} · {formatDate(item.createdAt)}</p></div>)}</div></Card>
    <Card className="mt-4 p-4"><h2 className="text-[16px] font-bold">Reserved folder capacity</h2><p className="mt-1 text-xs text-slate-500">Space held before planned folder uploads start. Reservations expire automatically.</p><div className="mt-3 grid gap-2">{reservations.length === 0 ? <p className="text-sm text-slate-500">No active reservations.</p> : reservations.map((item) => <div key={item.id} className="rounded-xl border border-slate-100 p-3"><p className="font-semibold text-sm">{item.fileName}</p><p className="mt-0.5 text-xs text-slate-500">{formatBytes(item.sizeBytes)} · {item.accountEmail || 'no account'} · {item.status}{item.expiresAt ? ` · expires ${formatDate(item.expiresAt)}` : ''}</p></div>)}</div></Card>
  </>
}
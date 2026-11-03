import { useState } from 'react'
import { apiUrl } from '../../../shared/api'
import { Button, Card } from '../../../shared/ui'
import { CameraCapture } from './CameraCapture'

interface Props {
  title: string
  kind: 'VISITOR' | 'ID_DOCUMENT'
  visitId: string
  saved: boolean
  version: number
  facingMode: 'user' | 'environment'
  /** called with the photo; throw to keep the camera open */
  onSave: (photo: Blob) => Promise<void>
  /** blocks capture with a message (e.g. the ID type was not chosen yet) */
  blockedReason?: string
  children?: React.ReactNode
}

/** One capture slot: shows the photo saved on the server, or the camera to take/retake it. */
export function PhotoSlot({ title, kind, visitId, saved, version, facingMode, onSave, blockedReason, children }: Props) {
  const [capturing, setCapturing] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string>()

  async function save(photo: Blob) {
    setSaving(true)
    setError(undefined)
    try {
      await onSave(photo)
      setCapturing(false)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not save the photo')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Card title={title} actions={saved ? <span className="rounded-full border border-green-400 bg-green-100 px-2.5 py-0.5 text-xs font-semibold text-green-900">Saved</span> : <span className="rounded-full border border-amber-400 bg-amber-100 px-2.5 py-0.5 text-xs font-semibold text-amber-900">Missing</span>}>
      <div className="space-y-3">
        {children}
        {saved && !capturing && (
          <img src={`${apiUrl(`/visits/${visitId}/photos/${kind}`)}?v=${version}-${saved}`} alt={`Saved ${title.toLowerCase()}`} className="aspect-video w-full rounded-md border border-slate-300 object-cover" />
        )}
        {error && <p role="alert" className="text-sm text-red-700">{error}</p>}
        {blockedReason && !capturing && <p className="text-sm text-amber-800">{blockedReason}</p>}
        {capturing ? (
          saving ? <p role="status" className="text-sm text-slate-600">Saving photo…</p> : (
            <CameraCapture facingMode={facingMode} label={title} onCapture={(b) => void save(b)} onCancel={() => setCapturing(false)} />
          )
        ) : (
          <Button variant={saved ? 'secondary' : 'primary'} disabled={!!blockedReason} onClick={() => setCapturing(true)}>
            {saved ? 'Retake photo' : 'Take photo'}
          </Button>
        )}
      </div>
    </Card>
  )
}

import { useCallback, useEffect, useRef, useState } from 'react'
import { Button } from '../../../shared/ui'

interface Props {
  /** `user` = front camera (the visitor), `environment` = back camera (an ID card) */
  facingMode: 'user' | 'environment'
  label: string
  onCapture: (photo: Blob) => void
  onCancel?: () => void
}

type Mode = 'starting' | 'live' | 'review' | 'fallback'

const MAX_SIDE = 1600

/**
 * Camera capture with retake. Falls back to a file input when the camera is unavailable or
 * permission is denied. Every media track is stopped on capture and on unmount.
 */
export function CameraCapture({ facingMode, label, onCapture, onCancel }: Props) {
  const video = useRef<HTMLVideoElement>(null)
  const stream = useRef<MediaStream | null>(null)
  const [mode, setMode] = useState<Mode>('starting')
  const [message, setMessage] = useState<string>()
  const [shot, setShot] = useState<{ blob: Blob; url: string } | null>(null)

  const stop = useCallback(() => {
    stream.current?.getTracks().forEach((t) => t.stop())
    stream.current = null
  }, [])

  const start = useCallback(async () => {
    setShot(null)
    setMode('starting')
    if (!navigator.mediaDevices?.getUserMedia) {
      setMessage('This device has no camera access here. Choose a photo instead.')
      return setMode('fallback')
    }
    try {
      const s = await navigator.mediaDevices.getUserMedia({ video: { facingMode: { ideal: facingMode } }, audio: false })
      stream.current = s
      setMode('live')
    } catch (e) {
      const denied = e instanceof DOMException && (e.name === 'NotAllowedError' || e.name === 'SecurityError')
      setMessage(denied ? 'Camera permission was denied. Allow the camera in the browser, or choose a photo instead.' : 'No camera was found. Choose a photo instead.')
      setMode('fallback')
    }
  }, [facingMode])

  useEffect(() => {
    void start()
    return stop
  }, [start, stop])

  useEffect(() => {
    if (mode === 'live' && video.current && stream.current) {
      video.current.srcObject = stream.current
      const p = video.current.play() as Promise<void> | undefined
      p?.catch(() => {})
    }
  }, [mode])

  useEffect(() => () => { if (shot) URL.revokeObjectURL(shot.url) }, [shot])

  function take() {
    const v = video.current
    if (!v) return
    const w = v.videoWidth || 640
    const h = v.videoHeight || 480
    const scale = Math.min(1, MAX_SIDE / Math.max(w, h))
    const canvas = document.createElement('canvas')
    canvas.width = Math.round(w * scale)
    canvas.height = Math.round(h * scale)
    canvas.getContext('2d')?.drawImage(v, 0, 0, canvas.width, canvas.height)
    canvas.toBlob((blob) => {
      if (!blob) return setMessage('Could not take the photo. Try again.')
      stop()
      setShot({ blob, url: URL.createObjectURL(blob) })
      setMode('review')
    }, 'image/jpeg', 0.9)
  }

  return (
    <div className="space-y-3">
      {mode === 'starting' && <p role="status" className="text-sm text-slate-600">Starting camera…</p>}
      {mode === 'live' && (
        <>
          <video ref={video} playsInline muted aria-label={`Camera preview for ${label}`} className="aspect-video w-full rounded-md bg-slate-900 object-cover" />
          <div className="flex gap-2">
            <Button onClick={take}>Take photo</Button>
            {onCancel && <Button variant="secondary" onClick={() => { stop(); onCancel() }}>Cancel</Button>}
          </div>
        </>
      )}
      {mode === 'review' && shot && (
        <>
          <img src={shot.url} alt={`Captured ${label}`} className="aspect-video w-full rounded-md border border-slate-300 object-cover" />
          <div className="flex gap-2">
            <Button onClick={() => onCapture(shot.blob)}>Use this photo</Button>
            <Button variant="secondary" onClick={() => void start()}>Retake</Button>
          </div>
        </>
      )}
      {mode === 'fallback' && (
        <div className="space-y-2">
          {message && <p role="alert" className="rounded-md bg-amber-50 p-3 text-sm text-amber-900">{message}</p>}
          <label className="block text-sm font-medium text-slate-800">
            Choose a photo of the {label.toLowerCase()}
            <input type="file" accept="image/*" capture={facingMode} className="mt-1 block w-full text-sm"
              onChange={(e) => { const f = e.target.files?.[0]; if (f) onCapture(f) }} />
          </label>
          <div className="flex gap-2">
            <Button variant="secondary" size="sm" onClick={() => void start()}>Try the camera again</Button>
            {onCancel && <Button variant="secondary" size="sm" onClick={onCancel}>Cancel</Button>}
          </div>
        </div>
      )}
    </div>
  )
}

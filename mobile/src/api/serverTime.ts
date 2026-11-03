// Countdowns must not trust the device clock: the offset comes from the server's Date header.
let offsetMs = 0

export function setServerTime(dateHeader: string | null) {
  if (!dateHeader) return
  const t = Date.parse(dateHeader)
  if (!Number.isNaN(t)) offsetMs = t - Date.now()
}

export function serverNow(): number {
  return Date.now() + offsetMs
}

export function getServerOffset(): number {
  return offsetMs
}

/** test helper */
export function resetServerTime() {
  offsetMs = 0
}

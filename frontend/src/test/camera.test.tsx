import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { CameraCapture } from '../modules/gate/components/CameraCapture'

function fakeStream() {
  const track = { stop: vi.fn() }
  return { track, stream: { getTracks: () => [track] } as unknown as MediaStream }
}

beforeEach(() => {
  URL.createObjectURL = vi.fn(() => 'blob:photo')
  URL.revokeObjectURL = vi.fn()
  HTMLCanvasElement.prototype.getContext = vi.fn(() => ({ drawImage: vi.fn() })) as never
  HTMLCanvasElement.prototype.toBlob = function (cb: BlobCallback) { cb(new Blob(['jpeg'], { type: 'image/jpeg' })) }
  HTMLMediaElement.prototype.play = vi.fn(() => Promise.resolve())
})
afterEach(() => { Reflect.deleteProperty(navigator, 'mediaDevices') })

function mockCamera(impl: () => Promise<MediaStream>) {
  const getUserMedia = vi.fn(impl)
  Object.defineProperty(navigator, 'mediaDevices', { value: { getUserMedia }, configurable: true })
  return getUserMedia
}

describe('CameraCapture', () => {
  it('asks for the right camera, captures, lets you retake, and hands over the photo', async () => {
    const { stream, track } = fakeStream()
    const gum = mockCamera(() => Promise.resolve(stream))
    const onCapture = vi.fn()
    const user = userEvent.setup()
    render(<CameraCapture facingMode="environment" label="ID photo" onCapture={onCapture} />)
    await user.click(await screen.findByRole('button', { name: 'Take photo' }))
    expect(gum).toHaveBeenCalledWith({ video: { facingMode: { ideal: 'environment' } }, audio: false })
    expect(track.stop).toHaveBeenCalled() // camera is released as soon as the photo is taken
    expect(await screen.findByAltText('Captured ID photo')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Retake' }))
    expect(await screen.findByRole('button', { name: 'Take photo' })).toBeInTheDocument()
    expect(gum).toHaveBeenCalledTimes(2)

    await user.click(screen.getByRole('button', { name: 'Take photo' }))
    await user.click(await screen.findByRole('button', { name: 'Use this photo' }))
    expect(onCapture).toHaveBeenCalledTimes(1)
    expect(onCapture.mock.calls[0]![0]).toBeInstanceOf(Blob)
  })

  it('stops every media track when it is removed from the page', async () => {
    const { stream, track } = fakeStream()
    mockCamera(() => Promise.resolve(stream))
    const { unmount } = render(<CameraCapture facingMode="user" label="Visitor photo" onCapture={vi.fn()} />)
    await screen.findByRole('button', { name: 'Take photo' })
    expect(track.stop).not.toHaveBeenCalled()
    unmount()
    expect(track.stop).toHaveBeenCalled()
  })

  it('falls back to a file picker when the camera permission is denied', async () => {
    mockCamera(() => Promise.reject(new DOMException('denied', 'NotAllowedError')))
    const onCapture = vi.fn()
    const user = userEvent.setup()
    render(<CameraCapture facingMode="user" label="Visitor photo" onCapture={onCapture} />)
    expect(await screen.findByRole('alert')).toHaveTextContent(/permission was denied/i)
    const file = new File(['x'], 'photo.jpg', { type: 'image/jpeg' })
    await user.upload(screen.getByLabelText(/Choose a photo/i), file)
    expect(onCapture).toHaveBeenCalledWith(file)
  })

  it('falls back to a file picker when there is no camera API at all', async () => {
    render(<CameraCapture facingMode="user" label="Visitor photo" onCapture={vi.fn()} />)
    await waitFor(() => expect(screen.getByLabelText(/Choose a photo/i)).toBeInTheDocument())
  })
})

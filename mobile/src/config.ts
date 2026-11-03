import Constants from 'expo-constants'

/**
 * API base URL. Set EXPO_PUBLIC_API_URL (e.g. http://192.168.1.20:8080) for a phone on the same
 * Wi-Fi. Without it, the dev machine's address is taken from Expo's own host (the LAN IP that
 * Expo Go already connects to) and the API port 8080 is used.
 */
export function apiBase(): string {
  const env = process.env.EXPO_PUBLIC_API_URL
  if (env) return env.replace(/\/$/, '')
  const host = Constants.expoConfig?.hostUri?.split(':')[0]
  return `http://${host ?? '127.0.0.1'}:8080`
}

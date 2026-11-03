import { useEffect } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Stack, useRouter, useSegments } from 'expo-router'
import { StatusBar } from 'expo-status-bar'
import { ActivityIndicator, View } from 'react-native'
import { AuthProvider, useAuth } from '../src/auth/AuthContext'

const client = new QueryClient({ defaultOptions: { queries: { retry: 1, staleTime: 5_000 } } })

function Gate() {
  const { user, loading } = useAuth()
  const segments = useSegments()
  const router = useRouter()
  const top = segments[0] as string | undefined

  useEffect(() => {
    if (loading) return
    if (!user) { if (top !== 'login') router.replace('/login') }
    else if (user.mustChangePassword) { if (top !== 'change-password') router.replace('/change-password') }
    else if (top === 'login' || top === 'change-password') router.replace('/')
  }, [user, loading, top, router])

  if (loading) return <View style={{ flex: 1, justifyContent: 'center' }}><ActivityIndicator /></View>
  return (
    <Stack>
      <Stack.Screen name="(app)" options={{ headerShown: false }} />
      <Stack.Screen name="login" options={{ title: 'Gatepass', headerBackVisible: false }} />
      <Stack.Screen name="change-password" options={{ title: 'Change password', headerBackVisible: false }} />
      <Stack.Screen name="visit/[id]" options={{ title: 'Visit' }} />
      <Stack.Screen name="visit/form" options={{ title: 'Planned visit' }} />
    </Stack>
  )
}

export default function Root() {
  return (
    <QueryClientProvider client={client}>
      <AuthProvider>
        <StatusBar style="dark" />
        <Gate />
      </AuthProvider>
    </QueryClientProvider>
  )
}

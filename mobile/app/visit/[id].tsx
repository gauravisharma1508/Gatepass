import { useLocalSearchParams } from 'expo-router'
import { VisitDetail } from '../../src/VisitDetail'

export default function VisitScreen() {
  const { id } = useLocalSearchParams<{ id: string }>()
  return <VisitDetail id={id} />
}

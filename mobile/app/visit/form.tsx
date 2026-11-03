import { useLocalSearchParams } from 'expo-router'
import { VisitFormScreen } from '../../src/VisitForm'

export default function FormRoute() {
  const { id } = useLocalSearchParams<{ id?: string }>()
  return <VisitFormScreen id={id} />
}

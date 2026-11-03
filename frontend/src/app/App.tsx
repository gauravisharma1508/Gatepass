import { BrowserRouter } from 'react-router-dom'
import { Providers } from './Providers'
import { AppRoutes } from './AppRoutes'

export default function App() {
  return (
    <Providers>
      <BrowserRouter>
        <AppRoutes />
      </BrowserRouter>
    </Providers>
  )
}

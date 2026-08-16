import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { AuthProvider } from './lib/auth'
import { ErrorBoundary } from './components/ErrorBoundary'
import App from './App'
import '@xyflow/react/dist/style.css'
import './styles.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ErrorBoundary>
      <BrowserRouter>
        <AuthProvider><App /></AuthProvider>
      </BrowserRouter>
    </ErrorBoundary>
  </StrictMode>,
)

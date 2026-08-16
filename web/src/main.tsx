import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import './i18n'
import './v2/styles/v2.css'
import { UiVariantProvider } from './v2/context/UiVariantContext'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <UiVariantProvider>
      <App />
    </UiVariantProvider>
  </StrictMode>,
)

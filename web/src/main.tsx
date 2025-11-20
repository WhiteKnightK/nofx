import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.tsx'
import './index.css'

// 全局错误处理：静默处理 Google Analytics/Tag Manager 相关错误
window.addEventListener('error', (event) => {
  // 忽略 Google Analytics/Tag Manager 相关的错误
  if (
    event.message?.includes('exmid') ||
    event.message?.includes('google-analytics') ||
    event.message?.includes('googletagmanager') ||
    event.filename?.includes('googletagmanager') ||
    event.filename?.includes('google-analytics')
  ) {
    event.preventDefault()
    return false
  }
})

// 捕获未处理的 Promise 拒绝（Google Analytics 可能产生的）
window.addEventListener('unhandledrejection', (event) => {
  if (
    event.reason?.message?.includes('google-analytics') ||
    event.reason?.message?.includes('googletagmanager') ||
    event.reason?.stack?.includes('google-analytics') ||
    event.reason?.stack?.includes('googletagmanager')
  ) {
    event.preventDefault()
  }
})

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
)

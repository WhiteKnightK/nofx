import { useEffect } from 'react'
import { CheckCircle, XCircle, AlertCircle, Info } from 'lucide-react'

export type ToastType = 'success' | 'error' | 'warning' | 'info'

interface ToastProps {
  message: string
  type?: ToastType
  duration?: number
  onClose: () => void
}

export function Toast({ message, type = 'info', duration = 3000, onClose }: ToastProps) {
  useEffect(() => {
    const timer = setTimeout(() => {
      onClose()
    }, duration)

    return () => clearTimeout(timer)
  }, [duration, onClose])

  const icons = {
    success: <CheckCircle className="w-6 h-6" style={{ color: '#10B981' }} />,
    error: <XCircle className="w-6 h-6" style={{ color: '#F6465D' }} />,
    warning: <AlertCircle className="w-6 h-6" style={{ color: '#F0B90B' }} />,
    info: <Info className="w-6 h-6" style={{ color: '#6366F1' }} />,
  }

  const bgColors = {
    success: 'linear-gradient(135deg, rgba(16, 185, 129, 0.15), rgba(16, 185, 129, 0.08))',
    error: 'linear-gradient(135deg, rgba(246, 70, 93, 0.15), rgba(246, 70, 93, 0.08))',
    warning: 'linear-gradient(135deg, rgba(240, 185, 11, 0.15), rgba(240, 185, 11, 0.08))',
    info: 'linear-gradient(135deg, rgba(99, 102, 241, 0.15), rgba(99, 102, 241, 0.08))',
  }

  const borderColors = {
    success: 'rgba(16, 185, 129, 0.4)',
    error: 'rgba(246, 70, 93, 0.4)',
    warning: 'rgba(240, 185, 11, 0.4)',
    info: 'rgba(99, 102, 241, 0.4)',
  }

  return (
    <div
      className="fixed top-6 right-6 z-50 transform transition-all duration-300 ease-out animate-fade-in-up"
      style={{
        background: bgColors[type],
        border: `1px solid ${borderColors[type]}`,
        borderRadius: '16px',
        padding: '16px 20px',
        minWidth: '320px',
        maxWidth: '480px',
        boxShadow: '0 20px 40px rgba(0, 0, 0, 0.4), 0 8px 16px rgba(0, 0, 0, 0.2)',
        backdropFilter: 'blur(20px)',
        transform: 'translateY(0)',
      }}
    >
      <div className="flex items-start gap-4">
        <div className="flex-shrink-0 mt-0.5">
          {icons[type]}
        </div>
        <div className="flex-1 min-w-0">
          <p
            className="text-sm font-medium leading-relaxed"
            style={{ color: '#EAECEF' }}
          >
            {message}
          </p>
        </div>
        <button
          onClick={onClose}
          className="flex-shrink-0 text-gray-400 hover:text-gray-200 transition-colors duration-200"
          style={{
            fontSize: '18px',
            lineHeight: '1',
            cursor: 'pointer',
            padding: '4px',
            borderRadius: '6px',
            marginTop: '-4px',
            marginRight: '-4px',
          }}
        >
          ×
        </button>
      </div>

      {/* Progress bar */}
      <div
        className="absolute bottom-0 left-0 h-1 rounded-b-2xl transition-all duration-300 ease-linear"
        style={{
          width: '100%',
          background: borderColors[type],
          animation: `progress ${duration}ms linear forwards`,
        }}
      />

      <style dangerouslySetInnerHTML={{
        __html: `
          @keyframes fade-in-up {
            from {
              opacity: 0;
              transform: translateY(-20px) scale(0.95);
            }
            to {
              opacity: 1;
              transform: translateY(0) scale(1);
            }
          }

          @keyframes progress {
            from { width: 100%; }
            to { width: 0%; }
          }

          .animate-fade-in-up {
            animation: fade-in-up 0.4s ease-out;
          }
        `
      }} />
    </div>
  )
}

interface ToastContainerProps {
  toasts: Array<{ id: string; message: string; type: ToastType }>
  onRemove: (id: string) => void
}

export function ToastContainer({ toasts, onRemove }: ToastContainerProps) {
  return (
    <>
      {toasts.map((toast, index) => (
        <div
          key={toast.id}
          className="mb-3"
          style={{
            animationDelay: `${index * 100}ms`,
          }}
        >
          <Toast
            message={toast.message}
            type={toast.type}
            onClose={() => onRemove(toast.id)}
          />
        </div>
      ))}
    </>
  )
}

// Modern Modal Component
interface ModernModalProps {
  isOpen: boolean
  onClose: () => void
  title: string
  children: React.ReactNode
  size?: 'sm' | 'md' | 'lg' | 'xl'
  showCloseButton?: boolean
}

export function ModernModal({
  isOpen,
  onClose,
  title,
  children,
  size = 'md',
  showCloseButton = true
}: ModernModalProps) {
  if (!isOpen) return null

  const sizeClasses = {
    sm: 'max-w-md',
    md: 'max-w-lg',
    lg: 'max-w-2xl',
    xl: 'max-w-4xl'
  }

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      {/* Backdrop */}
      <div
        className="fixed inset-0 transition-opacity duration-300"
        style={{
          background: 'rgba(0, 0, 0, 0.7)',
          backdropFilter: 'blur(12px)',
          WebkitBackdropFilter: 'blur(12px)',
        }}
        onClick={onClose}
      />

      {/* Modal */}
      <div className="flex min-h-full items-center justify-center p-2 sm:p-4">
        <div
          className={`relative w-full ${sizeClasses[size]} transform transition-all duration-300 ease-out max-h-[95vh] overflow-hidden`}
          style={{
            background: 'linear-gradient(135deg, #1a1d23 0%, #0f1116 100%)',
            borderRadius: '20px',
            border: '1px solid rgba(43, 49, 57, 0.8)',
            boxShadow: '0 32px 64px rgba(0, 0, 0, 0.5), 0 16px 32px rgba(0, 0, 0, 0.3)',
            animation: 'modal-slide-in 0.3s ease-out',
          }}
          onClick={(e) => e.stopPropagation()}
        >
          {/* Header */}
          <div className="flex items-center justify-between p-4 sm:p-6 pb-3 sm:pb-4 border-b border-gray-700/30">
            <h2
              className="text-lg sm:text-xl font-bold"
              style={{ color: '#EAECEF' }}
            >
              {title}
            </h2>
            {showCloseButton && (
              <button
                onClick={onClose}
                className="flex items-center justify-center w-7 h-7 sm:w-8 sm:h-8 rounded-full transition-all duration-200 hover:scale-110 hover:bg-gray-600/50"
                style={{
                  background: 'rgba(43, 49, 57, 0.5)',
                  color: '#848E9C',
                  border: '1px solid rgba(132, 142, 156, 0.2)',
                }}
              >
                <svg
                  className="w-3.5 h-3.5 sm:w-4 sm:h-4"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
              </button>
            )}
          </div>

          {/* Content */}
          <div className="px-4 sm:px-6 pb-4 sm:pb-6 overflow-y-auto max-h-[calc(95vh-120px)]">
            {children}
          </div>

          <style dangerouslySetInnerHTML={{
            __html: `
              @keyframes modal-slide-in {
                from {
                  opacity: 0;
                  transform: translateY(-20px) scale(0.95);
                }
                to {
                  opacity: 1;
                  transform: translateY(0) scale(1);
                }
              }
            `
          }} />
        </div>
      </div>
    </div>
  )
}


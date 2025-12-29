import { t, Language } from '../../i18n/translations'

interface FooterSectionProps {
  language: Language
}

export default function FooterSection({ language }: FooterSectionProps) {
  return (
    <footer
      style={{
        borderTop: '1px solid var(--panel-border)',
        background: 'var(--brand-dark-gray)',
      }}
    >
      <div className="max-w-[1200px] mx-auto px-6 py-10">
        <div className="flex flex-col items-center justify-center gap-4">
          {/* Brand */}
          <div className="flex items-center gap-3">
            <img src="/icons/nofx.svg" alt="Platform Logo" className="w-8 h-8" />
            <div>
              <div className="text-lg font-bold" style={{ color: '#EAECEF' }}>
                NOFX
              </div>
              <div className="text-xs" style={{ color: '#848E9C' }}>
                {t('futureStandardAI', language)}
              </div>
            </div>
          </div>
          
          {/* Bottom note */}
          <div
            className="text-center text-xs mt-4"
            style={{ color: 'var(--text-tertiary)' }}
          >
            <p>&copy; {new Date().getFullYear()} AI Trading OS. All rights reserved.</p>
            <p className="mt-1">{t('footerWarning', language)}</p>
          </div>
        </div>
      </div>
    </footer>
  )
}

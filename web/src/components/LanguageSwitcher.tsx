import React from 'react';
import { useTranslation } from 'react-i18next';
import { Globe } from 'lucide-react';

export const LanguageSwitcher: React.FC = () => {
  const { i18n } = useTranslation();
  const currentLang = i18n.language.startsWith('ru') ? 'ru' : 'en';

  const setLanguage = (lang: 'ru' | 'en') => {
    i18n.changeLanguage(lang);
  };

  return (
    <div
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        background: 'rgba(0, 0, 0, 0.45)',
        border: '1px solid rgba(255, 255, 255, 0.1)',
        borderRadius: '8px',
        padding: '2px',
        gap: '2px',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', padding: '0 4px', color: '#818cf8' }}>
        <Globe size={13} />
      </div>

      <button
        type="button"
        onClick={() => setLanguage('ru')}
        style={{
          border: 'none',
          borderRadius: '6px',
          padding: '4px 8px',
          background: currentLang === 'ru' ? 'rgba(99, 102, 241, 0.3)' : 'transparent',
          color: currentLang === 'ru' ? '#e0e7ff' : '#64748b',
          fontSize: '0.74rem',
          fontWeight: 700,
          cursor: 'pointer',
          transition: 'all 0.15s ease',
        }}
        title="Русский язык"
      >
        RU
      </button>

      <button
        type="button"
        onClick={() => setLanguage('en')}
        style={{
          border: 'none',
          borderRadius: '6px',
          padding: '4px 8px',
          background: currentLang === 'en' ? 'rgba(99, 102, 241, 0.3)' : 'transparent',
          color: currentLang === 'en' ? '#e0e7ff' : '#64748b',
          fontSize: '0.74rem',
          fontWeight: 700,
          cursor: 'pointer',
          transition: 'all 0.15s ease',
        }}
        title="English"
      >
        EN
      </button>
    </div>
  );
};


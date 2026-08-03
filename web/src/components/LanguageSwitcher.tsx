import React from 'react';
import { useTranslation } from 'react-i18next';

export const LanguageSwitcher: React.FC = () => {
  const { i18n } = useTranslation();

  const toggleLanguage = () => {
    const newLang = i18n.language.startsWith('ru') ? 'en' : 'ru';
    i18n.changeLanguage(newLang);
  };

  return (
    <button
      onClick={toggleLanguage}
      className="btn btn-secondary"
      style={{ display: 'flex', gap: '8px', alignItems: 'center', padding: '8px 12px' }}
      title="Switch Language"
    >
      <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 5h12M9 3v2m1.048 9.5A18.022 18.022 0 016.412 9m6.088 9h7M11 21l5-10 5 10M12.751 5C11.783 10.77 8.07 15.61 3 18.129" />
      </svg>
      {i18n.language.startsWith('ru') ? 'RU' : 'EN'}
    </button>
  );
};

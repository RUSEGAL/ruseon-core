import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';

import ruTranslations from './locales/ru.json';
import enTranslations from './locales/en.json';

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: {
      ru: { translation: ruTranslations },
      en: { translation: enTranslations }
    },
    fallbackLng: 'ru',
    debug: false,
    interpolation: {
      escapeValue: false // React уже защищает от XSS
    }
  });

export default i18n;

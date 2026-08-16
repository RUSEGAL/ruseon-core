import React from 'react';
import { useTranslation } from 'react-i18next';
import { useUiVariant } from '../../context/UiVariantContext';
import { Sparkles, Layout } from 'lucide-react';

interface UiVariantToggleProps {
  className?: string;
  style?: React.CSSProperties;
}

export const UiVariantToggle: React.FC<UiVariantToggleProps> = ({ className, style }) => {
  const { t } = useTranslation();
  const { uiVariant, toggleUiVariant } = useUiVariant();

  return (
    <button
      className={className || 'v2-ui-toggle'}
      onClick={toggleUiVariant}
      style={style}
      title={
        uiVariant === 'v2'
          ? t('v2.settings.switchToClassic', 'Switch to Classic UI (v1)')
          : t('v2.settings.switchToV2', 'Switch to Next-Gen UI (v2)')
      }
    >
      {uiVariant === 'v2' ? (
        <>
          <Layout size={13} color="#94a3b8" />
          <span>{t('v2.settings.switchToClassic', 'Switch to Classic UI (v1)')}</span>
        </>
      ) : (
        <>
          <Sparkles size={13} color="#a5b4fc" />
          <span>{t('v2.settings.switchToV2', 'Next-Gen UI (v2)')}</span>
        </>
      )}
    </button>
  );
};


import React from 'react';
import { useUiVariant } from '../../context/UiVariantContext';
import { Sparkles, Layout } from 'lucide-react';

export const UiVariantToggle: React.FC = () => {
  const { uiVariant, toggleUiVariant } = useUiVariant();

  return (
    <button
      className="v2-ui-toggle"
      onClick={toggleUiVariant}
      title="Switch between Classic UI and Next-Gen UI"
    >
      {uiVariant === 'v2' ? (
        <>
          <Sparkles size={13} color="#a5b4fc" />
          <span>Next-Gen UI (v2)</span>
        </>
      ) : (
        <>
          <Layout size={13} color="#94a3b8" />
          <span>Classic UI</span>
        </>
      )}
    </button>
  );
};

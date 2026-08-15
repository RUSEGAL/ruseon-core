import React, { createContext, useContext, useState } from 'react';

export type UiVariant = 'classic' | 'v2';

interface UiVariantContextType {
  uiVariant: UiVariant;
  setUiVariant: (variant: UiVariant) => void;
  toggleUiVariant: () => void;
}

const STORAGE_KEY = 'ruseon_ui_variant';

const UiVariantContext = createContext<UiVariantContextType | null>(null);

export const UiVariantProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [uiVariant, setUiVariantState] = useState<UiVariant>(() => {
    try {
      const saved = localStorage.getItem(STORAGE_KEY);
      if (saved === 'classic' || saved === 'v2') return saved;
    } catch {
      // Fallback
    }
    return 'classic';
  });

  const setUiVariant = (variant: UiVariant) => {
    setUiVariantState(variant);
    try {
      localStorage.setItem(STORAGE_KEY, variant);
    } catch (e) {
      console.warn('Failed to save UI variant to localStorage:', e);
    }
  };

  const toggleUiVariant = () => {
    setUiVariant(uiVariant === 'classic' ? 'v2' : 'classic');
  };

  return (
    <UiVariantContext.Provider value={{ uiVariant, setUiVariant, toggleUiVariant }}>
      {children}
    </UiVariantContext.Provider>
  );
};

export function useUiVariant(): UiVariantContextType {
  const context = useContext(UiVariantContext);
  if (!context) {
    throw new Error('useUiVariant must be used within a UiVariantProvider');
  }
  return context;
}

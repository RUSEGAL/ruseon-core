import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { LanguageSwitcher } from './LanguageSwitcher';
import i18n from '../i18n';

describe('LanguageSwitcher Component', () => {
  it('renders RU and EN buttons', () => {
    render(<LanguageSwitcher />);
    expect(screen.getByText('RU')).toBeInTheDocument();
    expect(screen.getByText('EN')).toBeInTheDocument();
  });

  it('changes language on button click', () => {
    render(<LanguageSwitcher />);
    const ruButton = screen.getByText('RU');
    fireEvent.click(ruButton);
    expect(i18n.language.startsWith('ru')).toBe(true);

    const enButton = screen.getByText('EN');
    fireEvent.click(enButton);
    expect(i18n.language.startsWith('en')).toBe(true);
  });
});

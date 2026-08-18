import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Login } from './Login';

describe('Login Component', () => {
  const mockOnLogin = vi.fn();

  beforeEach(() => {
    vi.restoreAllMocks();
    mockOnLogin.mockClear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders username and password inputs and submit button', () => {
    render(<Login onLogin={mockOnLogin} />);
    expect(screen.getByPlaceholderText(/username|имя пользователя/i)).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/password|пароль/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /войти|sign in|login/i })).toBeInTheDocument();
  });

  it('submits credentials and calls onLogin on successful response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ token: 'mock-jwt-token-12345' }),
    });

    render(<Login onLogin={mockOnLogin} />);

    const userInput = screen.getByPlaceholderText(/username|имя пользователя/i);
    const passInput = screen.getByPlaceholderText(/password|пароль/i);
    const submitBtn = screen.getByRole('button', { name: /войти|sign in|login/i });

    fireEvent.change(userInput, { target: { value: 'admin' } });
    fireEvent.change(passInput, { target: { value: 'secretpass' } });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: 'admin', password: 'secretpass' }),
      });
      expect(mockOnLogin).toHaveBeenCalledWith('mock-jwt-token-12345');
    });
  });

  it('displays error message on 401 response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      json: async () => ({ error: 'Invalid credentials' }),
    });

    render(<Login onLogin={mockOnLogin} />);

    const userInput = screen.getByPlaceholderText(/username|имя пользователя/i);
    const passInput = screen.getByPlaceholderText(/password|пароль/i);
    const submitBtn = screen.getByRole('button', { name: /войти|sign in|login/i });

    fireEvent.change(userInput, { target: { value: 'admin' } });
    fireEvent.change(passInput, { target: { value: 'wrongpass' } });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(screen.getByText(/invalid username or password|неверный логин/i)).toBeInTheDocument();
      expect(mockOnLogin).not.toHaveBeenCalled();
    });
  });
});

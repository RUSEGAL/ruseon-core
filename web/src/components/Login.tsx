import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Logo } from './Logo';

interface LoginProps {
  onLogin: (token: string) => void;
}

export function Login({ onLogin }: LoginProps) {
  const { t } = useTranslation();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const res = await fetch('/api/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password })
      });
      if (res.ok) {
        const data = await res.json();
        localStorage.setItem('token', data.token);
        onLogin(data.token);
      } else {
        alert('Invalid credentials');
      }
    } catch {
      alert('Login failed');
    }
  };

  return (
    <div className="login-container">
      <form onSubmit={handleLogin} className="login-form glass">
        <div style={{ textAlign: 'center', marginBottom: '2rem' }}>
          <Logo size={48} className="mx-auto" style={{ marginBottom: '1rem' }} />
          <h2 style={{ fontSize: '1.5rem', fontWeight: 600 }}>{t('login.title')}</h2>
          <p style={{ color: 'var(--text-muted)', fontSize: '0.9rem', marginTop: '4px' }}>{t('login.subtitle')}</p>
        </div>
        <div className="form-group">
          <label>Username</label>
          <input 
            type="text" 
            value={username} 
            onChange={e => setUsername(e.target.value)} 
            required
            className="input-field"
          />
        </div>
        <div className="form-group">
          <label>Password</label>
          <input 
            type="password" 
            value={password} 
            onChange={e => setPassword(e.target.value)} 
            required
            className="input-field"
            placeholder={t('login.placeholder')}
          />
        </div>
        <button type="submit" className="btn btn-primary" style={{ width: '100%', marginTop: '1rem' }}>{t('login.button')}</button>
      </form>
    </div>
  );
}

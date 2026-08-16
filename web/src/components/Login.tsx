import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Logo } from './Logo';
import { LanguageSwitcher } from './LanguageSwitcher';
import {
  User,
  Lock,
  Eye,
  EyeOff,
  ArrowRight,
  ShieldCheck,
  AlertCircle,
  Loader2,
  Radio,
} from 'lucide-react';

interface LoginProps {
  onLogin: (token: string) => void;
}

export const Login: React.FC<LoginProps> = ({ onLogin }) => {
  const { t } = useTranslation();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMessage(null);
    setLoading(true);

    try {
      const res = await fetch('/api/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      });

      if (res.ok) {
        const data = await res.json();
        localStorage.setItem('token', data.token);
        onLogin(data.token);
      } else {
        setErrorMessage(t('login.invalidCredentials', 'Invalid username or password. Please verify credentials.'));
      }
    } catch {
      setErrorMessage(t('login.networkError', 'Network error. Unable to connect to server.'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      style={{
        minHeight: '100vh',
        width: '100%',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'radial-gradient(ellipse at 50% 20%, #151b2e 0%, #070a13 70%)',
        position: 'relative',
        padding: '1.5rem',
        overflow: 'hidden',
      }}
    >
      {/* Ambient background glow spheres */}
      <div
        style={{
          position: 'absolute',
          top: '15%',
          left: '50%',
          transform: 'translateX(-50%)',
          width: '500px',
          height: '350px',
          background: 'radial-gradient(circle, rgba(99, 102, 241, 0.15) 0%, transparent 70%)',
          filter: 'blur(50px)',
          pointerEvents: 'none',
        }}
      />
      <div
        style={{
          position: 'absolute',
          bottom: '10%',
          right: '15%',
          width: '300px',
          height: '300px',
          background: 'radial-gradient(circle, rgba(56, 189, 248, 0.08) 0%, transparent 70%)',
          filter: 'blur(60px)',
          pointerEvents: 'none',
        }}
      />

      {/* Top Header Floating Controls */}
      <div
        style={{
          position: 'absolute',
          top: '20px',
          right: '24px',
          display: 'flex',
          alignItems: 'center',
          gap: '12px',
          zIndex: 10,
        }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
            background: 'rgba(16, 185, 129, 0.1)',
            border: '1px solid rgba(16, 185, 129, 0.25)',
            borderRadius: '20px',
            padding: '4px 10px',
            fontSize: '0.72rem',
            color: '#6ee7b7',
            fontWeight: 600,
          }}
        >
          <Radio size={11} className="animate-pulse" />
          <span>{t('login.systemOnline', 'Engine Ready')}</span>
        </div>

        <LanguageSwitcher />
      </div>

      {/* Main Glass Login Card */}
      <div
        className="glass"
        style={{
          width: '420px',
          maxWidth: '100%',
          padding: '2.5rem 2rem',
          borderRadius: '20px',
          background: 'rgba(13, 18, 30, 0.82)',
          backdropFilter: 'blur(16px)',
          WebkitBackdropFilter: 'blur(16px)',
          border: '1px solid rgba(255, 255, 255, 0.12)',
          boxShadow: '0 24px 64px rgba(0, 0, 0, 0.8), 0 0 0 1px rgba(99, 102, 241, 0.18)',
          position: 'relative',
          zIndex: 5,
        }}
      >
        {/* Brand Header */}
        <div style={{ textAlign: 'center', marginBottom: '2rem' }}>
          <div
            style={{
              width: '64px',
              height: '64px',
              borderRadius: '16px',
              background: 'rgba(99, 102, 241, 0.12)',
              border: '1px solid rgba(99, 102, 241, 0.3)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              margin: '0 auto 1.25rem auto',
              boxShadow: '0 0 24px rgba(99, 102, 241, 0.25)',
            }}
          >
            <Logo size={40} />
          </div>

          <h1
            style={{
              fontSize: '1.45rem',
              fontWeight: 700,
              color: '#f8fafc',
              margin: 0,
              letterSpacing: '-0.3px',
            }}
          >
            {t('login.title', 'RUSEON Core')}
          </h1>
          <p
            style={{
              color: '#94a3b8',
              fontSize: '0.82rem',
              marginTop: '6px',
              lineHeight: 1.4,
            }}
          >
            {t('login.subtitle', 'Next-Gen Video Surveillance & AI Stream Engine')}
          </p>
        </div>

        {/* Error Alert */}
        {errorMessage && (
          <div
            style={{
              background: 'rgba(239, 68, 68, 0.12)',
              border: '1px solid rgba(239, 68, 68, 0.35)',
              borderRadius: '10px',
              padding: '10px 14px',
              marginBottom: '1.25rem',
              display: 'flex',
              alignItems: 'center',
              gap: '10px',
              color: '#fca5a5',
              fontSize: '0.8rem',
              animation: 'v2FadeIn 0.2s ease',
            }}
          >
            <AlertCircle size={16} color="#ef4444" style={{ flexShrink: 0 }} />
            <span>{errorMessage}</span>
          </div>
        )}

        {/* Form */}
        <form onSubmit={handleLogin} style={{ display: 'flex', flexDirection: 'column', gap: '1.1rem' }}>
          {/* Username Input */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
            <label
              style={{
                fontSize: '0.76rem',
                fontWeight: 600,
                color: '#94a3b8',
                textTransform: 'uppercase',
                letterSpacing: '0.4px',
              }}
            >
              {t('login.username', 'Username')}
            </label>
            <div style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
              <User
                size={16}
                color="#6366f1"
                style={{ position: 'absolute', left: '12px', pointerEvents: 'none' }}
              />
              <input
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder={t('login.usernamePlaceholder', 'Enter your username...')}
                required
                autoFocus
                style={{
                  width: '100%',
                  background: 'rgba(0, 0, 0, 0.45)',
                  border: '1px solid rgba(255, 255, 255, 0.12)',
                  borderRadius: '10px',
                  color: '#f8fafc',
                  padding: '10px 12px 10px 38px',
                  fontSize: '0.86rem',
                  outline: 'none',
                  transition: 'border-color 0.15s ease, box-shadow 0.15s ease',
                }}
                onFocus={(e) => {
                  e.currentTarget.style.borderColor = '#6366f1';
                  e.currentTarget.style.boxShadow = '0 0 0 3px rgba(99, 102, 241, 0.2)';
                }}
                onBlur={(e) => {
                  e.currentTarget.style.borderColor = 'rgba(255, 255, 255, 0.12)';
                  e.currentTarget.style.boxShadow = 'none';
                }}
              />
            </div>
          </div>

          {/* Password Input */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
            <label
              style={{
                fontSize: '0.76rem',
                fontWeight: 600,
                color: '#94a3b8',
                textTransform: 'uppercase',
                letterSpacing: '0.4px',
              }}
            >
              {t('login.password', 'Password')}
            </label>
            <div style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
              <Lock
                size={16}
                color="#6366f1"
                style={{ position: 'absolute', left: '12px', pointerEvents: 'none' }}
              />
              <input
                type={showPassword ? 'text' : 'password'}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder={t('login.passwordPlaceholder', 'Enter your password...')}
                required
                style={{
                  width: '100%',
                  background: 'rgba(0, 0, 0, 0.45)',
                  border: '1px solid rgba(255, 255, 255, 0.12)',
                  borderRadius: '10px',
                  color: '#f8fafc',
                  padding: '10px 40px 10px 38px',
                  fontSize: '0.86rem',
                  outline: 'none',
                  transition: 'border-color 0.15s ease, box-shadow 0.15s ease',
                }}
                onFocus={(e) => {
                  e.currentTarget.style.borderColor = '#6366f1';
                  e.currentTarget.style.boxShadow = '0 0 0 3px rgba(99, 102, 241, 0.2)';
                }}
                onBlur={(e) => {
                  e.currentTarget.style.borderColor = 'rgba(255, 255, 255, 0.12)';
                  e.currentTarget.style.boxShadow = 'none';
                }}
              />
              <button
                type="button"
                onClick={() => setShowPassword(!showPassword)}
                style={{
                  position: 'absolute',
                  right: '10px',
                  background: 'transparent',
                  border: 'none',
                  color: '#64748b',
                  cursor: 'pointer',
                  padding: '4px',
                  display: 'flex',
                  alignItems: 'center',
                }}
                title={showPassword ? 'Hide password' : 'Show password'}
              >
                {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
              </button>
            </div>
          </div>

          {/* Submit Button */}
          <button
            type="submit"
            disabled={loading}
            style={{
              marginTop: '0.5rem',
              width: '100%',
              background: 'linear-gradient(135deg, #6366f1 0%, #4f46e5 100%)',
              border: 'none',
              borderRadius: '10px',
              padding: '11px 16px',
              color: '#fff',
              fontSize: '0.88rem',
              fontWeight: 700,
              cursor: loading ? 'not-allowed' : 'pointer',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              gap: '8px',
              boxShadow: '0 4px 18px rgba(99, 102, 241, 0.35)',
              transition: 'all 0.2s ease',
              opacity: loading ? 0.8 : 1,
            }}
          >
            {loading ? (
              <>
                <Loader2 size={18} className="animate-spin" />
                <span>{t('login.signingIn', 'Signing In...')}</span>
              </>
            ) : (
              <>
                <span>{t('login.button', 'Sign In to Console')}</span>
                <ArrowRight size={16} />
              </>
            )}
          </button>
        </form>

        {/* Security Footer Notice */}
        <div
          style={{
            marginTop: '2rem',
            paddingTop: '1.25rem',
            borderTop: '1px solid rgba(255, 255, 255, 0.06)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            gap: '6px',
            fontSize: '0.72rem',
            color: '#64748b',
          }}
        >
          <ShieldCheck size={14} color="#10b981" />
          <span>{t('login.securityNotice', 'Protected by JWT & Argon2id encryption')}</span>
        </div>
      </div>
    </div>
  );
};


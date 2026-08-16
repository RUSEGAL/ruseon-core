import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Zap, CheckCircle2, Sparkles, Laptop, Smartphone, Monitor } from 'lucide-react';
import {
  getHardwareProfile,
  getAiModelPreference,
  AI_MODELS_INFO,
  type HardwareProfile,
  type AiModelPreference,
  type AiModelTier,
} from '../../ai/device-profiler';
import { globalInferenceClient } from '../../ai/inference-client';

export const AiDetectionSettingsCard: React.FC = () => {
  const { t } = useTranslation();
  const [profile, setProfile] = useState<HardwareProfile | null>(null);
  const [preference, setPreference] = useState<AiModelPreference>(getAiModelPreference());
  const [activeTier, setActiveTier] = useState<AiModelTier>(globalInferenceClient.getCurrentTier());
  const [isSwitching, setIsSwitching] = useState(false);

  useEffect(() => {
    getHardwareProfile().then((p) => {
      setProfile(p);
    });

    const unsub = globalInferenceClient.onModelChanged((tier) => {
      setActiveTier(tier);
      setIsSwitching(false);
    });

    return unsub;
  }, []);

  const handleSelectPreference = async (pref: AiModelPreference) => {
    setIsSwitching(true);
    setPreference(pref);
    await globalInferenceClient.switchModelPreference(pref);
  };

  const getDeviceIcon = () => {
    if (!profile) return <Monitor size={16} color="#38bdf8" />;
    switch (profile.deviceType) {
      case 'mobile':
      case 'tablet':
        return <Smartphone size={16} color="#38bdf8" />;
      case 'laptop':
        return <Laptop size={16} color="#38bdf8" />;
      default:
        return <Monitor size={16} color="#38bdf8" />;
    }
  };

  return (
    <div
      style={{
        background: 'rgba(15, 23, 42, 0.65)',
        backdropFilter: 'blur(12px)',
        border: '1px solid rgba(255, 255, 255, 0.08)',
        borderRadius: '12px',
        padding: '1.25rem',
        display: 'flex',
        flexDirection: 'column',
        gap: '1.2rem',
      }}
    >
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '10px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
          <div
            style={{
              background: 'rgba(99, 102, 241, 0.15)',
              padding: '8px',
              borderRadius: '8px',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <Sparkles size={20} color="#818cf8" />
          </div>
          <div>
            <h3 style={{ fontSize: '1rem', fontWeight: 600, color: '#f8fafc', margin: 0 }}>
              {t('v2.ai.title', 'Client AI Engine (Browser-side WebGPU)')}
            </h3>
            <p style={{ fontSize: '0.78rem', color: '#94a3b8', margin: '2px 0 0 0' }}>
              {t('v2.ai.subtitle', 'Inference is executed locally on your GPU without sending video to the server.')}
            </p>
          </div>
        </div>

        {/* Live Status Badge */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            background: profile?.hasWebGpu ? 'rgba(16, 185, 129, 0.12)' : 'rgba(245, 158, 11, 0.12)',
            border: `1px solid ${profile?.hasWebGpu ? 'rgba(16, 185, 129, 0.25)' : 'rgba(245, 158, 11, 0.25)'}`,
            borderRadius: '20px',
            padding: '4px 12px',
            fontSize: '0.75rem',
            fontWeight: 600,
            color: profile?.hasWebGpu ? '#34d399' : '#fbbf24',
          }}
        >
          <span
            style={{
              width: '7px',
              height: '7px',
              borderRadius: '50%',
              background: profile?.hasWebGpu ? '#10b981' : '#f59e0b',
              boxShadow: `0 0 8px ${profile?.hasWebGpu ? '#10b981' : '#f59e0b'}`,
            }}
          />
          <span>{profile?.hasWebGpu ? t('v2.ai.webgpu_active', 'WebGPU Acceleration Active') : t('v2.ai.wasm_active', 'CPU WASM Fallback')}</span>
        </div>
      </div>

      {/* Hardware Profile Banner */}
      {profile && (
        <div
          style={{
            background: 'rgba(255, 255, 255, 0.03)',
            border: '1px solid rgba(255, 255, 255, 0.05)',
            borderRadius: '8px',
            padding: '10px 14px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            flexWrap: 'wrap',
            gap: '10px',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            {getDeviceIcon()}
            <div>
              <div style={{ fontSize: '0.8rem', fontWeight: 600, color: '#f1f5f9' }}>
                {profile.gpuRenderer}
              </div>
              <div style={{ fontSize: '0.72rem', color: '#64748b' }}>
                {profile.deviceType.toUpperCase()} • {profile.cpuCores} CPU • {profile.memoryGb} GB RAM
              </div>
            </div>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '0.74rem', color: '#94a3b8' }}>
            <span>{t('v2.ai.recommended', 'Recommended Profile:')}</span>
            <span style={{ color: '#38bdf8', fontWeight: 600 }}>
              {AI_MODELS_INFO[profile.recommendedTier].name}
            </span>
          </div>
        </div>
      )}

      {/* Quality Tier Selector Cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '10px' }}>
        {/* Auto Option */}
        <div
          onClick={() => handleSelectPreference('auto')}
          style={{
            background: preference === 'auto' ? 'rgba(99, 102, 241, 0.15)' : 'rgba(255, 255, 255, 0.02)',
            border: `1px solid ${preference === 'auto' ? '#6366f1' : 'rgba(255, 255, 255, 0.06)'}`,
            borderRadius: '10px',
            padding: '12px',
            cursor: 'pointer',
            transition: 'all 0.2s ease',
            position: 'relative',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '6px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontWeight: 600, color: '#f8fafc', fontSize: '0.84rem' }}>
              <Zap size={14} color="#818cf8" />
              <span>{t('v2.ai.auto', 'Auto Detect')}</span>
            </div>
            {preference === 'auto' && <CheckCircle2 size={15} color="#818cf8" />}
          </div>
          <p style={{ fontSize: '0.73rem', color: '#94a3b8', margin: 0, lineHeight: 1.35 }}>
            {t('v2.ai.auto_desc', 'Automatically selects the optimal model for your GPU (currently:')}{' '}
            <strong style={{ color: '#38bdf8' }}>{profile ? AI_MODELS_INFO[profile.recommendedTier].name.split(' ')[0] : '...'}</strong>).
          </p>
        </div>

        {/* Nano */}
        <div
          onClick={() => handleSelectPreference('nano')}
          style={{
            background: preference === 'nano' ? 'rgba(56, 189, 248, 0.15)' : 'rgba(255, 255, 255, 0.02)',
            border: `1px solid ${preference === 'nano' ? '#38bdf8' : 'rgba(255, 255, 255, 0.06)'}`,
            borderRadius: '10px',
            padding: '12px',
            cursor: 'pointer',
            transition: 'all 0.2s ease',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '6px' }}>
            <div style={{ fontWeight: 600, color: '#f8fafc', fontSize: '0.84rem' }}>
              YOLOv11-Nano (5.8 МБ)
            </div>
            {preference === 'nano' && <CheckCircle2 size={15} color="#38bdf8" />}
          </div>
          <p style={{ fontSize: '0.73rem', color: '#94a3b8', margin: 0, lineHeight: 1.35 }}>
            {AI_MODELS_INFO.nano.description}
          </p>
        </div>

        {/* Small */}
        <div
          onClick={() => handleSelectPreference('small')}
          style={{
            background: preference === 'small' ? 'rgba(56, 189, 248, 0.15)' : 'rgba(255, 255, 255, 0.02)',
            border: `1px solid ${preference === 'small' ? '#38bdf8' : 'rgba(255, 255, 255, 0.06)'}`,
            borderRadius: '10px',
            padding: '12px',
            cursor: 'pointer',
            transition: 'all 0.2s ease',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '6px' }}>
            <div style={{ fontWeight: 600, color: '#f8fafc', fontSize: '0.84rem' }}>
              YOLOv11-Small (20.1 МБ)
            </div>
            {preference === 'small' && <CheckCircle2 size={15} color="#38bdf8" />}
          </div>
          <p style={{ fontSize: '0.73rem', color: '#94a3b8', margin: 0, lineHeight: 1.35 }}>
            {AI_MODELS_INFO.small.description}
          </p>
        </div>

        {/* Medium */}
        <div
          onClick={() => handleSelectPreference('medium')}
          style={{
            background: preference === 'medium' ? 'rgba(56, 189, 248, 0.15)' : 'rgba(255, 255, 255, 0.02)',
            border: `1px solid ${preference === 'medium' ? '#38bdf8' : 'rgba(255, 255, 255, 0.06)'}`,
            borderRadius: '10px',
            padding: '12px',
            cursor: 'pointer',
            transition: 'all 0.2s ease',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '6px' }}>
            <div style={{ fontWeight: 600, color: '#f8fafc', fontSize: '0.84rem' }}>
              YOLOv11-Medium (41.2 МБ)
            </div>
            {preference === 'medium' && <CheckCircle2 size={15} color="#38bdf8" />}
          </div>
          <p style={{ fontSize: '0.73rem', color: '#94a3b8', margin: 0, lineHeight: 1.35 }}>
            {AI_MODELS_INFO.medium.description}
          </p>
        </div>
      </div>

      {/* Active Model Indicator */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', fontSize: '0.75rem', color: '#64748b' }}>
        <div>
          {t('v2.ai.active_model', 'Active Model in Memory:')}{' '}
          <strong style={{ color: '#e2e8f0' }}>{AI_MODELS_INFO[activeTier].name}</strong>
          {isSwitching && <span style={{ color: '#fbbf24', marginLeft: '6px' }}>({t('v2.ai.loading', 'loading...')})</span>}
        </div>
        <div>
          {t('v2.ai.cache', 'Cache:')} <span style={{ color: '#10b981' }}>{t('v2.ai.instant_start', 'Cache API (Instant Start)')}</span>
        </div>
      </div>
    </div>
  );
};

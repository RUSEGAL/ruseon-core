import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { X, Users, Plus, Trash2, Edit2, Key, Shield, User as UserIcon } from 'lucide-react';

interface User {
  username: string;
  role: 'admin' | 'operator' | 'viewer' | 'service';
}

interface V2UserManagerModalProps {
  token: string | null;
  onClose: () => void;
}

export const V2UserManagerModal: React.FC<V2UserManagerModalProps> = ({
  token,
  onClose,
}) => {
  const { t } = useTranslation();
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);

  // Create Form State
  const [newUsername, setNewUsername] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [newRole, setNewRole] = useState<'admin' | 'operator' | 'viewer' | 'service'>('viewer');

  // Edit / Password Reset State
  const [editingUser, setEditingUser] = useState<User | null>(null);
  const [editRole, setEditRole] = useState<'admin' | 'operator' | 'viewer' | 'service'>('viewer');
  const [resetPassUser, setResetPassUser] = useState<string | null>(null);
  const [resetPasswordVal, setResetPasswordVal] = useState('');

  const fetchUsers = useCallback(async () => {
    try {
      setLoading(true);
      const res = await fetch('/api/users', {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (res.ok) {
        const data = await res.json();
        setUsers(data || []);
      }
    } catch (e) {
      console.error('Failed to fetch users:', e);
    } finally {
      setLoading(false);
    }
  }, [token]);

  useEffect(() => {
    fetchUsers();
  }, [fetchUsers]);

  const handleCreateUser = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newUsername.trim() || !newPassword) return;

    try {
      const res = await fetch('/api/users', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          username: newUsername.trim(),
          password: newPassword,
          role: newRole,
        }),
      });

      if (res.ok) {
        setNewUsername('');
        setNewPassword('');
        setNewRole('viewer');
        fetchUsers();
      } else {
        const err = await res.json();
        alert('Failed to create user: ' + (err.error || 'Unknown error'));
      }
    } catch (err) {
      console.error(err);
    }
  };

  const handleDeleteUser = async (username: string) => {
    if (!confirm(`Delete user "${username}"?`)) return;
    try {
      const res = await fetch(`/api/users/${username}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` },
      });
      if (res.ok) {
        fetchUsers();
      }
    } catch (err) {
      console.error(err);
    }
  };

  const handleUpdateRole = async () => {
    if (!editingUser) return;
    try {
      const res = await fetch(`/api/users/${editingUser.username}/role`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ role: editRole }),
      });
      if (res.ok) {
        setEditingUser(null);
        fetchUsers();
      }
    } catch (err) {
      console.error(err);
    }
  };

  const handleResetPassword = async () => {
    if (!resetPassUser || !resetPasswordVal) return;
    try {
      const res = await fetch(`/api/users/${resetPassUser}/password`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ password: resetPasswordVal }),
      });
      if (res.ok) {
        alert(`Password for user "${resetPassUser}" has been updated.`);
        setResetPassUser(null);
        setResetPasswordVal('');
      }
    } catch (err) {
      console.error(err);
    }
  };

  const getRoleBadge = (role: string) => {
    switch (role) {
      case 'admin':
        return { bg: 'rgba(239, 68, 68, 0.2)', color: '#ef4444', label: t('users.roles.admin', 'ADMINISTRATOR') };
      case 'operator':
        return { bg: 'rgba(245, 158, 11, 0.2)', color: '#f59e0b', label: t('users.roles.operator', 'OPERATOR') };
      case 'service':
        return { bg: 'rgba(99, 102, 241, 0.2)', color: '#a5b4fc', label: t('users.roles.service', 'SERVICE ACCOUNT') };
      default:
        return { bg: 'rgba(16, 185, 129, 0.2)', color: '#10b981', label: t('users.roles.viewer', 'VIEWER') };
    }
  };

  return (
    <div className="v2-modal-overlay" onClick={onClose}>
      <div
        className="v2-modal-container"
        onClick={(e) => e.stopPropagation()}
        style={{ width: '680px', maxWidth: '95vw' }}
      >
        {/* Header */}
        <div className="v2-modal-header">
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <div
              style={{
                width: '34px',
                height: '34px',
                borderRadius: '8px',
                background: 'rgba(99, 102, 241, 0.15)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Users size={18} color="#818cf8" />
            </div>
            <h3 style={{ margin: 0, fontSize: '1.1rem', fontWeight: 700, color: '#f8fafc' }}>
              {t('v2.modals.users.title', 'User Access & RBAC Management')}
            </h3>
          </div>

          <button
            onClick={onClose}
            style={{
              background: 'rgba(255,255,255,0.06)',
              border: 'none',
              borderRadius: '8px',
              padding: '6px',
              color: '#94a3b8',
              cursor: 'pointer',
            }}
          >
            <X size={18} />
          </button>
        </div>

        {/* Body */}
        <div className="v2-modal-body" style={{ gap: '1.25rem' }}>
          {/* Create User Form */}
          <form
            onSubmit={handleCreateUser}
            style={{
              background: 'rgba(0, 0, 0, 0.3)',
              padding: '14px',
              borderRadius: '10px',
              border: '1px solid rgba(255, 255, 255, 0.08)',
              display: 'flex',
              flexDirection: 'column',
              gap: '10px',
            }}
          >
            <div style={{ fontSize: '0.78rem', fontWeight: 700, color: '#94a3b8' }}>
              {t('users.common.add', 'Create User')}
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1.2fr 1.2fr 1fr auto', gap: '8px', alignItems: 'flex-end' }}>
              <div className="v2-form-group">
                <label className="v2-form-label">{t('users.username', 'Username')}</label>
                <input
                  type="text"
                  className="v2-input"
                  placeholder={t('users.usernamePlaceholder', 'e.g. security_guard')}
                  value={newUsername}
                  onChange={(e) => setNewUsername(e.target.value)}
                  required
                />
              </div>

              <div className="v2-form-group">
                <label className="v2-form-label">{t('users.password', 'Password')}</label>
                <input
                  type="password"
                  className="v2-input"
                  placeholder={t('users.passwordPlaceholder', 'Password...')}
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  required
                />
              </div>

              <div className="v2-form-group">
                <label className="v2-form-label">{t('users.role', 'Role')}</label>
                <select
                  className="v2-input"
                  value={newRole}
                  onChange={(e) => setNewRole(e.target.value as any)}
                >
                  <option value="viewer" style={{ background: '#0d111a' }}>{t('users.roles.viewer', 'Viewer')}</option>
                  <option value="operator" style={{ background: '#0d111a' }}>{t('users.roles.operator', 'Operator')}</option>
                  <option value="admin" style={{ background: '#0d111a' }}>{t('users.roles.admin', 'Admin')}</option>
                  <option value="service" style={{ background: '#0d111a' }}>{t('users.roles.service', 'Service')}</option>
                </select>
              </div>

              <button type="submit" className="v2-btn-primary" style={{ height: '36px' }}>
                <Plus size={14} />
                <span>{t('users.common.add', 'Add')}</span>
              </button>
            </div>
          </form>

          {/* Reset Password Submodal inline */}
          {resetPassUser && (
            <div
              style={{
                background: 'rgba(245, 158, 11, 0.1)',
                border: '1px solid rgba(245, 158, 11, 0.3)',
                padding: '12px',
                borderRadius: '10px',
                display: 'flex',
                alignItems: 'center',
                gap: '10px',
              }}
            >
              <div style={{ flex: 1 }}>
                <div style={{ fontSize: '0.8rem', fontWeight: 600, color: '#f59e0b' }}>
                  Reset password for: <strong>{resetPassUser}</strong>
                </div>
                <input
                  type="password"
                  className="v2-input"
                  placeholder="Enter new password..."
                  value={resetPasswordVal}
                  onChange={(e) => setResetPasswordVal(e.target.value)}
                  style={{ marginTop: '6px' }}
                />
              </div>
              <div style={{ display: 'flex', gap: '6px', alignSelf: 'flex-end' }}>
                <button className="v2-btn-primary" onClick={handleResetPassword}>
                  {t('users.common.save', 'Save')}
                </button>
                <button className="v2-btn-secondary" onClick={() => setResetPassUser(null)}>
                  {t('users.common.cancel', 'Cancel')}
                </button>
              </div>
            </div>
          )}

          {/* User List Table */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', maxHeight: '300px', overflowY: 'auto' }}>
            {loading ? (
              <div style={{ textAlign: 'center', color: '#64748b', padding: '1rem' }}>{t('v2.ai.loading', 'Loading users...')}</div>
            ) : users.length === 0 ? (
              <div style={{ textAlign: 'center', color: '#64748b', padding: '1rem' }}>{t('users.noUsers', 'No users found.')}</div>
            ) : (
              users.map((u) => {
                const badge = getRoleBadge(u.role);
                const isEditingThis = editingUser?.username === u.username;

                return (
                  <div
                    key={u.username}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      padding: '10px 14px',
                      borderRadius: '8px',
                      background: 'rgba(255, 255, 255, 0.03)',
                      border: '1px solid rgba(255, 255, 255, 0.05)',
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                      <div
                        style={{
                          width: '28px',
                          height: '28px',
                          borderRadius: '50%',
                          background: 'rgba(255,255,255,0.06)',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                        }}
                      >
                        {u.role === 'admin' ? <Shield size={14} color="#ef4444" /> : <UserIcon size={14} color="#94a3b8" />}
                      </div>

                      <div>
                        <div style={{ fontWeight: 600, color: '#f1f5f9', fontSize: '0.86rem' }}>
                          {u.username}
                        </div>
                      </div>
                    </div>

                    <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                      {isEditingThis ? (
                        <div style={{ display: 'flex', gap: '6px' }}>
                          <select
                            className="v2-input"
                            value={editRole}
                            onChange={(e) => setEditRole(e.target.value as any)}
                            style={{ padding: '3px 8px', fontSize: '0.75rem' }}
                          >
                            <option value="viewer" style={{ background: '#0d111a' }}>{t('users.roles.viewer', 'Viewer')}</option>
                            <option value="operator" style={{ background: '#0d111a' }}>{t('users.roles.operator', 'Operator')}</option>
                            <option value="admin" style={{ background: '#0d111a' }}>{t('users.roles.admin', 'Admin')}</option>
                            <option value="service" style={{ background: '#0d111a' }}>{t('users.roles.service', 'Service')}</option>
                          </select>
                          <button className="v2-btn-primary" style={{ padding: '3px 8px', fontSize: '0.75rem' }} onClick={handleUpdateRole}>
                            {t('users.common.save', 'Save')}
                          </button>
                        </div>
                      ) : (
                        <span
                          style={{
                            background: badge.bg,
                            color: badge.color,
                            padding: '2px 8px',
                            borderRadius: '4px',
                            fontSize: '0.7rem',
                            fontWeight: 700,
                          }}
                        >
                          {badge.label}
                        </span>
                      )}

                      <div style={{ display: 'flex', gap: '4px' }}>
                        <button
                          onClick={() => {
                            setResetPassUser(u.username);
                            setResetPasswordVal('');
                          }}
                          style={{
                            background: 'rgba(255, 255, 255, 0.05)',
                            border: 'none',
                            borderRadius: '6px',
                            padding: '5px',
                            color: '#f59e0b',
                            cursor: 'pointer',
                          }}
                          title={t('users.password', 'Reset Password')}
                        >
                          <Key size={13} />
                        </button>

                        <button
                          onClick={() => {
                            setEditingUser(u);
                            setEditRole(u.role);
                          }}
                          style={{
                            background: 'rgba(255, 255, 255, 0.05)',
                            border: 'none',
                            borderRadius: '6px',
                            padding: '5px',
                            color: '#a5b4fc',
                            cursor: 'pointer',
                          }}
                          title={t('users.common.edit', 'Change Role')}
                        >
                          <Edit2 size={13} />
                        </button>

                        <button
                          onClick={() => handleDeleteUser(u.username)}
                          style={{
                            background: 'rgba(239, 68, 68, 0.15)',
                            border: 'none',
                            borderRadius: '6px',
                            padding: '5px',
                            color: '#ef4444',
                            cursor: 'pointer',
                          }}
                          title={t('users.common.delete', 'Delete User')}
                        >
                          <Trash2 size={13} />
                        </button>
                      </div>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </div>

        {/* Footer */}
        <div className="v2-modal-footer">
          <button className="v2-btn-secondary" onClick={onClose}>
            {t('cameras.cancel', 'Close')}
          </button>
        </div>
      </div>
    </div>
  );
};
